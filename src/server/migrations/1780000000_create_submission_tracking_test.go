package migrations

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestSubmissionTrackingMigrationCreatesLockedCollections(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := renameConferencesToEditorialEvents(app); err != nil {
		t.Fatal(err)
	}
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	// A later migration (1789000000_create_chief_editors.go) scopes both
	// collections' rules to the chief_editors auth collection; see
	// TestChiefEditorScopedAccess for that behavior.
	if tracking.Fields.GetByName("editorial_event") == nil {
		t.Fatal("tracking must reference an editorial event in this historical migration")
	}
	if err := dropSubmissionTracking(app); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("submission_tracking"); err == nil {
		t.Fatal("rollback must remove tracking before its event dependency")
	}
	if err := createSubmissionTracking(app); err != nil {
		t.Fatal(err)
	}
}

// TestTrackingCollectionsRejectGuestAPIRequests checks that a guest, with no
// chief_editors token, cannot see or change either collection through the
// API. List returns an empty page rather than a 403 because the collections
// carry a chief_editors-scoped rule (see 1789000000_create_chief_editors.go),
// not a superuser-only nil rule.
func TestTrackingCollectionsRejectGuestAPIRequests(t *testing.T) {
	_, mux, _ := apiFixture(t)
	expected := map[string]int{
		http.MethodGet:    http.StatusOK,
		http.MethodPost:   http.StatusBadRequest,
		http.MethodPatch:  http.StatusNotFound,
		http.MethodDelete: http.StatusNotFound,
	}
	for _, collection := range []string{"conferences", "submission_tracking"} {
		for method, want := range expected {
			path := "/api/collections/" + collection + "/records"
			if method == http.MethodPatch || method == http.MethodDelete {
				path += testMissingPath
			}
			body := []byte(`{"name":"x","ojs_submission_id":"1"}`)
			response := apiCall(mux, method, path, body, "")
			if response.Code != want {
				t.Fatalf("guest %s %s=%d want %d: %s", method, collection, response.Code, want, response.Body.String())
			}
		}
		if response := apiCall(mux, http.MethodGet, "/api/collections/"+collection+"/records", nil, ""); !bytes.Contains(response.Body.Bytes(), []byte(`"totalItems":0`)) {
			t.Fatalf("guest list %s must return no rows: %s", collection, response.Body.String())
		}
	}
}

func TestTrackingCollectionsSuperuserAPICRUDAndRelation(t *testing.T) {
	_, mux, token := apiFixture(t)
	conference := apiCall(mux, http.MethodPost, "/api/collections/conferences/records", []byte(`{"name":"Issue"}`), token)
	if conference.Code != http.StatusOK {
		t.Fatalf("create conference=%d %s", conference.Code, conference.Body.String())
	}
	conferenceID := jsonID(t, conference)
	if response := apiCall(mux, http.MethodGet, "/api/collections/conferences/records", nil, token); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(conferenceID)) {
		t.Fatalf("list conference=%d %s", response.Code, response.Body.String())
	}
	if response := apiCall(mux, http.MethodPatch, "/api/collections/conferences/records/"+conferenceID, []byte(`{"description":"Updated"}`), token); response.Code != http.StatusOK {
		t.Fatalf("update conference=%d", response.Code)
	}
	if response := apiCall(mux, http.MethodGet, "/api/collections/conferences/records/"+conferenceID, nil, token); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"description":"Updated"`)) {
		t.Fatalf("read conference=%d %s", response.Code, response.Body.String())
	}
	tracking := apiCall(mux, http.MethodPost, "/api/collections/submission_tracking/records", []byte(`{"ojs_submission_id":"42","conference":"`+conferenceID+`"}`), token)
	if tracking.Code != http.StatusOK {
		t.Fatalf("create tracking=%d %s", tracking.Code, tracking.Body.String())
	}
	trackingID := jsonID(t, tracking)
	read := apiCall(mux, http.MethodGet, "/api/collections/submission_tracking/records/"+trackingID, nil, token)
	if read.Code != http.StatusOK || !bytes.Contains(read.Body.Bytes(), []byte(`"conference":"`+conferenceID+`"`)) {
		t.Fatalf("read relation=%d %s", read.Code, read.Body.String())
	}
	if response := apiCall(mux, http.MethodGet, "/api/collections/submission_tracking/records", nil, token); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(trackingID)) {
		t.Fatalf("list tracking=%d", response.Code)
	}
	if response := apiCall(mux, http.MethodPatch, "/api/collections/submission_tracking/records/"+trackingID, []byte(`{"conference":""}`), token); response.Code != http.StatusOK {
		t.Fatalf("disassociate=%d", response.Code)
	}
	if response := apiCall(mux, http.MethodPost, "/api/collections/submission_tracking/records", []byte(`{"ojs_submission_id":"42"}`), token); response.Code != http.StatusBadRequest {
		t.Fatalf("unique=%d", response.Code)
	}
	if response := apiCall(mux, http.MethodDelete, "/api/collections/submission_tracking/records/"+trackingID, nil, token); response.Code != http.StatusNoContent {
		t.Fatalf("delete tracking=%d", response.Code)
	}
	if response := apiCall(mux, http.MethodDelete, "/api/collections/conferences/records/"+conferenceID, nil, token); response.Code != http.StatusNoContent {
		t.Fatalf("delete conference=%d", response.Code)
	}
}

func TestLegacyEditorialEventAPIIsAbsent(t *testing.T) {
	_, mux, token := apiFixture(t)
	if response := apiCall(mux, http.MethodGet, "/api/collections/editorial_events/records", nil, token); response.Code != http.StatusNotFound {
		t.Fatalf("legacy editorial_events API path must be absent, got %d", response.Code)
	}
}

// The Muted screen clusters submissions by conference name, so two
// conferences sharing a name would merge into one cluster with an over-reported
// count. idx_conferences_name, created by the forward migration, prevents that.
func TestConferenceNameIsUnique(t *testing.T) {
	_, mux, token := apiFixture(t)
	first := apiCall(mux, http.MethodPost, "/api/collections/conferences/records", []byte(`{"name":"SBQS"}`), token)
	if first.Code != http.StatusOK {
		t.Fatalf("create first=%d %s", first.Code, first.Body.String())
	}
	second := apiCall(mux, http.MethodPost, "/api/collections/conferences/records", []byte(`{"name":"SBQS"}`), token)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("create duplicate=%d %s, want 400", second.Code, second.Body.String())
	}
}

func apiFixture(t *testing.T) (*tests.TestApp, http.Handler, string) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	superusers, _ := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	operator := core.NewRecord(superusers)
	operator.SetEmail("operator@example.test")
	operator.SetPassword("valid-password")
	if err := app.Save(operator); err != nil {
		t.Fatal(err)
	}
	token, err := operator.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
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
	return app, mux, token
}

func apiCall(mux http.Handler, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", token)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func jsonID(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var value struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.ID == "" {
		t.Fatalf("missing id in %s", response.Body.String())
	}
	return value.ID
}

func TestTrackingValidationAndRelationIntegrity(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	conferences, _ := app.FindCollectionByNameOrId("conferences")
	conference := core.NewRecord(conferences)
	conference.Set(fieldName, "Issue")
	if err := app.Save(conference); err != nil {
		t.Fatal(err)
	}
	tracking, _ := app.FindCollectionByNameOrId("submission_tracking")
	record := core.NewRecord(tracking)
	record.Set(fieldOJSSubmissionID, "42")
	record.Set("conference", conference.Id)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	if record.GetString("conference") != conference.Id {
		t.Fatal("relation was not saved")
	}
	duplicate := core.NewRecord(tracking)
	duplicate.Set(fieldOJSSubmissionID, "42")
	if err := app.Save(duplicate); err == nil {
		t.Fatal("unique ojs submission id was not enforced")
	}
	record.Set("conference", "")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	if err := app.Delete(record); err != nil {
		t.Fatal(err)
	}
	if err := app.Delete(conference); err != nil {
		t.Fatal(err)
	}
}
