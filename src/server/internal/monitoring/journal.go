package monitoring

import (
	"server/internal/collections"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func findOrCreateTrackingRecord(app core.App, ojsSubmissionID string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter(collections.SubmissionTracking, "ojs_submission_id = {:id}", "", 1, 0, dbx.Params{"id": ojsSubmissionID})
	if err != nil {
		return nil, err
	}
	if len(records) > 0 {
		return records[0], nil
	}
	collection, err := app.FindCollectionByNameOrId(collections.SubmissionTracking)
	if err != nil {
		return nil, err
	}
	record := core.NewRecord(collection)
	record.Set("ojs_submission_id", ojsSubmissionID)
	return record, nil
}

// appendJournalEntryRecord is the one writer of submission_journal, shared by
// the plain note route, "mark up to date" and "send back to work": every
// journal entry carries the same five fields, whichever action wrote it.
func appendJournalEntryRecord(app core.App, ojsSubmissionID, body string, auth *core.Record) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId(collections.SubmissionJournal)
	if err != nil {
		return nil, err
	}
	entry := core.NewRecord(collection)
	entry.Set("ojs_submission_id", ojsSubmissionID)
	entry.Set("entry_date", time.Now().UTC())
	entry.Set("body", body)
	// AUTHOR-IDENTITY: reads the signed-in chief_editors record.
	entry.Set("author_id", auth.Id)
	entry.Set("author_label", auth.GetString("email"))
	if err := app.Save(entry); err != nil {
		return nil, err
	}
	return entry, nil
}
