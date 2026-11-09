package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addPriorityAndFindingDigest, dropPriorityAndFindingDigest)
}

func addPriorityAndFindingDigest(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	submissions.Fields.Add(
		&core.NumberField{Name: fieldEffectivePriority},
		&core.TextField{Name: fieldFindingDigest},
	)
	submissions.Indexes = append(submissions.Indexes,
		"CREATE INDEX `idx_ojs_submissions_effective_priority` ON `ojs_submissions` (`effective_priority`)",
	)
	return app.Save(submissions)
}

func dropPriorityAndFindingDigest(app core.App) error {
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		return err
	}
	submissions.Fields.RemoveByName(fieldEffectivePriority)
	submissions.Fields.RemoveByName(fieldFindingDigest)
	for i, index := range submissions.Indexes {
		if index == "CREATE INDEX `idx_ojs_submissions_effective_priority` ON `ojs_submissions` (`effective_priority`)" {
			submissions.Indexes = append(submissions.Indexes[:i], submissions.Indexes[i+1:]...)
			break
		}
	}
	return app.Save(submissions)
}
