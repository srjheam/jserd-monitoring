package notifications_test

import (
	"context"
	"errors"
	"server/internal/collections"
	"server/internal/events"
	"server/internal/notifications"
	"testing"

	mail "server/internal/email"
)

const (
	testJournalEditorKey   = "editor-2"
	testJournalEditorEmail = "journal@journal.test"
	testAssociateGroup     = "Associate Editor"
	testJournalGroup       = "Journal editor"
	testRoleReadError      = "OJS unavailable"
)

type assignmentSource struct {
	stage             int
	group, submission string
}

func (s *assignmentSource) Login(context.Context, string, string) error { return nil }
func (s *assignmentSource) AssignedEditors(_ context.Context, submission string, stage int, group string) ([]string, error) {
	s.submission, s.stage, s.group = submission, stage, group
	if group == testJournalGroup {
		return []string{testJournalEditorKey}, nil
	}
	return []string{testEditorKey}, nil
}

func TestEditorRulesTargetOnlyTheirSubmissionAssignmentGroup(t *testing.T) {
	for _, rule := range []string{ruleRN1, ruleRN2, ruleRN3, "RN4", events.RuleRN5, events.RuleRN18} {
		t.Run(rule, func(t *testing.T) {
			app, matchID := seedRuleDelivery(t, rule, []string{testEditorEmail, testJournalEditorEmail, "other@journal.test"})
			source := &assignmentSource{}
			service := notifications.NewService(app)
			service.EditorTargets = (notifications.OJSEditorTargets{Client: source}).Keys
			if err := service.CreateForMatch(matchID); err != nil {
				t.Fatal(err)
			}
			expectedKey, expectedStage, expectedGroup := testEditorKey, 3, testAssociateGroup
			if rule == events.RuleRN18 {
				expectedKey, expectedStage, expectedGroup = testJournalEditorKey, 4, testJournalGroup
			}
			targets, err := app.FindAllRecords(collections.NotificationTargets)
			if err != nil || len(targets) != 1 || targets[0].GetString("target_key") != expectedKey {
				t.Fatalf("targets=%v err=%v, want only %s", targets, err, expectedKey)
			}
			if source.stage != expectedStage || source.group != expectedGroup || source.submission != "42" {
				t.Fatalf("assignment query=%+v", source)
			}
		})
	}
}

func TestUnknownOrEmptyEditorAssignmentsQueueNothing(t *testing.T) {
	cases := []struct {
		name    string
		targets notifications.EditorTargets
	}{
		{name: "no source"},
		{name: "no matching group", targets: func(context.Context, string, string) ([]string, error) { return nil, nil }},
		{name: "read failure", targets: func(context.Context, string, string) ([]string, error) { return nil, errors.New(testRoleReadError) }},
		{name: "editor from another submission", targets: func(context.Context, string, string) ([]string, error) { return []string{"unrelated"}, nil }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
			service := notifications.NewService(app)
			service.EditorTargets = testCase.targets
			if err := service.CreateForMatch(matchID); err == nil {
				t.Fatal("expected unresolved assignment error")
			}
			requests, err := app.FindAllRecords(collections.EmailDeliveryRequests)
			if err != nil || len(requests) != 0 {
				t.Fatalf("requests=%v err=%v", requests, err)
			}
			service.EditorTargets = func(context.Context, string, string) ([]string, error) { return []string{testEditorKey}, nil }
			if err := service.CreateForMatch(matchID); err != nil {
				t.Fatalf("retry: %v", err)
			}
		})
	}
}

func TestQueuedEditorTargetsAreCheckedBeforeDelivery(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail, testJournalEditorEmail})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	transport := &mail.MockTransport{}
	worker := &mail.Worker{
		App: app, Transport: transport, OwnerID: "role-guard", From: testFromAddress,
		EditorTargets: func(context.Context, string, string) ([]string, error) { return []string{testEditorKey}, nil },
	}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(transport.Sent) != 1 {
		t.Fatalf("sent=%d, want 1", len(transport.Sent))
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 2 {
		t.Fatalf("attempts=%v err=%v", attempts, err)
	}
	statuses := map[string]string{}
	for _, attempt := range attempts {
		statuses[attempt.GetString("target_key")] = attempt.GetString("status")
	}
	if statuses[testEditorKey] != statusSent || statuses[testJournalEditorKey] != "blocked" {
		t.Fatalf("statuses=%v", statuses)
	}
}

func TestAssignmentReadFailureLeavesQueuedEmailRetryable(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	transport := &mail.MockTransport{}
	worker := &mail.Worker{
		App: app, Transport: transport, OwnerID: "role-retry", From: testFromAddress,
		EditorTargets: func(context.Context, string, string) ([]string, error) { return nil, errors.New(testRoleReadError) },
	}
	if err := worker.RunDue(context.Background()); err == nil {
		t.Fatal("expected assignment read error")
	}
	claims, err := app.FindAllRecords(collections.EmailDeliveryClaims)
	if err != nil || len(claims) != 0 || len(transport.Sent) != 0 {
		t.Fatalf("claims=%v sent=%d err=%v", claims, len(transport.Sent), err)
	}
	worker.EditorTargets = func(context.Context, string, string) ([]string, error) { return []string{testEditorKey}, nil }
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(transport.Sent) != 1 {
		t.Fatalf("sent=%d, want 1 after retry", len(transport.Sent))
	}
}

func TestRoleLookupFailureDoesNotBlockSiblingRequest(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail, testJournalEditorEmail})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	transport := &mail.MockTransport{}
	worker := &mail.Worker{
		App: app, Transport: transport, OwnerID: "role-siblings", From: testFromAddress,
		EditorTargets: func(context.Context, string, string) ([]string, error) {
			calls++
			if calls == 1 {
				return nil, errors.New(testRoleReadError)
			}
			return []string{testEditorKey, testJournalEditorKey}, nil
		},
	}
	if err := worker.RunDue(context.Background()); err == nil {
		t.Fatal("expected assignment read error")
	}
	if len(transport.Sent) != 1 || calls != 2 {
		t.Fatalf("sent=%d calls=%d, want sibling delivery", len(transport.Sent), calls)
	}
}

func TestQueuedWelcomeFollowsTheOriginalEditorRule(t *testing.T) {
	app := notificationsTestApp(t)
	seedWelcomeRecipient(t, app, testEditorKey, testEditorEmail)
	matchID := seedWelcomeMatch(t, app, events.RuleRN18, "role-welcome", []string{testEditorKey})
	stored, err := app.FindAllRecords(collections.Events)
	if err != nil {
		t.Fatal(err)
	}
	stored[0].Set("type", events.EventMissingCopyeditor)
	if err := app.Save(stored[0]); err != nil {
		t.Fatal(err)
	}
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	transport := &mail.MockTransport{}
	worker := &mail.Worker{
		App: app, Transport: transport, OwnerID: "role-welcome", From: testFromAddress,
		EditorTargets: func(_ context.Context, rule, submission string) ([]string, error) {
			if rule != events.RuleRN18 || submission != "42" {
				t.Errorf("lookup rule=%s submission=%s", rule, submission)
			}
			return nil, nil
		},
	}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 2 || len(transport.Sent) != 0 {
		t.Fatalf("attempts=%d sent=%d err=%v", len(attempts), len(transport.Sent), err)
	}
	for _, attempt := range attempts {
		if attempt.GetString("status") != "blocked" {
			t.Fatalf("attempt status=%s", attempt.GetString("status"))
		}
	}
}
