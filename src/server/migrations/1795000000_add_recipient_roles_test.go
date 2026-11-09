package migrations

import (
	"server/internal/collections"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecipientRoleMigrationAddsOptionalHistoryFields(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	for _, name := range []string{collections.NotificationTargets, collections.EmailDeliveryAttempts} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatal(err)
		}
		field, ok := collection.Fields.GetByName("recipient_role").(*core.TextField)
		if !ok || field.Required {
			t.Fatalf("%s.recipient_role=%#v, want optional text field", name, field)
		}
	}

	if err := removeRecipientRoles(app); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{collections.NotificationTargets, collections.EmailDeliveryAttempts} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatal(err)
		}
		if collection.Fields.GetByName("recipient_role") != nil {
			t.Fatalf("%s.recipient_role remains after rollback", name)
		}
	}
}
