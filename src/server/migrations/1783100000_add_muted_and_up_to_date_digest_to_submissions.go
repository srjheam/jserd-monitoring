package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

// mutedUpToDateIndex lets the Muted and Up to date screens read their rows in
// display order with an index seek.
const mutedUpToDateIndex = "CREATE INDEX `idx_ojs_submissions_muted_up_to_date` ON `ojs_submissions` (`muted`, `up_to_date_digest`, `effective_priority`, `ojs_submission_id` COLLATE NOCASE)"

func init() {
	migrations.Register(addMutedAndUpToDateDigestToSubmissions, dropMutedAndUpToDateDigestFromSubmissions)
}

// addMutedAndUpToDateDigestToSubmissions mirrors submission_tracking.muted and
// submission_tracking.up_to_date_digest onto the materialized row, the same
// way effective_priority and finding_digest mirror the tracking-derived
// state (see 1782100000_add_priority_and_finding_digest.go). The mirrored
// columns let the work list be indexed and filtered without joining back to
// submission_tracking, and extend effective_queue with the Muted and Up to
// date placements.
func addMutedAndUpToDateDigestToSubmissions(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	submissions.Fields.Add(
		&core.BoolField{Name: fieldMuted},
		&core.TextField{Name: fieldUpToDateDigest},
	)
	if queue, ok := submissions.Fields.GetByName("effective_queue").(*core.SelectField); ok {
		queue.Values = []string{queueValueDaily, fieldEvent, queueValueArchived, fieldMuted, queueValueUpToDate}
	}
	submissions.Indexes = append(submissions.Indexes, mutedUpToDateIndex)
	return app.Save(submissions)
}

func dropMutedAndUpToDateDigestFromSubmissions(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	submissions.Fields.RemoveByName(fieldMuted)
	submissions.Fields.RemoveByName(fieldUpToDateDigest)
	if queue, ok := submissions.Fields.GetByName("effective_queue").(*core.SelectField); ok {
		queue.Values = []string{queueValueDaily, fieldEvent, queueValueArchived}
	}
	submissions.Indexes = withoutIndex(submissions.Indexes, mutedUpToDateIndex)
	return app.Save(submissions)
}

// withoutIndex drops one index definition from a list.
func withoutIndex(indexes []string, drop string) []string {
	kept := make([]string, 0, len(indexes))
	for _, index := range indexes {
		if index != drop {
			kept = append(kept, index)
		}
	}
	return kept
}
