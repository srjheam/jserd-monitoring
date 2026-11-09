package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(dropAttentionCategory, restoreAttentionCategory)
}

// dropAttentionCategory removes the hand-typed attention field. Attention is
// derived from the computed editorial findings, so a chief editor never types
// one.
func dropAttentionCategory(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	collection.Fields.RemoveByName("attention_category")
	return app.Save(collection)
}

func restoreAttentionCategory(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		return err
	}
	collection.Fields.Add(&core.SelectField{Name: "attention_category", Values: []string{"awaiting_revision", "overdue", "attention", "normal", "submission_problem"}, MaxSelect: 1})
	return app.Save(collection)
}
