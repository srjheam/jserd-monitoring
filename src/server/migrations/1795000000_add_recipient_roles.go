package migrations

import (
	"server/internal/collections"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addRecipientRoles, removeRecipientRoles)
}

func addRecipientRoles(app core.App) error {
	for _, name := range []string{collections.NotificationTargets, collections.EmailDeliveryAttempts} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return err
		}
		collection.Fields.Add(&core.TextField{Name: "recipient_role"})
		if err := app.Save(collection); err != nil {
			return err
		}
	}
	return nil
}

func removeRecipientRoles(app core.App) error {
	for _, name := range []string{collections.NotificationTargets, collections.EmailDeliveryAttempts} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return err
		}
		collection.Fields.RemoveByName("recipient_role")
		if err := app.Save(collection); err != nil {
			return err
		}
	}
	return nil
}
