package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createSubmissionJournal, dropSubmissionJournal)
}

// createSubmissionJournal adds the append-only journal that replaces the
// single-cell dated notes previously kept in submission_tracking free-text
// fields. List/View/Create/Update/Delete are all left without an API rule so
// the collection is reachable only through backend code and superuser
// tooling; genuine append-only enforcement (including for superusers) comes
// from the OnRecordUpdateExecute/OnRecordDeleteExecute hooks registered in
// internal/state/hooks.go.
func createSubmissionJournal(app core.App) error {
	journal := core.NewBaseCollection("submission_journal")
	journal.Fields.Add(
		&core.TextField{Name: fieldOJSSubmissionID, Required: true},
		&core.DateField{Name: "entry_date", Required: true},
		&core.TextField{Name: "author_id", Required: true},
		&core.TextField{Name: "author_label"},
		&core.TextField{Name: "body", Required: true},
	)
	journal.Indexes = append(journal.Indexes, "CREATE INDEX `idx_submission_journal_ojs_submission_id` ON `submission_journal` (`ojs_submission_id`)")
	return app.Save(journal)
}

func dropSubmissionJournal(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("submission_journal")
	if err != nil {
		return err
	}
	return app.Delete(collection)
}
