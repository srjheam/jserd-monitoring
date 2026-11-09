package email_test

import (
	"context"
	"errors"
	"server/internal/collections"
	mail "server/internal/email"
	"server/internal/events"
	"testing"
)

const (
	testQueuedEditorKey   = "submission-42"
	testRoleBlockedStatus = "blocked"
)

func TestWorkerChecksQueuedEditorAssignments(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		keys           []string
		lookupErr      error
		expectedStatus string
	}{
		{name: "assigned", keys: []string{testQueuedEditorKey}, expectedStatus: statusSent},
		{name: "removed", expectedStatus: testRoleBlockedStatus},
		{name: "lookup failed", lookupErr: errors.New("OJS unavailable")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			app := seedRequest(t, "editor@journal.test")
			worker := &mail.Worker{App: app, Transport: &mail.MockTransport{}, OwnerID: ownerIDWorker, From: testFromAddress}
			worker.EditorTargets = func(_ context.Context, rule, submission string) ([]string, error) {
				if rule != events.RuleRN1 || submission != "42" {
					t.Errorf("rule=%s submission=%s", rule, submission)
				}
				return testCase.keys, testCase.lookupErr
			}
			err := worker.RunDue(context.Background())
			if !errors.Is(err, testCase.lookupErr) {
				t.Fatalf("RunDue=%v, want %v", err, testCase.lookupErr)
			}
			attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
			if err != nil {
				t.Fatal(err)
			}
			if testCase.expectedStatus == "" {
				if len(attempts) != 0 {
					t.Fatalf("failed lookup created %d attempts", len(attempts))
				}
				return
			}
			if len(attempts) != 1 || attempts[0].GetString("status") != testCase.expectedStatus {
				t.Fatalf("attempts=%v, want %s", attempts, testCase.expectedStatus)
			}
		})
	}
}

func TestWorkerCannotInferWelcomeRoleFromUnknownEvent(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	records, err := app.FindAllRecords(collections.Notifications)
	if err != nil {
		t.Fatal(err)
	}
	records[0].Set("rule_code", events.RuleWelcome)
	mustSave(t, app, records[0])
	storedEvents, err := app.FindAllRecords(collections.Events)
	if err != nil {
		t.Fatal(err)
	}
	storedEvents[0].Set("type", "unknown-event")
	mustSave(t, app, storedEvents[0])
	transport := &mail.MockTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: ownerIDWorker, From: testFromAddress}
	worker.EditorTargets = func(context.Context, string, string) ([]string, error) {
		t.Error("unknown role reached OJS lookup")
		return nil, nil
	}
	if err := worker.RunDue(context.Background()); err == nil {
		t.Fatal("unknown welcome role was accepted")
	}
	claims, err := app.FindAllRecords(collections.EmailDeliveryClaims)
	if err != nil || len(claims) != 0 || len(transport.Sent) != 0 {
		t.Fatalf("claims=%v sent=%d err=%v", claims, len(transport.Sent), err)
	}
}

func TestLegacyQueuedTargetRecordsItsVerifiedRole(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	targets, err := app.FindAllRecords(collections.NotificationTargets)
	if err != nil {
		t.Fatal(err)
	}
	targets[0].Set("recipient_role", "")
	mustSave(t, app, targets[0])
	worker := &mail.Worker{App: app, Transport: &mail.MockTransport{}, OwnerID: ownerIDWorker, From: testFromAddress}
	worker.EditorTargets = func(context.Context, string, string) ([]string, error) { return []string{testQueuedEditorKey}, nil }
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts=%v err=%v", attempts, err)
	}
	if role := attempts[0].GetString("recipient_role"); role != events.RecipientRoleAssociateEditor {
		t.Fatalf("role=%s, want verified associate editor", role)
	}
}
