package migrations

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestChiefEditorsMigrationCreatesAuthCollection(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := renameConferencesToEditorialEvents(app); err != nil {
		t.Fatal(err)
	}
	chiefEditors, err := app.FindCollectionByNameOrId("chief_editors")
	if err != nil {
		t.Fatal(err)
	}
	if !chiefEditors.IsAuth() {
		t.Fatal("chief_editors must be an auth collection")
	}
	if err := dropChiefEditors(app); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("chief_editors"); err == nil {
		t.Fatal("rollback must remove chief_editors")
	}
	if err := createChiefEditors(app); err != nil {
		t.Fatal(err)
	}
}

func TestChiefEditorScopedAccess(t *testing.T) {
	app, mux, casey := chiefEditorFixture(t, "casey@example.test")
	editorTwo := signInAgainst(t, app, "chief_editors", "taycasey@example.test")
	submission := seedScopedAccessSubmission(t, app)

	t.Run("chief editor can list but not update ojs_submissions", func(t *testing.T) {
		assertOjsSubmissionsReadOnly(t, mux, casey, submission)
	})
	t.Run("chief editor can delete a conference created by another chief editor", func(t *testing.T) {
		assertCrossChiefEditorDelete(t, mux, casey, editorTwo, "conferences", []byte(`{"name":"Issue"}`))
	})
	t.Run("chief editor can delete tracking created by another chief editor", func(t *testing.T) {
		assertCrossChiefEditorDelete(t, mux, casey, editorTwo, "submission_tracking", []byte(`{"ojs_submission_id":"42"}`))
	})
	t.Run("submission_journal stays unreachable by direct API rule", func(t *testing.T) {
		assertSubmissionJournalUnreachable(t, mux, casey)
	})
}

func seedScopedAccessSubmission(t *testing.T, app *tests.TestApp) *core.Record {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	submission := core.NewRecord(submissions)
	submission.Set(fieldOJSSubmissionID, "42")
	submission.Set("effective_queue", queueValueDaily)
	submission.Set("evidence", map[string]any{"url": "https://ojs.example.test/42"})
	if err := app.Save(submission); err != nil {
		t.Fatal(err)
	}
	return submission
}

func assertOjsSubmissionsReadOnly(t *testing.T, mux http.Handler, token string, submission *core.Record) {
	t.Helper()
	if response := apiCall(mux, http.MethodGet, "/api/collections/ojs_submissions/records", nil, token); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(submission.Id)) {
		t.Fatalf("list submissions=%d %s", response.Code, response.Body.String())
	}
	if response := apiCall(mux, http.MethodPatch, "/api/collections/ojs_submissions/records/"+submission.Id, []byte(`{"title":"Rewritten"}`), token); response.Code != http.StatusForbidden {
		t.Fatalf("chief editor must not update ojs_submissions, got %d", response.Code)
	}
}

// assertCrossChiefEditorDelete creates a record on collectionName as
// creatorToken, then checks that deleterToken (a different chief editor)
// can delete it.
func assertCrossChiefEditorDelete(t *testing.T, mux http.Handler, creatorToken, deleterToken, collectionName string, body []byte) {
	t.Helper()
	created := apiCall(mux, http.MethodPost, "/api/collections/"+collectionName+"/records", body, creatorToken)
	if created.Code != http.StatusOK {
		t.Fatalf("create %s=%d %s", collectionName, created.Code, created.Body.String())
	}
	id := jsonID(t, created)
	if response := apiCall(mux, http.MethodDelete, "/api/collections/"+collectionName+"/records/"+id, nil, deleterToken); response.Code != http.StatusNoContent {
		t.Fatalf("a chief editor must delete a %s record created by another chief editor, got %d", collectionName, response.Code)
	}
}

func assertSubmissionJournalUnreachable(t *testing.T, mux http.Handler, token string) {
	t.Helper()
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/api/collections/submission_journal/records"
		if response := apiCall(mux, method, path, []byte(`{}`), token); response.Code != http.StatusForbidden {
			t.Fatalf("submission_journal must stay unreachable by direct API rule, got %d for %s", response.Code, method)
		}
	}
}

func chiefEditorFixture(t *testing.T, email string) (*tests.TestApp, http.Handler, string) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
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
	return app, mux, signInAgainst(t, app, "chief_editors", email)
}

// signInAgainst creates and authenticates a record on the named auth
// collection, returning its token.
func signInAgainst(t *testing.T, app *tests.TestApp, collectionName, email string) string {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.SetEmail(email)
	record.SetPassword("valid-password")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	token, err := record.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}
