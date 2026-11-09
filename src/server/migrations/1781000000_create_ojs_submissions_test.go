package migrations

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestOjsSubmissionsMigrationCreatesLockedCollection(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	// CreateRule/UpdateRule/DeleteRule stay superuser-only forever: every
	// field is server-derived. A later migration
	// (1789000000_create_chief_editors.go) opens ListRule/ViewRule to the
	// chief_editors auth collection, read-only; see TestChiefEditorScopedAccess.
	if submissions.CreateRule != nil || submissions.UpdateRule != nil || submissions.DeleteRule != nil {
		t.Fatal("ojs_submissions must stay write-locked")
	}
	for _, field := range []string{fieldOJSSubmissionID, "title", fieldStatus, "effective_final_state", "effective_queue", "effective_attention", "evidence"} {
		if submissions.Fields.GetByName(field) == nil {
			t.Fatalf("missing field %s", field)
		}
	}
	if err := dropOjsSubmissions(app); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("ojs_submissions"); err == nil {
		t.Fatal("rollback must remove ojs_submissions")
	}
	if err := createOjsSubmissions(app); err != nil {
		t.Fatal(err)
	}
}

// TestOjsSubmissionsRejectsGuestAPIRequests checks that a guest, with no
// chief_editors token, cannot view or create a submission. A single-record
// GET returns 404 rather than 403 because ListRule/ViewRule are scoped to
// chief_editors (see 1789000000_create_chief_editors.go), not a superuser-only
// nil rule; POST stays 403 because CreateRule is unchanged.
func TestOjsSubmissionsRejectsGuestAPIRequests(t *testing.T) {
	_, mux, _ := apiFixture(t)
	expected := map[string]int{http.MethodGet: http.StatusNotFound, http.MethodPost: http.StatusForbidden}
	for method, want := range expected {
		path := "/api/collections/ojs_submissions/records"
		if method == http.MethodGet {
			path += testMissingPath
		}
		if status := apiCall(mux, method, path, nil, "").Code; status != want {
			t.Fatalf("guest %s status=%d want %d", method, status, want)
		}
	}
}

func TestOjsSubmissionsSuperuserAPICRUDAndUniqueIndex(t *testing.T) {
	_, mux, token := apiFixture(t)
	body := []byte(`{"ojs_submission_id":"42","title":"Fixture","status":"Queued","effective_final_state":"","effective_queue":"daily","effective_attention":"unknown","evidence":{"url":"https://ojs.example.test/submission/42"}}`)
	if response := apiCall(mux, http.MethodPost, "/api/collections/ojs_submissions/records", body, token); response.Code != http.StatusOK {
		t.Fatalf("create submission=%d %s", response.Code, response.Body.String())
	}
	if response := apiCall(mux, http.MethodPost, "/api/collections/ojs_submissions/records", body, token); response.Code != http.StatusBadRequest {
		t.Fatalf("unique index not enforced=%d %s", response.Code, response.Body.String())
	}
	if response := apiCall(mux, http.MethodPost, "/api/collections/ojs_submissions/records", []byte(`{"ojs_submission_id":"43"}`), token); response.Code != http.StatusBadRequest {
		t.Fatalf("required fields not enforced=%d", response.Code)
	}
	list := apiCall(mux, http.MethodGet, "/api/collections/ojs_submissions/records", nil, token)
	if list.Code != http.StatusOK {
		t.Fatalf("list submissions=%d %s", list.Code, list.Body.String())
	}
	var page struct {
		TotalItems int `json:"totalItems"`
	}
	if err := decodeJSON(list.Body.Bytes(), &page); err != nil || page.TotalItems != 1 {
		t.Fatalf("list total=%d err=%v", page.TotalItems, err)
	}
}

func decodeJSON(payload []byte, target any) error {
	return json.Unmarshal(payload, target)
}

func TestOjsSubmissionsRowShape(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(submissions)
	record.Set(fieldOJSSubmissionID, "42")
	record.Set("title", "Fixture")
	record.Set(fieldStatus, "Queued")
	record.Set("effective_queue", queueValueDaily)
	record.Set("effective_attention", "unknown")
	record.Set("evidence", map[string]any{"url": "https://ojs.example.test/42"})
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	stored, err := app.FindFirstRecordByData("ojs_submissions", fieldOJSSubmissionID, "42")
	if err != nil {
		t.Fatal(err)
	}
	if stored.GetString("effective_queue") != queueValueDaily {
		t.Fatalf("row=%#v", stored)
	}
}
