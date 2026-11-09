package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(dropLocalStateAndDecisionDate, restoreLocalStateAndDecisionDate)
}

// dropLocalStateAndDecisionDate removes the chief-editor-set local state and
// decision date. OJS's own Decisions list is the sole source of a
// submission's decision and final state.
func dropLocalStateAndDecisionDate(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	collection.Fields.RemoveByName(fieldLocalState)
	collection.Fields.RemoveByName(fieldDecisionDate)
	return app.Save(collection)
}

// restoreLocalStateAndDecisionDate re-adds the fields but not their data:
// this removal is one-way, per the ADR.
func restoreLocalStateAndDecisionDate(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	collection.Fields.Add(
		&core.SelectField{Name: fieldLocalState, Values: []string{fieldActive, "accepted", "rejected", "withdrawn", "other_final"}, MaxSelect: 1},
		&core.DateField{Name: fieldDecisionDate},
	)
	return app.Save(collection)
}
