package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

// The name chief_editors is needed for the new auth collection added right
// after this migration: a chief editor's dashboard sign-in
// account, distinct from this manual e-mail-recipient fallback roster. This
// migration frees the name by renaming the roster to editorial_recipients;
// its rows, fields, and every reader (internal/editorial/recipients.go) move
// with it.
func init() {
	migrations.Register(renameChiefEditorsToEditorialRecipients, renameEditorialRecipientsToChiefEditors)
}

func renameChiefEditorsToEditorialRecipients(app core.App) error {
	return renameCollection(app, "chief_editors", "editorial_recipients")
}

func renameEditorialRecipientsToChiefEditors(app core.App) error {
	return renameCollection(app, "editorial_recipients", "chief_editors")
}

func renameCollection(app core.App, from, to string) error {
	collection, err := app.FindCollectionByNameOrId(from)
	if err != nil {
		return err
	}
	collection.Name = to
	return app.Save(collection)
}
