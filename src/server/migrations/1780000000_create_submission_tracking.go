package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createSubmissionTracking, dropSubmissionTracking)
}

func createSubmissionTracking(app core.App) error {
	events := core.NewBaseCollection("editorial_events")
	events.Fields.Add(
		&core.TextField{Name: fieldName, Required: true},
		&core.TextField{Name: "description"},
	)
	events.Indexes = append(events.Indexes, "CREATE UNIQUE INDEX `idx_editorial_events_name` ON `editorial_events` (`name`)")
	if err := app.Save(events); err != nil {
		return err
	}

	tracking := core.NewBaseCollection("submission_tracking")
	tracking.Fields.Add(
		&core.TextField{Name: fieldOJSSubmissionID, Required: true},
		&core.SelectField{Name: fieldLocalState, Values: []string{fieldActive, "accepted", "rejected", "withdrawn", "other_final"}, MaxSelect: 1},
		&core.TextField{Name: fieldStatusDetail},
		&core.SelectField{Name: "attention_category", Values: []string{"awaiting_revision", "overdue", "attention", "normal", "submission_problem"}, MaxSelect: 1},
		&core.TextField{Name: fieldEditorNote},
		&core.DateField{Name: "last_verified_at"},
		&core.TextField{Name: "verified_by"},
		&core.TextField{Name: fieldObservations},
		&core.DateField{Name: fieldDecisionDate},
		&core.TextField{Name: fieldDecisionDetail},
		&core.RelationField{Name: "editorial_event", CollectionId: events.Id, MaxSelect: 1, CascadeDelete: false},
	)
	tracking.Indexes = append(tracking.Indexes, "CREATE UNIQUE INDEX `idx_submission_tracking_ojs_submission_id` ON `submission_tracking` (`ojs_submission_id`)")
	return app.Save(tracking)
}

func dropSubmissionTracking(app core.App) error {
	for _, name := range []string{"submission_tracking", "editorial_events"} {
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
