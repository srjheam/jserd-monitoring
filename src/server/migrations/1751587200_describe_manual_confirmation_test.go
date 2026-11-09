package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDescribeManualConfirmationMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	confirmed, ok := requests.Fields.GetByName("confirmed").(*core.BoolField)
	if !ok || confirmed.Help != manualConfirmationHelp {
		t.Fatalf("confirmed help = %q, want %q", confirmed.Help, manualConfirmationHelp)
	}
}
