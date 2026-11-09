package editorial

import (
	"context"
	"errors"
	"server/internal/collections"
	"server/internal/email"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type failingTransport struct{ err error }

func (f failingTransport) Send(context.Context, email.Message) error {
	if f.err != nil {
		return f.err
	}
	return errors.New("password=secret body contents")
}

type captureTransport struct{ sent []email.Message }

func (t *captureTransport) Send(_ context.Context, message email.Message) error {
	t.sent = append(t.sent, message)
	return nil
}

type blockingTransport struct {
	mu      sync.Mutex
	sends   int
	entered chan struct{}
	release chan struct{}
}

func (t *blockingTransport) Send(context.Context, email.Message) error {
	t.mu.Lock()
	t.sends++
	t.mu.Unlock()
	t.entered <- struct{}{}
	<-t.release
	return nil
}

func TestDeliveryAtomicClaim(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedChief(t, app, "A", "a@example.test")
	transport := &blockingTransport{entered: make(chan struct{}, 1), release: make(chan struct{})}
	newService := func(owner string) *DeliveryService {
		return &DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: transport, From: testOJSEmail, OwnerID: owner}
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- newService("process-a").Deliver(context.Background(), cycleID) }()
	<-transport.entered
	attempt, err := app.FindFirstRecordByFilter(collections.EditorialDeliveryAttempts, "cycle = {:cycle}", map[string]any{fieldCycle: cycleID})
	if err != nil || attempt.GetString(fieldStatus) != "sending" {
		t.Fatalf("attempt before SMTP=%v err=%v", attempt, err)
	}
	if err := newService("process-b").Deliver(context.Background(), cycleID); !errors.Is(err, ErrAmbiguousDelivery) {
		t.Fatalf("second deliver=%v", err)
	}
	close(transport.release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.sends != 1 {
		t.Fatalf("sends=%d", transport.sends)
	}
}

func TestDeliveryAbandonedClaim(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedChief(t, app, "A", "a@example.test")
	service := &DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: &email.MockTransport{}, From: testOJSEmail}
	rendered, err := service.Renderer.Render("2026-01-01 09:00", []ArticleFinding{{SubmissionID: "42", Findings: []Finding{{Kind: FindingMissingReviewer}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := service.claimAttempt(cycleID, RecipientResult{Valid: []Recipient{{Address: "a@example.test"}}}, rendered); err != nil || !claimed {
		t.Fatalf("claim=%v err=%v", claimed, err)
	}
	if err := service.Deliver(context.Background(), cycleID); !errors.Is(err, ErrAmbiguousDelivery) {
		t.Fatalf("deliver=%v", err)
	}
}

func TestDeliverySendsOneCollectiveMessage(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedChief(t, app, "B", "b@example.test")
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Renderer: Renderer{}, Transport: transport, From: testOJSEmailTypo}
	if err := service.Deliver(context.Background(), cycleID); err != nil {
		t.Fatal(err)
	}
	if len(transport.sent) != 1 || !strings.Contains(transport.sent[0].To, ",") {
		t.Fatalf("sent=%#v", transport.sent)
	}
	attempts, _ := app.FindRecordsByFilter(collections.EditorialDeliveryAttempts, "", "", 0, 0)
	if len(attempts) != 1 || attempts[0].GetString(fieldStatus) != "sent" || attempts[0].GetString("template_version") != SummaryTemplateVersion {
		t.Fatalf("attempts=%#v", attempts)
	}
}

func TestDeliveryConcurrentCallsSendOnce(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: transport, From: testOJSEmailTypo}
	var waitGroup sync.WaitGroup
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if err := service.Deliver(context.Background(), cycleID); err != nil {
				t.Errorf("deliver: %v", err)
			}
		}()
	}
	waitGroup.Wait()
	if len(transport.sent) != 1 {
		t.Fatalf("sends=%d", len(transport.sent))
	}
	attempts, _ := app.FindRecordsByFilter(collections.EditorialDeliveryAttempts, "", "", 0, 0)
	if len(attempts) != 1 {
		t.Fatalf("attempts=%d", len(attempts))
	}
}

func TestDeliveryBlockedAndFailedAreSanitized(t *testing.T) {
	for _, testCase := range []struct {
		name, want string
		transport  email.Transport
		chief      bool
	}{{statusBlocked, statusBlocked, &email.MockTransport{}, false}, {statusFailed, statusFailed, failingTransport{}, true}} {
		t.Run(testCase.name, func(t *testing.T) {
			app := editorialTestApp(t)
			cycleID := seedDeliveryRun(t, app)
			if testCase.chief {
				seedChief(t, app, "A", "a@example.test")
			}
			service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: testCase.transport, From: testOJSEmailTypo}
			err := service.Deliver(context.Background(), cycleID)
			if testCase.name == statusFailed && err == nil {
				t.Fatal("expected transport error")
			}
			if testCase.name == statusBlocked && err != nil {
				t.Fatal(err)
			}
			attempts, _ := app.FindRecordsByFilter(collections.EditorialDeliveryAttempts, "", "", 1, 0)
			if len(attempts) != 1 || attempts[0].GetString(fieldStatus) != testCase.want {
				t.Fatalf("attempt=%#v", attempts)
			}
			errText := attempts[0].GetString("sanitized_error")
			if strings.Contains(errText, "secret") || strings.Contains(errText, "contents") {
				t.Fatalf("leaked error=%q", errText)
			}
			findings, _ := app.FindRecordsByFilter(collections.EditorialFindings, "", "", 0, 0)
			if len(findings) != 1 {
				t.Fatalf("findings=%d", len(findings))
			}
		})
	}
}

func TestDeliveryTransportFailureIsReturnedAndAudited(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedChief(t, app, "A", "a@example.test")
	sendErr := errors.New("SMTP unavailable")
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: failingTransport{err: sendErr}, From: testOJSEmail}
	if err := service.Deliver(context.Background(), cycleID); !errors.Is(err, sendErr) {
		t.Fatalf("deliver=%v", err)
	}
	attempt, err := app.FindFirstRecordByFilter(collections.EditorialDeliveryAttempts, "cycle = {:cycle}", map[string]any{fieldCycle: cycleID})
	if err != nil {
		t.Fatal(err)
	}
	if attempt.GetString(fieldStatus) != statusFailed || attempt.GetString("sanitized_error") != "SMTP send failed" {
		t.Fatalf("attempt=%#v", attempt)
	}
}

func TestDeliveryFailurePreservesSendAndFinishErrors(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedChief(t, app, "A", "a@example.test")
	sendErr := errors.New("SMTP unavailable")
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: failingTransport{err: sendErr}, From: testOJSEmail}
	service.BeforeSend = func() {
		collection, err := app.FindCollectionByNameOrId(collections.EditorialDeliveryAttempts)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Delete(collection); err != nil {
			t.Fatal(err)
		}
	}
	err := service.Deliver(context.Background(), cycleID)
	if !errors.Is(err, sendErr) || !strings.Contains(err.Error(), "finish editorial delivery attempt") {
		t.Fatalf("deliver=%v", err)
	}
}

func TestDeliveryEmptyCycleCreatesNoAttempt(t *testing.T) {
	app := editorialTestApp(t)
	cycle := saveRecord(t, app, collections.CollectionCycles, map[string]any{fieldSourceMode: sourceModeFixtures, fieldStatus: statusSucceeded, fieldStartedAt: time.Now(), fieldFinishedAt: time.Now(), fieldManifest: map[string]any{fieldSourceMode: sourceModeFixtures}})
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Transport: &email.MockTransport{}}
	if err := service.Deliver(context.Background(), cycle.Id); err != nil {
		t.Fatal(err)
	}
	attempts, _ := app.FindRecordsByFilter(collections.EditorialDeliveryAttempts, "", "", 0, 0)
	if len(attempts) != 0 {
		t.Fatalf("attempts=%d", len(attempts))
	}
}

func TestDeliverySendsNoDigestWhenEveryItemIsFilteredOut(t *testing.T) {
	app := editorialTestApp(t)
	findings := []ArticleFinding{
		{SubmissionID: "42", Title: "Artigo mutado", DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingMissingReviewer}}},
		{SubmissionID: "43", Title: "Artigo em dia", DateSubmitted: "2026-01-02", Findings: []Finding{{Kind: FindingMissingEditor}}},
	}
	cycleID := seedDeliveryRunWithFindings(t, app, findings)
	seedMuted(t, app, "42")
	seedUpToDate(t, app, "43", findings[1].Findings, "")
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Renderer: Renderer{}, Transport: transport, From: testOJSEmail}
	if err := service.Deliver(context.Background(), cycleID); err != nil {
		t.Fatal(err)
	}
	if len(transport.sent) != 0 {
		t.Fatalf("expected no digest, sent=%#v", transport.sent)
	}
	attempts, _ := app.FindRecordsByFilter(collections.EditorialDeliveryAttempts, "", "", 0, 0)
	if len(attempts) != 0 {
		t.Fatalf("attempts=%d", len(attempts))
	}
}

func seedDeliveryRun(t *testing.T, app core.App) string {
	t.Helper()
	return seedDeliveryRunWithFindings(t, app, []ArticleFinding{{SubmissionID: "42", Title: testSubmissionTitle, URL: testSubmissionURL, DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingMissingReviewer}}}})
}

func seedDeliveryRunWithFindings(t *testing.T, app core.App, findings []ArticleFinding) string {
	t.Helper()
	cycle := saveRecord(t, app, collections.CollectionCycles, map[string]any{fieldSourceMode: sourceModeFixtures, fieldStatus: statusSucceeded, fieldStartedAt: time.Date(2026, 6, 30, 23, 0, 0, 0, time.UTC), fieldFinishedAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), fieldManifest: map[string]any{fieldSourceMode: sourceModeFixtures}})
	if err := NewStore(app).ReplaceFindings(cycle.Id, findings); err != nil {
		t.Fatal(err)
	}
	return cycle.Id
}

func seedMuted(t *testing.T, app core.App, ojsSubmissionID string) {
	t.Helper()
	seedTracking(t, app, ojsSubmissionID, map[string]any{"muted": true})
}

func seedUpToDate(t *testing.T, app core.App, ojsSubmissionID string, findings []Finding, until string) {
	t.Helper()
	fields := map[string]any{"up_to_date_digest": DigestFor(findings)}
	if until != "" {
		fields["up_to_date_until"] = until
	}
	seedTracking(t, app, ojsSubmissionID, fields)
}

func seedTracking(t *testing.T, app core.App, ojsSubmissionID string, fields map[string]any) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collections.SubmissionTracking)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.Set("ojs_submission_id", ojsSubmissionID)
	for field, value := range fields {
		record.Set(field, value)
	}
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryDropsMutedSubmission(t *testing.T) {
	app := editorialTestApp(t)
	cycleID := seedDeliveryRun(t, app)
	seedMuted(t, app, "42")
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Renderer: Renderer{}, Transport: transport, From: testOJSEmail}
	if err := service.Deliver(context.Background(), cycleID); err != nil {
		t.Fatal(err)
	}
	if len(transport.sent) != 0 {
		t.Fatalf("expected no digest, sent=%#v", transport.sent)
	}
}

func TestDeliveryDropsUpToDateSubmission(t *testing.T) {
	app := editorialTestApp(t)
	findings := []Finding{{Kind: FindingMissingReviewer}}
	cycleID := seedDeliveryRun(t, app)
	seedUpToDate(t, app, "42", findings, "")
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Renderer: Renderer{}, Transport: transport, From: testOJSEmail}
	if err := service.Deliver(context.Background(), cycleID); err != nil {
		t.Fatal(err)
	}
	if len(transport.sent) != 0 {
		t.Fatalf("expected no digest, sent=%#v", transport.sent)
	}
}

func TestDeliveryReincludesSetAsideSubmissionOnceFindingsChange(t *testing.T) {
	app := editorialTestApp(t)
	// Set aside for a different finding set than the run's current findings,
	// so the stored digest no longer matches and the submission returns.
	cycleID := seedDeliveryRun(t, app)
	seedUpToDate(t, app, "42", []Finding{{Kind: FindingMissingEditor}}, "")
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Renderer: Renderer{}, Transport: transport, From: testOJSEmail}
	if err := service.Deliver(context.Background(), cycleID); err != nil {
		t.Fatal(err)
	}
	if len(transport.sent) != 1 {
		t.Fatalf("expected the digest to be sent, sent=%#v", transport.sent)
	}
	if !strings.Contains(transport.sent[0].Text, testSubmissionTitle) {
		t.Fatalf("expected the submission back in the digest, text=%q", transport.sent[0].Text)
	}
}

func TestDeliveryOrdersSurvivorsByPriorityThenElapsedDaysDescending(t *testing.T) {
	app := editorialTestApp(t)
	findings := []ArticleFinding{
		{SubmissionID: "1", Title: "Recente sem editor", DateSubmitted: testDate20260601, Findings: []Finding{{Kind: FindingMissingEditor}}},
		{SubmissionID: "2", Title: "Antiga sem revisor", DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingMissingReviewer}}},
		{SubmissionID: "3", Title: "Recente sem editor tardia", DateSubmitted: "2026-06-15", Findings: []Finding{{Kind: FindingMissingEditor}}},
	}
	cycleID := seedDeliveryRunWithFindings(t, app, findings)
	seedChief(t, app, "A", "a@example.test")
	transport := &captureTransport{}
	service := DeliveryService{App: app, Resolver: RecipientResolver{App: app}, Renderer: Renderer{}, Transport: transport, From: testOJSEmail}
	if err := service.Deliver(context.Background(), cycleID); err != nil {
		t.Fatal(err)
	}
	if len(transport.sent) != 1 {
		t.Fatalf("sent=%#v", transport.sent)
	}
	text := transport.sent[0].Text
	// FindingMissingEditor outranks FindingMissingReviewer, so submissions 1
	// and 3 come first (worst priority), ordered oldest-first between them;
	// submission 2 comes last.
	first := strings.Index(text, "Recente sem editor\n")
	tardia := strings.Index(text, "Recente sem editor tardia")
	last := strings.Index(text, "Antiga sem revisor")
	if first == -1 || tardia == -1 || last == -1 {
		t.Fatalf("missing article labels in text=%q", text)
	}
	if first >= tardia || tardia >= last {
		t.Fatalf("unexpected order: first=%d tardia=%d last=%d text=%q", first, tardia, last, text)
	}
}
