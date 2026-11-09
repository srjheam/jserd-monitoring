package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"server/internal/collections"
	"server/internal/collector"
	"server/internal/config"
	"server/internal/editorial"
	"server/internal/monitoring"
	"server/internal/testseed"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

type fakeCycleProcessor struct {
	err    error
	called bool
}

type fakeEditorialRunner struct {
	result        editorial.Result
	err           error
	calls         int
	gotSendDigest bool
}

func (f *fakeEditorialRunner) RunCycle(_ context.Context, _ editorial.RunSource, _ string, sendDigest bool) (editorial.Result, error) {
	f.calls++
	f.gotSendDigest = sendDigest
	return f.result, f.err
}

type fakeScheduler struct{ jobs map[string]func() }

func createTestSuperuser(t *testing.T, app core.App) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.SetEmail("operator@example.test")
	record.SetPassword("valid-password")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func createManualCollectionRequest(t *testing.T, app core.App, collection *core.Collection, operator *core.Record) *core.Record {
	t.Helper()
	record := core.NewRecord(collection)
	record.Set("confirmed", true)
	record.Set("operator", operator.Id)
	record.Set("status", "pending")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func createSucceededCollectionCycle(t *testing.T, app core.App) *core.Record {
	t.Helper()
	cycles, _ := app.FindCollectionByNameOrId(collections.CollectionCycles)
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", "fixtures")
	cycle.Set("status", "succeeded")
	cycle.Set("started_at", time.Now())
	cycle.Set("finished_at", time.Now())
	cycle.Set("manifest", map[string]any{"source_mode": "fixtures"})
	if err := app.Save(cycle); err != nil {
		t.Fatal(err)
	}
	return cycle
}

func (s *fakeScheduler) MustAdd(id, _ string, run func()) {
	if s.jobs == nil {
		s.jobs = map[string]func(){}
	}
	s.jobs[id] = run
}

func TestCollectionJobRegistersSharedRunner(t *testing.T) {
	s := &fakeScheduler{}
	runner := &fakeEditorialRunner{}
	registerCollectionJob(s, "0 * * * *", runner)
	job := s.jobs["ojs-collection"]
	if job == nil {
		t.Fatal("ojs-collection job not registered")
	}
	job()
	if runner.calls != 1 {
		t.Fatalf("calls=%d", runner.calls)
	}
	if runner.gotSendDigest {
		t.Fatal("the scheduled collection job must not force the summary e-mail")
	}
}

func serverTestApp(t *testing.T) *tests.TestApp {
	t.Helper()
	dir, err := testseed.Dir()
	if err != nil {
		t.Fatal(err)
	}
	app, err := tests.NewTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestManualCollectionRequestTransitions(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	collection, err := app.FindCollectionByNameOrId(collections.ManualCollectionRequests)
	if err != nil {
		t.Fatal(err)
	}
	if collection.CreateRule != nil {
		t.Fatal("ordinary users must not have create access")
	}
	operator := createTestSuperuser(t, app)
	record := createManualCollectionRequest(t, app, collection, operator)
	cycle := createSucceededCollectionCycle(t, app)
	runner := &fakeEditorialRunner{result: editorial.Result{CycleID: cycle.Id}}
	if err := executeManualRequest(context.Background(), app, record.Id, runner); err != nil {
		t.Fatal(err)
	}
	fresh, _ := app.FindRecordById(collection.Id, record.Id)
	if runner.calls != 1 || fresh.GetString("status") != "succeeded" || fresh.GetString("cycle") != cycle.Id || fresh.GetString("operator") != operator.Id {
		t.Fatalf("calls=%d record=%#v", runner.calls, fresh)
	}
	if runner.gotSendDigest {
		t.Fatal("send_digest omitted must default to false")
	}
}

func TestManualCollectionRequestSendDigestOptIn(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	collection, err := app.FindCollectionByNameOrId(collections.ManualCollectionRequests)
	if err != nil {
		t.Fatal(err)
	}
	operator := createTestSuperuser(t, app)
	record := core.NewRecord(collection)
	record.Set("confirmed", true)
	record.Set("operator", operator.Id)
	record.Set("status", "pending")
	record.Set("send_digest", true)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	runner := &fakeEditorialRunner{}
	if err := executeManualRequest(context.Background(), app, record.Id, runner); err != nil {
		t.Fatal(err)
	}
	if !runner.gotSendDigest {
		t.Fatal("send_digest=true must be passed through to the runner")
	}
}

func TestManualCollectionRequestFailure(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	collection, _ := app.FindCollectionByNameOrId(collections.ManualCollectionRequests)
	operator := createTestSuperuser(t, app)
	record := core.NewRecord(collection)
	record.Set("confirmed", true)
	record.Set("operator", operator.Id)
	record.Set("status", "pending")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	if err := executeManualRequest(context.Background(), app, record.Id, &fakeEditorialRunner{err: errors.New("SMTP send failed")}); err != nil {
		t.Fatal(err)
	}
	fresh, _ := app.FindRecordById(collection.Id, record.Id)
	if fresh.GetString("status") != "failed" || fresh.GetString("sanitized_error") == "" {
		t.Fatalf("record=%#v", fresh)
	}
}

func TestManualCollectionRequestReportsLoadFailure(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	if err := executeManualRequest(context.Background(), app, "missing-request", &fakeEditorialRunner{}); err == nil {
		t.Fatal("expected load error")
	}
}

func TestManualCollectionRequestRequiresConfirmation(t *testing.T) {
	runner := &fakeEditorialRunner{}
	record := core.NewRecord(core.NewBaseCollection(collections.ManualCollectionRequests))
	record.Set("confirmed", false)
	if err := validateManualRequest(record, "admin-id"); err == nil || runner.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, runner.calls)
	}
}

func TestManualCollectionRequestSetsManagedFields(t *testing.T) {
	record := core.NewRecord(core.NewBaseCollection(collections.ManualCollectionRequests))
	record.Set("confirmed", true)
	record.Set("status", "failed")
	record.Set("operator", "submitted-operator")
	if err := validateManualRequest(record, "authenticated-superuser"); err != nil {
		t.Fatal(err)
	}
	if record.GetString("status") != "pending" || record.GetString("operator") != "authenticated-superuser" {
		t.Fatalf("status=%q operator=%q", record.GetString("status"), record.GetString("operator"))
	}
}

func TestStartupRecoveryRunsAsynchronously(t *testing.T) {
	started := make(chan struct{})
	startupRecovery(func() { close(started) })
	select {
	case <-started:
	case <-context.Background().Done():
		t.Fatal("recovery did not start")
	}
}

func TestImmutableHooksRegisterOnlyWhenServing(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	registerImmutableHooksOnServe(app)

	collection, err := app.FindCollectionByNameOrId(collections.CollectionCycles)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.Set("source_mode", "fixtures")
	record.Set("status", "failed")
	record.Set("started_at", time.Now())
	record.Set("finished_at", time.Now())
	record.Set("manifest", map[string]any{"source_mode": "fixtures"})
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	record.Set("status", "succeeded")
	if err := app.Save(record); err != nil {
		t.Fatalf("update before serve: %v", err)
	}

	event := &core.ServeEvent{App: app}
	if err := app.OnServe().Trigger(event, func(*core.ServeEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	record.Set("status", "failed")
	if err := app.Save(record); err == nil {
		t.Fatal("operational update after serve unexpectedly allowed")
	}
}

func TestCollectionJobLogsFailure(t *testing.T) {
	s := &fakeScheduler{}
	registerCollectionJob(s, "* * * * *", &fakeEditorialRunner{err: errors.New("token=secret")})
	if s.jobs["ojs-collection"] == nil {
		t.Fatal("job not registered")
	}
	s.jobs["ojs-collection"]()
}

func createTestChiefEditor(t *testing.T, app core.App) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collections.ChiefEditors)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.SetEmail("editor@example.test")
	record.SetPassword("valid-password")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestMonitoringRouteRequiresChiefEditor(t *testing.T) {
	guestApp := serverTestApp(t)
	defer guestApp.Cleanup()
	monitoring.Register(guestApp)
	(&tests.ApiScenario{Name: "guest rejected", Method: http.MethodGet, URL: monitoring.Route, ExpectedStatus: http.StatusUnauthorized, ExpectedContent: []string{"requires valid record authorization token"}, TestAppFactory: func(testing.TB) *tests.TestApp { return guestApp }, DisableTestAppCleanup: true}).Test(t)

	superuserApp := serverTestApp(t)
	defer superuserApp.Cleanup()
	operator := createTestSuperuser(t, superuserApp)
	superuserAppToken, err := operator.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	monitoring.Register(superuserApp)
	(&tests.ApiScenario{Name: "superuser rejected", Method: http.MethodGet, URL: monitoring.Route, Headers: map[string]string{"Authorization": superuserAppToken}, ExpectedStatus: http.StatusForbidden, ExpectedContent: []string{"is not allowed to perform this action"}, TestAppFactory: func(testing.TB) *tests.TestApp { return superuserApp }, DisableTestAppCleanup: true}).Test(t)

	chiefEditorApp := serverTestApp(t)
	defer chiefEditorApp.Cleanup()
	editor := createTestChiefEditor(t, chiefEditorApp)
	editorToken, err := editor.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	monitoring.Register(chiefEditorApp)
	(&tests.ApiScenario{Name: "chief editor receives empty monitoring projection", Method: http.MethodGet, URL: monitoring.Route, Headers: map[string]string{"Authorization": editorToken}, ExpectedStatus: http.StatusOK, ExpectedContent: []string{"\"availability\":\"empty\""}, TestAppFactory: func(testing.TB) *tests.TestApp { return chiefEditorApp }, DisableTestAppCleanup: true}).Test(t)
}

func TestChiefEditorPasswordResetSendsWithoutSuperuser(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	cfg := config.Config{SMTPHost: "smtp.example.test", SMTPPort: 587, SMTPFromAddress: "JSERD <noreply@example.test>"}
	if err := applySMTPSettings(app, cfg); err != nil {
		t.Fatal(err)
	}
	if !app.Settings().SMTP.Enabled {
		t.Fatal("SMTP must be enabled once a host is configured")
	}
	createTestChiefEditor(t, app)

	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	event := &core.ServeEvent{App: app, Router: router}
	if err := app.OnServe().Trigger(event, func(*core.ServeEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	mux, err := event.Router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/collections/chief_editors/request-password-reset", bytes.NewReader([]byte(`{"email":"editor@example.test"}`)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("request-password-reset=%d %s", response.Code, response.Body.String())
	}
	// SendRecordPasswordReset runs in a fire-and-forget goroutine
	// (pocketbase/apis.recordRequestPasswordReset), so give it a moment.
	deadline := time.Now().Add(2 * time.Second)
	for app.TestMailer.TotalSend() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if app.TestMailer.TotalSend() != 1 {
		t.Fatalf("expected one reset e-mail, got %d", app.TestMailer.TotalSend())
	}
}

func (f *fakeCycleProcessor) ProcessCycle(collector.CycleResult) error { f.called = true; return f.err }

type fakePendingCreator struct {
	err    error
	called bool
}

func (f *fakePendingCreator) CreatePending() error { f.called = true; return f.err }

func TestCollectionCallbackMaterializesPendingMatches(t *testing.T) {
	store := &fakeCycleProcessor{}
	service := &fakePendingCreator{}
	persistErr, notificationErr := processCollectedCycle(store, service, collector.CycleResult{})
	if persistErr != nil || notificationErr != nil || !store.called || !service.called {
		t.Fatalf("persistErr=%v notificationErr=%v store=%v service=%v", persistErr, notificationErr, store.called, service.called)
	}
	store = &fakeCycleProcessor{err: errors.New("persist")}
	service = &fakePendingCreator{}
	persistErr, _ = processCollectedCycle(store, service, collector.CycleResult{})
	if persistErr == nil || service.called {
		t.Fatalf("persistence failure reached notifications: err=%v called=%v", persistErr, service.called)
	}
}
