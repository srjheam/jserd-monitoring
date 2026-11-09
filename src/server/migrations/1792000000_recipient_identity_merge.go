package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(mergeRecipientIdentity, restoreRecipientKindKey)
}

const (
	recipientKindKeyIndex  = "CREATE UNIQUE INDEX `idx_recipients_kind_key` ON `recipients` (`kind`, `key`)"
	recipientIdentityIndex = "CREATE UNIQUE INDEX `idx_recipients_identity` ON `recipients` (`identity`)"
)

// mergeRecipientIdentity replaces the recipients kind+key composite with one
// identity per real, e-mailable person (ADR 0015): a reviewer's OJS
// username, or an assigned editor's name-slug. An assigned editor and a
// reviewer sharing a name-slug become one row; rows with no valid e-mail are
// dropped as sync noise.
func mergeRecipientIdentity(app core.App) error {
	if err := addRecipientIdentityFields(app); err != nil {
		return err
	}
	if err := mergeAllRecipients(app); err != nil {
		return err
	}
	return dropRecipientKindKeyFields(app)
}

func addRecipientIdentityFields(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		return err
	}
	collection.Fields.Add(
		&core.TextField{Name: "identity", Required: true},
		&core.TextField{Name: "name_slug", Required: true},
		&core.BoolField{Name: "is_editor"},
		&core.BoolField{Name: "is_reviewer"},
	)
	return app.Save(collection)
}

func dropRecipientKindKeyFields(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		return err
	}
	collection.Fields.RemoveByName("kind")
	collection.Fields.RemoveByName("key")
	for i, index := range collection.Indexes {
		if index == recipientKindKeyIndex {
			collection.Indexes = append(collection.Indexes[:i], collection.Indexes[i+1:]...)
			break
		}
	}
	collection.Indexes = append(collection.Indexes, recipientIdentityIndex)
	return app.Save(collection)
}

// restoreRecipientKindKey re-adds kind/key but not their data: like the
// merge itself, this rollback is lossy.
func restoreRecipientKindKey(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		return err
	}
	collection.Fields.RemoveByName("identity")
	collection.Fields.RemoveByName("name_slug")
	collection.Fields.RemoveByName("is_editor")
	collection.Fields.RemoveByName("is_reviewer")
	collection.Fields.Add(
		&core.TextField{Name: "kind", Required: true},
		&core.TextField{Name: "key", Required: true},
	)
	for i, index := range collection.Indexes {
		if index == recipientIdentityIndex {
			collection.Indexes = append(collection.Indexes[:i], collection.Indexes[i+1:]...)
			break
		}
	}
	collection.Indexes = append(collection.Indexes, recipientKindKeyIndex)
	return app.Save(collection)
}
