package migrations

import (
	"slices"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

// scopedSubmissionIndexes are the indexes that let the default list query and
// its keyset cursor use an index for ordering instead of falling back to a
// temp b-tree sort, and that let the queue/attention and queue/title/id
// filters used by the dashboard hit an index instead of a full table scan.
var scopedSubmissionIndexes = []string{
	"CREATE INDEX `idx_ojs_submissions_ojs_submission_id_nocase` ON `ojs_submissions` (`ojs_submission_id` COLLATE NOCASE)",
	"CREATE INDEX `idx_ojs_submissions_queue_attention` ON `ojs_submissions` (`effective_queue`, `effective_attention`)",
	"CREATE INDEX `idx_ojs_submissions_queue_title_id` ON `ojs_submissions` (`effective_queue`, `title` COLLATE NOCASE, `ojs_submission_id` COLLATE NOCASE)",
	"CREATE INDEX `idx_ojs_submissions_effective_final_state` ON `ojs_submissions` (`effective_final_state`)",
	// Not in the ticket's list verbatim, but required for its own Definition of
	// Done: the default list query filters `effective_queue = 'daily'` and
	// sorts by `ojs_submission_id COLLATE NOCASE` (sort=id is the default sort
	// key). None of the four indexes above cover that combination, so SQLite
	// still fell back to `USE TEMP B-TREE FOR ORDER BY` for the default query
	// even with them in place. This composite mirrors the queue+title+id index
	// above, but for the default (id) sort key.
	"CREATE INDEX `idx_ojs_submissions_queue_id` ON `ojs_submissions` (`effective_queue`, `ojs_submission_id` COLLATE NOCASE)",
}

func init() {
	migrations.Register(scopeSubmissionQueries, unscopeSubmissionQueries)
}

func scopeSubmissionQueries(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	submissions.Indexes = append(submissions.Indexes, scopedSubmissionIndexes...)
	return app.Save(submissions)
}

func unscopeSubmissionQueries(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(submissions.Indexes))
	for _, index := range submissions.Indexes {
		if !slices.Contains(scopedSubmissionIndexes, index) {
			kept = append(kept, index)
		}
	}
	submissions.Indexes = kept
	return app.Save(submissions)
}
