package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createOjsSubmissions, dropOjsSubmissions)
}

const evidenceMaxSize = 5 << 20

func createOjsSubmissions(app core.App) error {
	submissions := core.NewBaseCollection("ojs_submissions")
	submissions.Fields.Add(
		&core.TextField{Name: fieldOJSSubmissionID, Required: true},
		&core.TextField{Name: "title"},
		&core.TextField{Name: fieldStatus},
		&core.TextField{Name: "effective_final_state"},
		&core.SelectField{Name: "effective_queue", Values: []string{queueValueDaily, fieldEvent, queueValueArchived}, Required: true, MaxSelect: 1},
		&core.TextField{Name: "effective_attention"},
		&core.JSONField{Name: "evidence", Required: true, MaxSize: evidenceMaxSize},
	)
	submissions.Indexes = append(submissions.Indexes,
		"CREATE UNIQUE INDEX `idx_ojs_submissions_ojs_submission_id` ON `ojs_submissions` (`ojs_submission_id`)",
		"CREATE INDEX `idx_ojs_submissions_effective_queue` ON `ojs_submissions` (`effective_queue`)",
		"CREATE INDEX `idx_ojs_submissions_status` ON `ojs_submissions` (`status`)",
		"CREATE INDEX `idx_ojs_submissions_effective_attention` ON `ojs_submissions` (`effective_attention`)",
		"CREATE INDEX `idx_ojs_submissions_title_nocase` ON `ojs_submissions` (`title` COLLATE NOCASE)",
	)
	return app.Save(submissions)
}

func dropOjsSubmissions(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	return app.Delete(collection)
}
