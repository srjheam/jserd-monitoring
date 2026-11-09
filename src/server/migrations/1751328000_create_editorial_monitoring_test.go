package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestEditorialMigrationSchema(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	// editorial_runs and editorial_items, created by this migration, are
	// dropped by a later one (1788000000_drop_editorial_runs_and_items.go);
	// they are covered by that migration's own test instead. chief_editors,
	// created by this migration, is renamed to editorial_recipients by a
	// later one (1788900000_rename_chief_editors_to_editorial_recipients.go)
	// to free the name for the chief_editors auth collection.
	for _, name := range []string{"editorial_recipients", "editorial_delivery_attempts", "manual_collection_requests"} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if collection.ListRule != nil || collection.ViewRule != nil || collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
			t.Fatalf("%s must be superuser-only", name)
		}
	}
	attempts, _ := app.FindCollectionByNameOrId("editorial_delivery_attempts")
	if !hasUnique(attempts, fieldCycle) {
		t.Error("editorial_delivery_attempts unique cycle missing")
	}
	requests, _ := app.FindCollectionByNameOrId("manual_collection_requests")
	for _, field := range []string{fieldStatus, "confirmed", fieldOperator, "note", fieldCycle, fieldSanitizedError} {
		if requests.Fields.GetByName(field) == nil {
			t.Errorf("request field %s missing", field)
		}
	}
}
