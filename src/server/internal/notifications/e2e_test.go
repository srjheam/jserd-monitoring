package notifications_test

import (
	"bytes"
	"context"
	"fmt"
	"server/internal/audit"
	"server/internal/collections"
	"server/internal/collector"
	"server/internal/config"
	"server/internal/events"
	"server/internal/normalizer"
	"server/internal/notifications"
	"server/internal/snapshotpayload"
	"server/internal/state"
	"server/internal/testseed"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	mail "server/internal/email"

	_ "server/migrations"
)

func TestReplayDoesNotDuplicateInitialDelivery(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	service := newTestService(app)
	var waitGroup sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() { defer waitGroup.Done(); errs <- service.CreateForMatch(matchID) }()
	}
	waitGroup.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	transport := &mail.MockTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "replay-worker", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{collections.Notifications: 1, collections.NotificationTargets: 1, collections.EmailDeliveryRequests: 1, collections.EmailDeliveryAttempts: 1} {
		records, err := app.FindAllRecords(name)
		if err != nil || len(records) != want {
			t.Fatalf("%s count=%d err=%v", name, len(records), err)
		}
	}
	if len(transport.Sent) != 1 {
		t.Fatalf("sent=%d", len(transport.Sent))
	}
}

func TestRN1ToRN8AndRN18DeliveryMatrix(t *testing.T) {
	for _, rule := range []string{ruleRN1, ruleRN2, ruleRN3, "RN4", "RN5", "RN6", "RN7", "RN8", events.RuleRN18} {
		t.Run(rule, func(t *testing.T) {
			app, matchID := seedRuleDelivery(t, rule, []string{"recipient@journal.test"})
			if err := newTestService(app).CreateForMatch(matchID); err != nil {
				t.Fatal(err)
			}
			transport := &mail.MockTransport{}
			worker := &mail.Worker{App: app, Transport: transport, OwnerID: "matrix-" + rule, From: testFromAddress, ReplyTo: []string{"editorial@jserd.test"}}
			if err := worker.RunDue(context.Background()); err != nil {
				t.Fatal(err)
			}
			attempts, _ := app.FindAllRecords(collections.EmailDeliveryAttempts)
			if len(attempts) != 1 || attempts[0].GetString("status") != statusSent || len(transport.Sent) != 1 {
				t.Fatalf("attempts=%d sent=%d", len(attempts), len(transport.Sent))
			}
			message := transport.Sent[0]
			if strings.Contains(message.Subject, rule) || strings.Contains(message.Text, "42") || !strings.Contains(message.Text, "SUB-") || !strings.Contains(message.HTML, "Please") || strings.Contains(message.Text, "Fato:") || strings.Contains(message.Text, "Ação esperada:") || strings.Contains(message.Text, "Prazo:") {
				t.Fatalf("message=%#v", message)
			}
			if (rule == ruleRN1 || rule == ruleRN2 || rule == ruleRN3) && !strings.Contains(message.Text, "Revisor-") {
				t.Fatalf("%s missing pseudonymized reviewer name: %#v", rule, message)
			}
			if rule == "RN5" && (!strings.Contains(message.Text, "sign") || strings.Contains(message.Text, "the authors submitted") || strings.Contains(message.Text, "confirmed the revised version")) {
				t.Fatalf("RN5 asserted evidence too strongly: %s", message.Text)
			}
		})
	}
}

func TestBlockedTargetDoesNotBlockSiblings(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{"", "valid@journal.test"})
	if err := newTestService(app).CreateForMatch(matchID); err != nil {
		t.Fatal(err)
	}
	transport := &mail.MockTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "siblings", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, _ := app.FindAllRecords(collections.EmailDeliveryAttempts)
	statuses := map[string]int{}
	for _, attempt := range attempts {
		statuses[attempt.GetString("status")]++
	}
	if len(attempts) != 2 || statuses["blocked"] != 1 || statuses[statusSent] != 1 || len(transport.Sent) != 1 {
		t.Fatalf("attempts=%d statuses=%v sent=%d", len(attempts), statuses, len(transport.Sent))
	}
}

func notificationsTestApp(t *testing.T) *tests.TestApp {
	t.Helper()
	dir, err := testseed.Dir()
	if err != nil {
		t.Fatal(err)
	}
	app, err := tests.NewTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func seedRuleDelivery(t *testing.T, rule string, addresses []string) (*tests.TestApp, string) {
	t.Helper()
	app := notificationsTestApp(t)
	seedRuleDeliveryRecipients(t, app, rule, addresses)
	event := seedRuleDeliveryEvent(t, app, rule, addresses)
	match := seedRuleDeliveryMatch(t, app, rule, event)
	return app, match.Id
}

func seedRuleDeliveryRecipients(t *testing.T, app *tests.TestApp, rule string, addresses []string) {
	t.Helper()
	recipients, _ := app.FindCollectionByNameOrId(collections.Recipients)
	for recipientIndex, address := range addresses {
		record := core.NewRecord(recipients)
		if events.IsReviewerRule(rule) {
			record.Set("identity", testReviewerKey)
			record.Set("name_slug", testReviewerKey)
			record.Set("is_reviewer", true)
		} else {
			slug := fmt.Sprintf("editor-%d", recipientIndex+1)
			record.Set("identity", slug)
			record.Set("name_slug", slug)
			record.Set("is_editor", true)
		}
		record.Set("name", "Destinatária")
		record.Set("email", address)
		record.Set("welcomed_at", time.Now().UTC())
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
}

func seedRuleDeliveryEvent(t *testing.T, app *tests.TestApp, rule string, addresses []string) *core.Record {
	t.Helper()
	eventsCollection, _ := app.FindCollectionByNameOrId(collections.Events)
	event := core.NewRecord(eventsCollection)
	event.Set("snapshot", seedSnapshot(t, app))
	event.Set("type", "notification_matrix")
	event.Set("entity_type", "submission")
	event.Set("entity_key", "42")
	event.Set("transition", "observed")
	event.Set("occurrence_key", strings.ToLower(rule)+"|42")
	event.Set("idempotency_key", "matrix-"+strings.ToLower(rule))
	editorKeys := make([]string, 0, len(addresses))
	for recipientIndex := range addresses {
		editorKeys = append(editorKeys, fmt.Sprintf("editor-%d", recipientIndex+1))
	}
	payload := map[string]any{payloadSubmissionID: "42", payloadTargetKind: map[bool]string{true: targetKindReviewer, false: targetKindEditor}[events.IsReviewerRule(rule)], payloadTargetKey: testReviewerKey, payloadEditorKeys: editorKeys, payloadTitle: testSubmissionTitle, payloadOJSURL: testSubmissionURL, payloadDeadline: testDeadlineValue}
	if rule == ruleRN1 || rule == ruleRN2 || rule == ruleRN3 {
		payload["reviewerName"] = testReviewerName
	}
	event.Set("payload", payload)
	event.Set("occurred_at", "2026-06-01 10:00:00.000Z")
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	return event
}

func seedRuleDeliveryMatch(t *testing.T, app *tests.TestApp, rule string, event *core.Record) *core.Record {
	t.Helper()
	matches, _ := app.FindCollectionByNameOrId(collections.EventRuleMatches)
	match := core.NewRecord(matches)
	match.Set("event", event.Id)
	match.Set("rule_code", rule)
	match.Set("status", "pending")
	if err := app.Save(match); err != nil {
		t.Fatal(err)
	}
	return match
}

func TestRN1MockDeliveryEndToEnd(t *testing.T) {
	app := notificationsTestApp(t)
	match := seedRN1MockDeliveryFixture(t, app)

	service := newTestService(app)
	if err := service.CreateForMatch(match.Id); err != nil {
		t.Fatal(err)
	}
	if err := service.CreateForMatch(match.Id); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{collections.Notifications: 1, collections.NotificationTargets: 1, collections.EmailDeliveryRequests: 1} {
		records, err := app.FindAllRecords(name)
		if err != nil || len(records) != want {
			t.Fatalf("%s count=%d err=%v", name, len(records), err)
		}
	}

	assertRN1WorkerDeliversMockedMessage(t, app)
}

func seedRN1MockDeliveryFixture(t *testing.T, app *tests.TestApp) *core.Record {
	t.Helper()
	seedRN1MockDeliveryRecipient(t, app)
	event := seedRN1MockDeliveryEvent(t, app)

	matches, _ := app.FindCollectionByNameOrId(collections.EventRuleMatches)
	match := core.NewRecord(matches)
	match.Set("event", event.Id)
	match.Set("rule_code", ruleRN1)
	match.Set("status", "pending")
	if err := app.Save(match); err != nil {
		t.Fatal(err)
	}
	return match
}

func seedRN1MockDeliveryRecipient(t *testing.T, app *tests.TestApp) {
	t.Helper()
	recipients, _ := app.FindCollectionByNameOrId(collections.Recipients)
	recipient := core.NewRecord(recipients)
	recipient.Set("identity", "submission-42")
	recipient.Set("name_slug", "submission-42")
	recipient.Set("is_editor", true)
	recipient.Set("name", "Editora")
	recipient.Set("email", testEditorEmail)
	recipient.Set("welcomed_at", time.Now().UTC())
	if err := app.Save(recipient); err != nil {
		t.Fatal(err)
	}
}

func seedRN1MockDeliveryEvent(t *testing.T, app *tests.TestApp) *core.Record {
	t.Helper()
	eventsCollection, _ := app.FindCollectionByNameOrId(collections.Events)
	event := core.NewRecord(eventsCollection)
	event.Set("snapshot", seedSnapshot(t, app))
	event.Set("type", "review_invitation_accepted")
	event.Set("entity_type", "submission")
	event.Set("entity_key", "submission-42")
	event.Set("transition", "pending->accepted")
	event.Set("occurrence_key", "42|1")
	event.Set("idempotency_key", "rn1-e2e")
	event.Set("payload", map[string]any{payloadSubmissionID: "42", payloadEditorKeys: []string{"submission-42"}, payloadTitle: "Artigo teste", payloadOJSURL: "https://ojs.example.test/submission/42", "reviewerName": testReviewerName})
	event.Set("occurred_at", "2026-06-01 10:00:00.000Z")
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	return event
}

func assertRN1WorkerDeliversMockedMessage(t *testing.T, app *tests.TestApp) {
	t.Helper()
	var output bytes.Buffer
	transport := &mail.MockTransport{Output: &output}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "e2e-worker", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempt count=%d err=%v", len(attempts), err)
	}
	if attempts[0].GetString("status") != statusSent {
		t.Fatalf("status=%q", attempts[0].GetString("status"))
	}
	if len(transport.Sent) != 1 {
		t.Fatalf("sent=%d", len(transport.Sent))
	}
	message := transport.Sent[0]
	if !strings.HasSuffix(message.To, "@example.test") || strings.Contains(message.Subject, ruleRN1) || strings.Contains(message.Text, testReviewerName) || !strings.Contains(message.Text, "Revisor-") || !strings.Contains(message.Text, "submission") || !strings.Contains(message.HTML, "<p>") {
		t.Fatalf("unexpected mock message: %+v", message)
	}
}

func TestMockTransportPseudonymizesBeforeOutput(t *testing.T) {
	var output bytes.Buffer
	transport := &mail.MockTransport{Output: &output}
	original := "real.person@journal.org"
	if err := transport.Send(context.Background(), mail.Message{To: original, From: "sender@journal.org", Subject: "Aviso", Text: original, HTML: "<p>" + original + "</p>"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), original) {
		t.Fatal("stdout contains personal address")
	}
	if !strings.Contains(output.String(), "@example.test") {
		t.Fatal("stdout lacks pseudonymized address")
	}
}

func TestReviewerDeliveryEndToEnd(t *testing.T) {
	app := notificationsTestApp(t)
	match := seedReviewerDeliveryFixture(t, app)
	if err := newTestService(app).CreateForMatch(match.Id); err != nil {
		t.Fatal(err)
	}
	assertReviewerDeliveryMessage(t, app)
}

func seedReviewerDeliveryFixture(t *testing.T, app *tests.TestApp) *core.Record {
	t.Helper()
	seedReviewerDeliveryRecipients(t, app)
	eventsCollection, _ := app.FindCollectionByNameOrId(collections.Events)
	event := core.NewRecord(eventsCollection)
	event.Set("snapshot", seedSnapshot(t, app))
	event.Set("type", "review_response_reminder")
	event.Set("entity_type", "review")
	event.Set("entity_key", "assignment-1")
	event.Set("transition", "due-2d")
	event.Set("occurrence_key", "assignment-1|due-2d|2026-06-10T00:00:00Z")
	event.Set("idempotency_key", "rn6-e2e")
	event.Set("occurred_at", "2026-06-08 00:00:00.000Z")
	event.Set("payload", map[string]any{payloadSubmissionID: "42", payloadTargetKind: targetKindReviewer, payloadTargetKey: testReviewerKey, payloadEditorKeys: []string{testEditorKey, "editor-2", "editor-3"}, payloadTitle: testSubmissionTitle, payloadOJSURL: testSubmissionURL, payloadDeadline: testDeadlineValue})
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	matches, _ := app.FindCollectionByNameOrId(collections.EventRuleMatches)
	match := core.NewRecord(matches)
	match.Set("event", event.Id)
	match.Set("rule_code", "RN6")
	match.Set("status", "pending")
	if err := app.Save(match); err != nil {
		t.Fatal(err)
	}
	return match
}

func seedReviewerDeliveryRecipients(t *testing.T, app *tests.TestApp) {
	t.Helper()
	recipients, _ := app.FindCollectionByNameOrId(collections.Recipients)
	for _, item := range []struct{ kind, key, name, address string }{
		{targetKindReviewer, testReviewerKey, "Revisora", "reviewer@journal.test"},
		{targetKindEditor, testEditorKey, "Editora A", "a@journal.test"},
		{targetKindEditor, "editor-2", "Editora inválida", ""},
		{targetKindEditor, "editor-3", "Editor B", "b@journal.test"},
	} {
		record := core.NewRecord(recipients)
		record.Set("identity", item.key)
		record.Set("name_slug", item.key)
		record.Set("is_editor", item.kind == targetKindEditor)
		record.Set("is_reviewer", item.kind == targetKindReviewer)
		record.Set("name", item.name)
		record.Set("email", item.address)
		record.Set("welcomed_at", time.Now().UTC())
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
}

func assertReviewerDeliveryMessage(t *testing.T, app *tests.TestApp) {
	t.Helper()
	transport := &mail.MockTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "reviewer-e2e", From: testFromAddress, ReplyTo: []string{"fallback@jserd.test"}}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(transport.Sent) != 1 {
		t.Fatalf("sent=%d", len(transport.Sent))
	}
	message := transport.Sent[0]
	if len(message.ReplyTo) != 2 || strings.Contains(message.Text, testDeadlineValue) || !strings.Contains(message.Text, "Prazo-") || !strings.Contains(message.HTML, "Please") || strings.Contains(message.Text, "Fato:") || strings.Contains(message.Text, "Ação esperada:") || strings.Contains(message.Text, "Prazo:") {
		t.Fatalf("reviewer message=%#v", message)
	}
	attempts, _ := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if len(attempts) != 1 || !strings.Contains(attempts[0].GetString("reply_to_addresses"), "a@journal.test") || !strings.Contains(attempts[0].GetString("reply_to_addresses"), "b@journal.test") {
		t.Fatalf("attempts=%#v", attempts)
	}
}

func TestCreatePendingContinuesAfterMatchFailure(t *testing.T) {
	app, _ := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	storedEvents, _ := app.FindAllRecords(collections.Events)
	matches, _ := app.FindCollectionByNameOrId(collections.EventRuleMatches)
	invalid := core.NewRecord(matches)
	invalid.Set("event", storedEvents[0].Id)
	invalid.Set("rule_code", "RN99")
	invalid.Set("status", "pending")
	if err := app.Save(invalid); err != nil {
		t.Fatal(err)
	}
	service := newTestService(app)
	if err := service.CreatePending(); err == nil {
		t.Fatal("expected aggregated error for unsupported match")
	}
	records, _ := app.FindAllRecords(collections.Notifications)
	if len(records) != 1 || records[0].GetString("rule_code") != ruleRN1 {
		t.Fatalf("notifications=%#v", records)
	}
}

func TestProcessCycleToDeliveryEndToEnd(t *testing.T) {
	app := notificationsTestApp(t)
	runRealCycleTwice(t, app)
	markRecipientsWelcomed(t, app)

	service := newTestService(app)
	if err := service.CreatePending(); err != nil {
		t.Fatal(err)
	}
	transport := &mail.MockTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "real-cycle", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.CreatePending(); err != nil {
		t.Fatal(err)
	}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}

	assertRealCycleDeliveredOnePseudonymizedMessage(t, app, transport)
}

// runRealCycleTwice processes the same submission/review twice, the second
// time with a confirmed review, so a real RN1 event and match exist for the
// delivery pipeline to pick up.
func runRealCycleTwice(t *testing.T, app *tests.TestApp) {
	t.Helper()
	store := state.NewStore(app)
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	cycle := func(confirmed string, offset time.Duration) collector.CycleResult {
		return collector.CycleResult{
			SourceMode: config.SourceModeFixtures, StartedAt: now.Add(offset), FinishedAt: now.Add(offset + time.Minute), Status: collector.CycleStatusOK,
			Manifest: audit.Manifest{SourceMode: sourceModeFixtures, CollectedAt: now.Add(offset)},
			Normalized: normalizer.NormalizedCycle{
				SourceMode: sourceModeFixtures, CollectedAt: now.Add(offset),
				Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: "Artigo real", URL: testSubmissionURL, Editors: []normalizer.Editor{{Sequence: 1, GivenName: "Editora", Email: testEditorEmail}}}},
				Reviews:     []normalizer.ReviewAssignment{{Key: "review-42", SubmissionID: "42", SubmissionTitle: "Artigo real", ReviewerUsername: testReviewerKey, ReviewerGivenName: "Revisora", ReviewerFamilyName: "Silva", ReviewerEmail: "reviewer@journal.test", DateConfirmed: confirmed}},
			},
		}
	}
	if err := store.ProcessCycle(cycle("", 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessCycle(cycle("2026-06-01T10:00:00Z", time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func assertRealCycleDeliveredOnePseudonymizedMessage(t *testing.T, app *tests.TestApp, transport *mail.MockTransport) {
	t.Helper()
	for name, want := range map[string]int{collections.Events: 1, collections.EventRuleMatches: 1, collections.Notifications: 1, collections.NotificationTargets: 1, collections.EmailDeliveryRequests: 1, collections.EmailDeliveryAttempts: 1} {
		records, err := app.FindAllRecords(name)
		if err != nil || len(records) != want {
			t.Fatalf("%s=%d err=%v", name, len(records), err)
		}
	}
	if len(transport.Sent) != 1 || strings.Contains(transport.Sent[0].Text, "Artigo real") || strings.Contains(transport.Sent[0].Text, testReviewerName) || !strings.Contains(transport.Sent[0].Text, "Artigo-") || !strings.Contains(transport.Sent[0].Text, "Revisor-") {
		t.Fatalf("sent=%#v", transport.Sent)
	}
}

func markRecipientsWelcomed(t *testing.T, app *tests.TestApp) {
	t.Helper()
	recipients, err := app.FindAllRecords(collections.Recipients)
	if err != nil {
		t.Fatal(err)
	}
	for _, recipient := range recipients {
		recipient.Set("welcomed_at", time.Now().UTC())
		if err := app.Save(recipient); err != nil {
			t.Fatal(err)
		}
	}
}

func seedSnapshot(t *testing.T, app *tests.TestApp) string {
	t.Helper()
	cycles, _ := app.FindCollectionByNameOrId(collections.CollectionCycles)
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", sourceModeFixtures)
	cycle.Set("status", "succeeded")
	cycle.Set("started_at", "2026-06-01 09:00:00.000Z")
	cycle.Set("finished_at", "2026-06-01 09:01:00.000Z")
	cycle.Set("manifest", map[string]any{"sourceMode": sourceModeFixtures})
	if err := app.Save(cycle); err != nil {
		t.Fatal(err)
	}
	snapshots, _ := app.FindCollectionByNameOrId(collections.Snapshots)
	snapshot := core.NewRecord(snapshots)
	snapshot.Set("cycle", cycle.Id)
	snapshot.Set("schema_version", "snapshot:v1")
	payload, err := snapshotpayload.NewFile([]byte(`{"sourceMode":"fixtures"}`))
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Set("payload", payload)
	snapshot.Set("sha256", "abc")
	if err := app.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot.Id
}

// A reviewer rule whose target key names no stored recipient must refuse the
// match rather than record a notification nobody can receive.
func TestReviewerRuleWithNoStoredRecipientIsRefused(t *testing.T) {
	app, matchID := seedRuleDelivery(t, events.RuleRN6, []string{})
	err := newTestService(app).CreateForMatch(matchID)
	if err == nil || !strings.Contains(err.Error(), "reviewer recipient not found") {
		t.Fatalf("CreateForMatch = %v, want a reviewer-recipient failure", err)
	}
	records, _ := app.FindAllRecords(collections.Notifications)
	if len(records) != 0 {
		t.Fatalf("notifications=%#v, want none", records)
	}
}

// An event whose payload cannot be read names nobody, so the match is refused
// before any row is written.
func TestMatchWithUnreadableEventPayloadIsRefused(t *testing.T) {
	app, matchID := seedRuleDelivery(t, ruleRN1, []string{testEditorEmail})
	stored, _ := app.FindAllRecords(collections.Events)
	stored[0].Set("payload", "not a payload")
	if err := app.Save(stored[0]); err != nil {
		t.Fatal(err)
	}
	err := newTestService(app).CreateForMatch(matchID)
	if err == nil || !strings.Contains(err.Error(), "decode event payload") {
		t.Fatalf("CreateForMatch = %v, want a payload decode failure", err)
	}
}

// Other delivery tests use a fixture source that accepts their editor assignments.
func newTestService(app core.App) *notifications.Service {
	service := notifications.NewService(app)
	service.EditorTargets = func(context.Context, string, string) ([]string, error) {
		records, err := app.FindRecordsByFilter(collections.Recipients, "is_editor = true", "", 0, 0)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(records))
		for _, record := range records {
			keys = append(keys, record.GetString("name_slug"))
		}
		return keys, nil
	}
	return service
}
