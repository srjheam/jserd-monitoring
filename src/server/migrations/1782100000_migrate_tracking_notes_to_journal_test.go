package migrations

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestMigrateTrackingNotesToJournalMovesAllFourFieldsAndLosesNothing(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := restoreTrackingNoteFields(app); err != nil {
		t.Fatal(err)
	}

	seedTrackingNotesFixture(t, app)

	beforeMigration := time.Now().UTC()
	if err := migrateTrackingNotesToJournal(app); err != nil {
		t.Fatal(err)
	}
	afterMigration := time.Now().UTC()

	t.Run("legacy fields are dropped", func(t *testing.T) {
		assertTrackingNoteFieldsAbsent(t, app)
	})
	t.Run("journal entries carry every legacy note without loss", func(t *testing.T) {
		assertMigratedJournalEntries(t, app, beforeMigration, afterMigration)
	})
	t.Run("empty legacy fields create no journal entries", func(t *testing.T) {
		assertNoJournalEntriesFor(t, app, "43")
	})
	t.Run("rollback restores the legacy fields", func(t *testing.T) {
		if err := restoreTrackingNoteFields(app); err != nil {
			t.Fatal(err)
		}
		assertTrackingNoteFieldsPresent(t, app)
	})
}

// seedTrackingNotesFixture creates one submission_tracking record with all
// four legacy note fields set, and one with none, so the "no spurious
// journal entry" case has a fixture to exercise.
func seedTrackingNotesFixture(t *testing.T, app core.App) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(tracking)
	record.Set(fieldOJSSubmissionID, "42")
	record.Set(fieldObservations, "20/03: sem alteração")
	record.Set(fieldStatusDetail, "Aguardando revisão")
	record.Set(fieldEditorNote, "Falar com a Morgan")
	record.Set(fieldDecisionDetail, "Aceito com ressalvas")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	// Field with no legacy note must not produce a spurious journal entry.
	empty := core.NewRecord(tracking)
	empty.Set(fieldOJSSubmissionID, "43")
	if err := app.Save(empty); err != nil {
		t.Fatal(err)
	}
}

func assertTrackingNoteFieldsAbsent(t *testing.T, app core.App) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range legacyTrackingNoteFields {
		if tracking.Fields.GetByName(field) != nil {
			t.Fatalf("legacy field %s was not dropped", field)
		}
	}
}

func assertTrackingNoteFieldsPresent(t *testing.T, app core.App) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range legacyTrackingNoteFields {
		if tracking.Fields.GetByName(field) == nil {
			t.Fatalf("rollback must restore field %s", field)
		}
	}
}

func assertMigratedJournalEntries(t *testing.T, app core.App, beforeMigration, afterMigration time.Time) {
	t.Helper()
	entries, err := app.FindRecordsByFilter("submission_journal", "ojs_submission_id = {:id}", "", 0, 0, map[string]any{"id": "42"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 migrated entries, got %d: %#v", len(entries), entries)
	}
	want := map[string]string{
		fieldObservations:   "20/03: sem alteração",
		fieldStatusDetail:   "Aguardando revisão",
		fieldEditorNote:     "Falar com a Morgan",
		fieldDecisionDetail: "Aceito com ressalvas",
	}
	got := map[string]string{}
	for _, entry := range entries {
		got[entry.GetString("author_label")] = entry.GetString("body")
		entryDate := entry.GetDateTime("entry_date").Time()
		if entryDate.Before(beforeMigration) || entryDate.After(afterMigration) {
			t.Fatalf("entry_date = %v, want between %v and %v", entryDate, beforeMigration, afterMigration)
		}
		if entry.GetString("author_id") == "" {
			t.Fatal("migrated entry must carry a non-empty author_id")
		}
	}
	for field, body := range want {
		if got[field] != body {
			t.Fatalf("field %s: got %q, want %q (lost data)", field, got[field], body)
		}
	}
}

func assertNoJournalEntriesFor(t *testing.T, app core.App, submissionID string) {
	t.Helper()
	noEntries, err := app.FindRecordsByFilter("submission_journal", "ojs_submission_id = {:id}", "", 0, 0, map[string]any{"id": submissionID})
	if err != nil {
		t.Fatal(err)
	}
	if len(noEntries) != 0 {
		t.Fatalf("empty legacy fields must not create journal entries, got %d", len(noEntries))
	}
}
