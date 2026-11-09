package migrations

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

// legacyTrackingNoteFields are the free-text submission_tracking fields being
// replaced by dated submission_journal entries. legacyJournalAuthorID marks
// entries created by this migration, since the original notes carried no
// author.
//
// The ticket asks to date each migrated entry by "the tracking record's
// updated value", but submission_tracking (see
// 1780000000_create_submission_tracking.go) never defined a created/updated
// autodate field, so no such history was ever recorded for existing rows.
// There is nothing to recover, so migrated entries are dated by the moment
// this migration runs instead.
const legacyJournalAuthorID = "legacy-migration"

var legacyTrackingNoteFields = []string{fieldObservations, fieldStatusDetail, fieldEditorNote, fieldDecisionDetail}

func init() {
	migrations.Register(migrateTrackingNotesToJournal, restoreTrackingNoteFields)
}

func migrateTrackingNotesToJournal(app core.App) error {
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	if err := copyTrackingNotes(app, tracking); err != nil {
		return err
	}
	for _, field := range legacyTrackingNoteFields {
		tracking.Fields.RemoveByName(field)
	}
	return app.Save(tracking)
}

// copyTrackingNotes turns every stored note into a journal entry, stamped with
// the moment of the migration.
func copyTrackingNotes(app core.App, tracking *core.Collection) error {
	journal, err := app.FindCollectionByNameOrId("submission_journal")
	if err != nil {
		return err
	}
	records, err := app.FindAllRecords(tracking)
	if err != nil {
		return err
	}
	return writeJournalForAll(app, journal, records, time.Now().UTC())
}

// writeJournalForAll turns the notes on every tracking record into journal
// entries.
func writeJournalForAll(app core.App, journal *core.Collection, records []*core.Record, migratedAt time.Time) error {
	for _, record := range records {
		if err := writeJournalEntries(app, journal, record, migratedAt); err != nil {
			return err
		}
	}
	return nil
}

// writeJournalEntries turns the notes on one tracking record into journal
// entries, one per note field that carries text.
func writeJournalEntries(app core.App, journal *core.Collection, record *core.Record, migratedAt time.Time) error {
	for _, field := range legacyTrackingNoteFields {
		if err := writeJournalEntry(app, journal, record, field, migratedAt); err != nil {
			return err
		}
	}
	return nil
}

// writeJournalEntry writes one note as a journal entry. An empty note becomes
// no entry.
func writeJournalEntry(app core.App, journal *core.Collection, record *core.Record, field string, migratedAt time.Time) error {
	value := record.GetString(field)
	if value == "" {
		return nil
	}
	entry := core.NewRecord(journal)
	entry.Set(fieldOJSSubmissionID, record.GetString(fieldOJSSubmissionID))
	entry.Set("entry_date", migratedAt)
	entry.Set("author_id", legacyJournalAuthorID)
	entry.Set("author_label", field)
	entry.Set("body", value)
	return app.Save(entry)
}

func restoreTrackingNoteFields(app core.App) error {
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	tracking.Fields.Add(
		&core.TextField{Name: fieldStatusDetail},
		&core.TextField{Name: fieldEditorNote},
		&core.TextField{Name: fieldObservations},
		&core.TextField{Name: fieldDecisionDetail},
	)
	return app.Save(tracking)
}
