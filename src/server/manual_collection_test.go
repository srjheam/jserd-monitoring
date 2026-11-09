package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// manualCollectionMux wires registerManualCollectionHooks onto app and builds
// the same request mux the running server would serve manual collection
// requests through.
func manualCollectionMux(t *testing.T, app core.App, runner editorialRunner) http.Handler {
	t.Helper()
	registerManualCollectionHooks(app, runner)

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
	return mux
}

func postManualCollectionRequest(t *testing.T, mux http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/collections/manual_collection_requests/records", bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

// TestManualCollectionRequestHookChainRunsTheRequest drives a real HTTP
// record-create request through registerManualCollectionHooks: checkManualRequest
// gates it, and its success starts executeManualRequest in the background via
// startManualRequest/startupRecovery.
func TestManualCollectionRequestHookChainRunsTheRequest(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()

	runner := &fakeEditorialRunner{}
	mux := manualCollectionMux(t, app, runner)
	operator := createTestSuperuser(t, app)
	token, err := operator.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}

	response := postManualCollectionRequest(t, mux, token, `{"confirmed":true}`)
	if response.Code != http.StatusOK {
		t.Fatalf("create=%d %s", response.Code, response.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for runner.calls == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if runner.calls != 1 {
		t.Fatalf("expected the manual collection to run once in the background, calls=%d", runner.calls)
	}
}

// TestManualCollectionRequestHookChainRejectsUnconfirmed exercises
// checkManualRequest's rejection path when the request omits confirmation.
func TestManualCollectionRequestHookChainRejectsUnconfirmed(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()

	mux := manualCollectionMux(t, app, &fakeEditorialRunner{})
	operator := createTestSuperuser(t, app)
	token, err := operator.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}

	response := postManualCollectionRequest(t, mux, token, `{"confirmed":false}`)
	if response.Code == http.StatusOK {
		t.Fatalf("expected the create to be rejected, got %d %s", response.Code, response.Body.String())
	}
}
