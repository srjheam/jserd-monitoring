package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// chiefEditorRule scopes an access rule to a signed-in chief_editors record.
// It carries no ownership filter: every rule that uses it grants the same
// access to every chief editor, including over an editorial_events or
// submission_tracking record created by a different chief editor.
var chiefEditorRule = types.Pointer(`@request.auth.collectionName = "chief_editors"`)

func init() {
	migrations.Register(createChiefEditors, dropChiefEditors)
}

func createChiefEditors(app core.App) error {
	chiefEditors := core.NewAuthCollection("chief_editors")
	if err := app.Save(chiefEditors); err != nil {
		return err
	}
	// ojs_submissions stays read-only for a chief editor: every field on it is
	// server-derived from the OJS collection cycle, not editor input.
	if err := scopeAccess(app, "ojs_submissions", chiefEditorRule, chiefEditorRule, nil, nil, nil); err != nil {
		return err
	}
	if err := scopeAccess(app, "submission_tracking", chiefEditorRule, chiefEditorRule, chiefEditorRule, chiefEditorRule, chiefEditorRule); err != nil {
		return err
	}
	return scopeAccess(app, "editorial_events", chiefEditorRule, chiefEditorRule, chiefEditorRule, chiefEditorRule, chiefEditorRule)
}

// scopeAccess sets the List/View/Create/Update/Delete rules on the named
// collection. A nil rule leaves that action superuser-only.
func scopeAccess(app core.App, name string, list, view, create, update, deleteRule *string) error {
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		return err
	}
	collection.ListRule = list
	collection.ViewRule = view
	collection.CreateRule = create
	collection.UpdateRule = update
	collection.DeleteRule = deleteRule
	return app.Save(collection)
}

func dropChiefEditors(app core.App) error {
	for _, name := range []string{"editorial_events", "submission_tracking", "ojs_submissions"} {
		if err := scopeAccess(app, name, nil, nil, nil, nil, nil); err != nil {
			return err
		}
	}
	chiefEditors, err := app.FindCollectionByNameOrId("chief_editors")
	if err != nil {
		return err
	}
	return app.Delete(chiefEditors)
}
