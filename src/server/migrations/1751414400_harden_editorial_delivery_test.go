package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestHardenEditorialDeliveryMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	attempts, err := app.FindCollectionByNameOrId("editorial_delivery_attempts")
	if err != nil {
		t.Fatal(err)
	}
	status, isSelectField := attempts.Fields.GetByName(fieldStatus).(*core.SelectField)
	if !isSelectField {
		t.Fatal("status must be a select field")
	}
	if attempts.Fields.GetByName("lease_owner") == nil || attempts.Fields.GetByName("lease_expires_at") == nil || len(status.Values) != 4 || status.Values[0] != "sending" {
		t.Fatalf("migration not applied: %#v", status.Values)
	}
}
