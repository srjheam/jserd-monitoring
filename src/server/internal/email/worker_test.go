package email_test

import (
	"context"
	"errors"
	"server/internal/collections"
	"server/internal/notifications"
	"server/internal/snapshotpayload"
	"server/internal/testseed"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/cron"
	mail "server/internal/email"

	_ "server/migrations"
)

type checkingTransport struct {
	app   core.App
	calls int
	t     *testing.T
}

type failingTransport struct{ err error }

type concurrentTransport struct {
	mu    sync.Mutex
	calls int
}

type captureMessageTransport struct{ message mail.Message }

func (t *captureMessageTransport) Send(_ context.Context, message mail.Message) error {
	t.message = message
	return nil
}

func TestWorkerPassesSensitiveFieldsToMock(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	transport := &captureMessageTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "sensitive-fields", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"recipient_name": false, "submission_id": false, fieldTitle: false, fieldOJSURL: false, "deadline": false, "reviewer_name": false}
	for _, field := range transport.message.SensitiveFields {
		if _, ok := want[field.Name]; ok {
			want[field.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("missing sensitive field %s: %#v", name, transport.message.SensitiveFields)
		}
	}
}

func TestWorkerDoesNotSendIncompleteReviewerPayload(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	events, err := app.FindAllRecords(collections.Events)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	events[0].Set("payload", map[string]any{fieldTitle: testSubmissionTitle, fieldOJSURL: testSubmissionURL})
	if err := app.Save(events[0]); err != nil {
		t.Fatal(err)
	}
	transport := &captureMessageTransport{}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: "incomplete-reviewer", From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if transport.message.Subject != "" {
		t.Fatalf("transport received incomplete message: %#v", transport.message)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 || attempts[0].GetString("status") != statusFailed || attempts[0].GetString("result_class") != "render_failure" {
		t.Fatalf("attempts=%#v err=%v", attempts, err)
	}
}

func (t *concurrentTransport) Send(context.Context, mail.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	return nil
}

func TestConcurrentWorkersSendOnce(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	transport := &concurrentTransport{}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	beforeClaim := func() { once.Do(func() { close(entered); <-release }) }
	workers := []*mail.Worker{
		{App: app, Transport: transport, OwnerID: "worker-1", From: testFromAddress, BeforeClaim: beforeClaim},
		{App: app, Transport: transport, OwnerID: "worker-2", From: testFromAddress, BeforeClaim: beforeClaim},
	}
	errs := make(chan error, len(workers))
	go func() { errs <- workers[0].RunDue(context.Background()) }()
	<-entered
	go func() { errs <- workers[1].RunDue(context.Background()) }()
	close(release)
	for range workers {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.calls != 1 {
		t.Fatalf("Send calls=%d", transport.calls)
	}
}

func TestUnrelatedConstraintRollsBack(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	transport := &concurrentTransport{}
	worker := &mail.Worker{
		App: app, Transport: transport, OwnerID: ownerIDWorker, From: testFromAddress,
		SaveClaim: func(txApp core.App, _ *core.Record) error {
			collection, err := txApp.FindCollectionByNameOrId(collections.Recipients)
			if err != nil {
				return err
			}
			duplicate := core.NewRecord(collection)
			duplicate.Set("identity", "submission-42")
			duplicate.Set("name_slug", "submission-42")
			duplicate.Set("is_editor", true)
			duplicate.Set("email", "other@example.test")
			return txApp.Save(duplicate)
		},
	}
	if err := worker.RunDue(context.Background()); err == nil {
		t.Fatalf("unrelated constraint was masked: %v", err)
	}
	claims, _ := app.FindAllRecords(collections.EmailDeliveryClaims)
	if len(claims) != 0 || transport.calls != 0 {
		t.Fatalf("constraint did not rollback: claims=%d sends=%d", len(claims), transport.calls)
	}
}

func (f failingTransport) Send(context.Context, mail.Message) error { return f.err }

func TestRegisterJobsUsesConfiguredCronExpressions(t *testing.T) {
	scheduler := cron.New()
	mail.RegisterJobs(scheduler, "7 * * * *", "*/3 * * * *", func() {}, func() {})
	jobs := scheduler.Jobs()
	if len(jobs) != 2 || jobs[0].Id() != "ojs-collection" || jobs[0].Expression() != "7 * * * *" || jobs[1].Id() != "email-delivery-worker" || jobs[1].Expression() != "*/3 * * * *" {
		t.Fatalf("jobs=%v", jobs)
	}
}

func TestRegisterJobsAfterRestart(t *testing.T) {
	scheduler := cron.New()
	mail.RegisterJobs(scheduler, "0 * * * *", "* * * * *", func() {}, func() {})
	mail.RegisterJobs(scheduler, "5 * * * *", "*/2 * * * *", func() {}, func() {})
	jobs := scheduler.Jobs()
	if len(jobs) != 2 || jobs[0].Expression() != "5 * * * *" || jobs[1].Expression() != "*/2 * * * *" {
		t.Fatalf("restart jobs=%v", jobs)
	}
}

func TestWorkerPersistsConfirmedResults(t *testing.T) {
	for _, testCase := range []struct {
		name, status, result string
		transport            mail.Transport
	}{
		{name: statusSent, status: statusSent, result: "success", transport: &checkingTransport{}},
		{name: statusFailed, status: statusFailed, result: "smtp_permanent_failure", transport: failingTransport{err: errors.New("sentinel-password sentinel-token")}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			app := seedRequest(t, "editor@journal.test")
			if checking, ok := testCase.transport.(*checkingTransport); ok {
				checking.app, checking.t = app, t
			}
			worker := &mail.Worker{App: app, Transport: testCase.transport, OwnerID: ownerIDWorker, From: testFromAddress}
			if err := worker.RunDue(context.Background()); err != nil {
				t.Fatal(err)
			}
			attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
			if err != nil || len(attempts) != 1 || attempts[0].GetString("status") != testCase.status || attempts[0].GetString("result_class") != testCase.result {
				t.Fatalf("attempts=%v err=%v", attempts, err)
			}
			errorText := attempts[0].GetString("sanitized_error") + attempts[0].GetString("raw_error")
			if strings.Contains(errorText, "sentinel-password") || strings.Contains(errorText, "sentinel-token") {
				t.Fatalf("attempt leaked secret: %q", errorText)
			}
		})
	}
}

func TestAttemptStoresParsedFromAddress(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	worker := &mail.Worker{App: app, Transport: &captureMessageTransport{}, OwnerID: "from-address", From: "JSERD <noreply@jserd.test>"}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 || attempts[0].GetString("from_address") != testFromAddress {
		t.Fatalf("attempts=%#v err=%v", attempts, err)
	}
}

func TestSMTPFailureDoesNotInvalidateCollection(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	cyclesBefore, _ := app.FindAllRecords(collections.CollectionCycles)
	snapshotsBefore, _ := app.FindAllRecords(collections.Snapshots)
	eventsBefore, _ := app.FindAllRecords(collections.Events)
	matchesBefore, _ := app.FindAllRecords(collections.EventRuleMatches)
	worker := &mail.Worker{App: app, Transport: failingTransport{err: errors.New("confirmed rejection")}, OwnerID: ownerIDWorker, From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	cyclesAfter, _ := app.FindAllRecords(collections.CollectionCycles)
	snapshotsAfter, _ := app.FindAllRecords(collections.Snapshots)
	eventsAfter, _ := app.FindAllRecords(collections.Events)
	matchesAfter, _ := app.FindAllRecords(collections.EventRuleMatches)
	if len(cyclesBefore) != len(cyclesAfter) || len(snapshotsBefore) != len(snapshotsAfter) || len(eventsBefore) != len(eventsAfter) || len(matchesBefore) != len(matchesAfter) {
		t.Fatal("SMTP failure changed collection state")
	}
}

func (c *checkingTransport) Send(_ context.Context, _ mail.Message) error {
	claims, err := c.app.FindAllRecords(collections.EmailDeliveryClaims)
	if err != nil || len(claims) != 1 {
		c.t.Fatalf("claim must exist before Send: count=%d err=%v", len(claims), err)
	}
	c.calls++
	return nil
}

func TestWorkerClaimsBeforeSend(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	transport := &checkingTransport{app: app, t: t}
	first := &mail.Worker{App: app, Transport: transport, OwnerID: "worker-1", From: testFromAddress}
	second := &mail.Worker{App: app, Transport: transport, OwnerID: "worker-2", From: testFromAddress}
	if err := first.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := second.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if transport.calls != 1 {
		t.Fatalf("Send calls=%d", transport.calls)
	}
}

func TestWorkerBlocksInvalidRecipient(t *testing.T) {
	app := seedRequest(t, "")
	transport := &checkingTransport{app: app, t: t}
	worker := &mail.Worker{App: app, Transport: transport, OwnerID: ownerIDWorker, From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if transport.calls != 0 {
		t.Fatal("transport called for invalid recipient")
	}
	attempts, _ := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if len(attempts) != 1 || attempts[0].GetString("status") != "blocked" {
		t.Fatalf("blocked attempt missing")
	}
}

func TestInvalidRecipientCreatesBlockedAttempt(t *testing.T) {
	app := seedRequest(t, "")
	worker := &mail.Worker{App: app, Transport: failingTransport{err: errors.New("must not send")}, OwnerID: ownerIDWorker, From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 || attempts[0].GetString("status") != "blocked" || attempts[0].GetString("result_class") != "recipient_blocked" {
		t.Fatalf("attempts=%v err=%v", attempts, err)
	}
}

func TestAttemptRedactsSecrets(t *testing.T) {
	app := seedRequest(t, "editor@journal.test")
	secret := "password=secret cookie=session token=bearer postgres://user:pass@db/jserd"
	worker := &mail.Worker{App: app, Transport: failingTransport{err: errors.New(secret)}, OwnerID: ownerIDWorker, From: testFromAddress}
	if err := worker.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempts, err := app.FindAllRecords(collections.EmailDeliveryAttempts)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts=%v err=%v", attempts, err)
	}
	serialized := attempts[0].GetString("sanitized_error") + attempts[0].GetString("raw_error")
	for _, sentinel := range []string{"secret", "session", "bearer", "postgres://"} {
		if strings.Contains(serialized, sentinel) {
			t.Fatalf("attempt leaked %q: %q", sentinel, serialized)
		}
	}
	if attempts[0].GetString("template_hash") == "" || attempts[0].GetString("to_address") == "" || attempts[0].GetString("from_address") == "" || attempts[0].GetString("subject") == "" {
		t.Fatalf("attempt missing frozen envelope/audit fields: %#v", attempts[0])
	}
}

func seedRequest(t *testing.T, address string) *tests.TestApp {
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

	snapshot := seedRequestSnapshot(t, app)
	event := seedRequestEvent(t, app, snapshot, address)
	match := seedRequestMatch(t, app, event)
	seedRequestRecipient(t, app, address)

	service := notifications.NewService(app)
	service.EditorTargets = func(context.Context, string, string) ([]string, error) { return []string{"submission-42"}, nil }
	if err := service.CreateForMatch(match.Id); err != nil {
		t.Fatal(err)
	}
	return app
}

func seedRequestSnapshot(t *testing.T, app core.App) *core.Record {
	t.Helper()
	cycles, _ := app.FindCollectionByNameOrId(collections.CollectionCycles)
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", "fixtures")
	cycle.Set("status", "succeeded")
	cycle.Set("started_at", "2026-06-01 09:00:00.000Z")
	cycle.Set("finished_at", "2026-06-01 09:01:00.000Z")
	cycle.Set("manifest", map[string]any{"sourceMode": "fixtures"})
	mustSave(t, app, cycle)

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
	mustSave(t, app, snapshot)
	return snapshot
}

func seedRequestEvent(t *testing.T, app core.App, snapshot *core.Record, address string) *core.Record {
	t.Helper()
	events, _ := app.FindCollectionByNameOrId(collections.Events)
	event := core.NewRecord(events)
	event.Set("snapshot", snapshot.Id)
	event.Set("type", "review_invitation_accepted")
	event.Set("entity_type", "submission")
	event.Set("entity_key", "submission-42")
	event.Set("transition", "pending->accepted")
	event.Set("occurrence_key", "42|1")
	event.Set("idempotency_key", "worker-test-"+address)
	event.Set("payload", map[string]any{"submissionId": "42", "editorTargetKeys": []string{"submission-42"}, fieldTitle: testSubmissionTitle, fieldOJSURL: testSubmissionURL, "reviewerName": "Revisora Silva"})
	event.Set("occurred_at", "2026-06-01 10:00:00.000Z")
	mustSave(t, app, event)
	return event
}

func seedRequestMatch(t *testing.T, app core.App, event *core.Record) *core.Record {
	t.Helper()
	matches, _ := app.FindCollectionByNameOrId(collections.EventRuleMatches)
	match := core.NewRecord(matches)
	match.Set("event", event.Id)
	match.Set("rule_code", "RN1")
	match.Set("status", "pending")
	mustSave(t, app, match)
	return match
}

func seedRequestRecipient(t *testing.T, app core.App, address string) *core.Record {
	t.Helper()
	recipients, _ := app.FindCollectionByNameOrId(collections.Recipients)
	recipient := core.NewRecord(recipients)
	recipient.Set("identity", "submission-42")
	recipient.Set("name_slug", "submission-42")
	recipient.Set("is_editor", true)
	recipient.Set("name", "Editora")
	recipient.Set("email", address)
	recipient.Set("welcomed_at", time.Now().UTC())
	mustSave(t, app, recipient)
	return recipient
}

func mustSave(t *testing.T, app core.App, record *core.Record) {
	t.Helper()
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
}
