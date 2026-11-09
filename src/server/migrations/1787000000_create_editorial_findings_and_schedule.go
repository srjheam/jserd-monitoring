package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createEditorialFindingsAndSchedule, dropEditorialFindingsAndSchedule)
}

// createEditorialFindingsAndSchedule adds the storage the unified cycle
// needs: editorial_findings holds the latest cycle's findings, replaced
// wholesale every cycle, and editorial_schedule is a one-row table holding
// the forward-only next_summary_due_at timestamp.
func createEditorialFindingsAndSchedule(app core.App) error {
	findings := core.NewBaseCollection("editorial_findings")
	findings.Fields.Add(
		&core.TextField{Name: "submission_key", Required: true},
		&core.TextField{Name: "date_submitted"},
		&core.JSONField{Name: fieldPayload, Required: true},
	)
	findings.Indexes = append(findings.Indexes,
		"CREATE UNIQUE INDEX `idx_editorial_findings_submission_key` ON `editorial_findings` (`submission_key`)",
		"CREATE INDEX `idx_editorial_findings_date_submitted` ON `editorial_findings` (`date_submitted`)",
	)
	if err := app.Save(findings); err != nil {
		return err
	}

	schedule := core.NewBaseCollection("editorial_schedule")
	schedule.Fields.Add(&core.DateField{Name: "next_summary_due_at"})
	return app.Save(schedule)
}

func dropEditorialFindingsAndSchedule(app core.App) error {
	for _, name := range []string{"editorial_schedule", "editorial_findings"} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return err
		}
		if err := app.Delete(collection); err != nil {
			return err
		}
	}
	return nil
}
