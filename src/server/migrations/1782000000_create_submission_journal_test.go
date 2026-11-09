package migrations

import (
	"net/http"
	"server/internal/state"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestSubmissionJournalMigrationCreatesLockedCollection(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	journal, err := app.FindCollectionByNameOrId("submission_journal")
	if err != nil {
		t.Fatal(err)
	}
	if journal.ListRule != nil || journal.ViewRule != nil || journal.CreateRule != nil || journal.UpdateRule != nil || journal.DeleteRule != nil {
		t.Fatal("submission_journal must be locked")
	}
	for _, field := range []string{fieldOJSSubmissionID, "entry_date", "author_id", "author_label", "body"} {
		if journal.Fields.GetByName(field) == nil {
			t.Fatalf("missing field %s", field)
		}
	}
	if err := dropSubmissionJournal(app); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("submission_journal"); err == nil {
		t.Fatal("rollback must remove submission_journal")
	}
	if err := createSubmissionJournal(app); err != nil {
		t.Fatal(err)
	}
}

func TestSubmissionJournalRejectsGuestAPIRequests(t *testing.T) {
	_, mux, _ := apiFixture(t)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/api/collections/submission_journal/records"
		if method == http.MethodGet {
			path += testMissingPath
		}
		if status := apiCall(mux, method, path, nil, "").Code; status != http.StatusForbidden {
			t.Fatalf("guest %s status=%d", method, status)
		}
	}
}

func TestSubmissionJournalEntryRejectsUpdateAndDelete(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	state.RegisterImmutableHooks(app)
	entry := saveJournalEntry(t, app, "42", "2026-06-01 00:00:00.000Z", fieldOperator, "Editor", "sem alteração")
	entry.Set("body", "alterado")
	if err := app.Save(entry); err == nil {
		t.Fatal("journal update unexpectedly allowed")
	}
	if err := app.Delete(entry); err == nil {
		t.Fatal("journal delete unexpectedly allowed")
	}
}

func saveJournalEntry(t *testing.T, app core.App, submissionID, entryDate, authorID, authorLabel, body string) *core.Record {
	t.Helper()
	journal, err := app.FindCollectionByNameOrId("submission_journal")
	if err != nil {
		t.Fatal(err)
	}
	entry := core.NewRecord(journal)
	entry.Set(fieldOJSSubmissionID, submissionID)
	entry.Set("entry_date", entryDate)
	entry.Set("author_id", authorID)
	entry.Set("author_label", authorLabel)
	entry.Set("body", body)
	if err := app.Save(entry); err != nil {
		t.Fatal(err)
	}
	return entry
}
