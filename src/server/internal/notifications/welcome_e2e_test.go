package notifications_test

import (
	"context"
	"errors"
	"server/internal/collections"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	mail "server/internal/email"

	_ "server/migrations"
)

func TestFirstNotificationEnqueuesWelcomeEmail(t *testing.T) {
	app := notificationsTestApp(t)
	seedWelcomeRecipient(t, app, testEditorKey, testEditorEmail)
	matchID := seedWelcomeMatch(t, app, ruleRN1, "welcome-first", []string{testEditorKey})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}

	notificationRecords, _ := app.FindAllRecords(collections.Notifications)
	targets, _ := app.FindAllRecords(collections.NotificationTargets)
	requests, _ := app.FindAllRecords(collections.EmailDeliveryRequests)
	if len(notificationRecords) != 2 || len(targets) != 2 || len(requests) != 2 {
		t.Fatalf("notifications=%d targets=%d requests=%d", len(notificationRecords), len(targets), len(requests))
	}
	if welcomeCount(notificationRecords) != 1 {
		t.Fatalf("welcome notifications=%d, want 1: %#v", welcomeCount(notificationRecords), notificationRecords)
	}

	recipients, _ := app.FindAllRecords(collections.Recipients)
	if len(recipients) != 1 || recipients[0].GetString("welcomed_at") == "" {
		t.Fatalf("recipient not marked welcomed: %#v", recipients)
	}
}

func TestSecondNotificationDoesNotEnqueueAnotherWelcome(t *testing.T) {
	app := notificationsTestApp(t)
	seedWelcomeRecipient(t, app, testEditorKey, testEditorEmail)
	service := newTestService(app)
	first := seedWelcomeMatch(t, app, ruleRN1, "welcome-first", []string{testEditorKey})
	if err := service.CreateForMatch(first); err != nil {
		t.Fatal(err)
	}
	second := seedWelcomeMatch(t, app, "RN4", "welcome-second", []string{testEditorKey})
	if err := service.CreateForMatch(second); err != nil {
		t.Fatal(err)
	}

	notificationRecords, _ := app.FindAllRecords(collections.Notifications)
	if welcomeCount(notificationRecords) != 1 {
		t.Fatalf("welcome notifications=%d, want 1 after a second real notification: %#v", welcomeCount(notificationRecords), notificationRecords)
	}
}

func TestDualRoleRecipientGetsOneWelcomeEmailEver(t *testing.T) {
	app := notificationsTestApp(t)
	recipients, _ := app.FindCollectionByNameOrId(collections.Recipients)
	record := core.NewRecord(recipients)
	record.Set("identity", "casey-reviewer")
	record.Set("name_slug", "casey-lee")
	record.Set("is_editor", true)
	record.Set("is_reviewer", true)
	record.Set("name", "Casey Lee")
	record.Set("email", "ana@journal.test")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	service := newTestService(app)
	editorMatch := seedWelcomeMatch(t, app, ruleRN1, "dual-editor", []string{"casey-lee"})
	if err := service.CreateForMatch(editorMatch); err != nil {
		t.Fatal(err)
	}
	reviewerMatch := seedWelcomeReviewerMatch(t, app, "RN6", "dual-reviewer", "casey-reviewer")
	if err := service.CreateForMatch(reviewerMatch); err != nil {
		t.Fatal(err)
	}

	notificationRecords, _ := app.FindAllRecords(collections.Notifications)
	if welcomeCount(notificationRecords) != 1 {
		t.Fatalf("welcome notifications=%d, want exactly 1 across both roles: %#v", welcomeCount(notificationRecords), notificationRecords)
	}
}

type selectiveFailTransport struct{ failSubstring string }

func (s *selectiveFailTransport) Send(_ context.Context, message mail.Message) error {
	if strings.Contains(message.Subject, s.failSubstring) {
		return errors.New("delivery failed")
	}
	return nil
}

func TestWelcomeDeliveryFailureDoesNotBlockTriggeredSibling(t *testing.T) {
	app := notificationsTestApp(t)
	seedWelcomeRecipient(t, app, testEditorKey, testEditorEmail)
	matchID := seedWelcomeMatch(t, app, ruleRN1, "welcome-worker", []string{testEditorKey})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}

	transport := &selectiveFailTransport{failSubstring: "Welcome"}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "welcome-worker-test", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}

	attempts, _ := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if len(attempts) != 2 {
		t.Fatalf("attempts=%d, want 2: %#v", len(attempts), attempts)
	}
	statuses := map[string]string{}
	for _, attempt := range attempts {
		statuses[attempt.GetString("rule_code")] = attempt.GetString("status")
	}
	if statuses["WELCOME"] != "failed" || statuses[ruleRN1] != statusSent {
		t.Fatalf("statuses=%v", statuses)
	}
}

func welcomeCount(records []*core.Record) int {
	count := 0
	for _, record := range records {
		if record.GetString("rule_code") == "WELCOME" {
			count++
		}
	}
	return count
}

func seedWelcomeRecipient(t *testing.T, app *tests.TestApp, slug, address string) {
	t.Helper()
	recipients, _ := app.FindCollectionByNameOrId(collections.Recipients)
	record := core.NewRecord(recipients)
	record.Set("identity", slug)
	record.Set("name_slug", slug)
	record.Set("is_editor", true)
	record.Set("name", "Destinatária")
	record.Set("email", address)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
}

func seedWelcomeMatch(t *testing.T, app *tests.TestApp, rule, idempotencyKey string, editorTargetKeys []string) string {
	t.Helper()
	event := core.NewRecord(mustCollection(t, app, collections.Events))
	event.Set("snapshot", seedSnapshot(t, app))
	event.Set("type", "welcome_email_test")
	event.Set("entity_type", "submission")
	event.Set("entity_key", "42")
	event.Set("transition", "observed")
	event.Set("occurrence_key", idempotencyKey)
	event.Set("idempotency_key", idempotencyKey)
	event.Set("payload", map[string]any{payloadSubmissionID: "42", payloadTargetKind: targetKindEditor, payloadEditorKeys: editorTargetKeys, payloadTitle: testSubmissionTitle, payloadOJSURL: testSubmissionURL, payloadDeadline: testDeadlineValue, "reviewerName": testReviewerName})
	event.Set("occurred_at", "2026-06-01 10:00:00.000Z")
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	return saveWelcomeMatch(t, app, event.Id, rule)
}

func seedWelcomeReviewerMatch(t *testing.T, app *tests.TestApp, rule, idempotencyKey, reviewerKey string) string {
	t.Helper()
	event := core.NewRecord(mustCollection(t, app, collections.Events))
	event.Set("snapshot", seedSnapshot(t, app))
	event.Set("type", "welcome_email_test")
	event.Set("entity_type", "review")
	event.Set("entity_key", "assignment-1")
	event.Set("transition", "observed")
	event.Set("occurrence_key", idempotencyKey)
	event.Set("idempotency_key", idempotencyKey)
	event.Set("payload", map[string]any{payloadSubmissionID: "42", payloadTargetKind: targetKindReviewer, payloadTargetKey: reviewerKey, payloadEditorKeys: []string{}, payloadTitle: testSubmissionTitle, payloadOJSURL: testSubmissionURL, payloadDeadline: testDeadlineValue})
	event.Set("occurred_at", "2026-06-01 10:00:00.000Z")
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	return saveWelcomeMatch(t, app, event.Id, rule)
}

func saveWelcomeMatch(t *testing.T, app *tests.TestApp, eventID, rule string) string {
	t.Helper()
	match := core.NewRecord(mustCollection(t, app, collections.EventRuleMatches))
	match.Set("event", eventID)
	match.Set("rule_code", rule)
	match.Set("status", "pending")
	if err := app.Save(match); err != nil {
		t.Fatal(err)
	}
	return match.Id
}

func mustCollection(t *testing.T, app *tests.TestApp, name string) *core.Collection {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatal(err)
	}
	return collection
}
