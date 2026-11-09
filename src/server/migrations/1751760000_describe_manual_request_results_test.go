package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDescribeManualRequestResultsMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	// "run" is dropped by 1788000000_drop_editorial_runs_and_items.go, so it
	// no longer exists to check the help text of at HEAD.
	for name, want := range manualRequestResultHelp {
		if name == fieldRun {
			continue
		}
		field := requests.Fields.GetByName(name)
		var got string
		switch typed := field.(type) {
		case *core.RelationField:
			got = typed.Help
		case *core.TextField:
			got = typed.Help
		default:
			t.Fatalf("field %q has type %T", name, field)
		}
		if got != want {
			t.Errorf("field %q help = %q, want %q", name, got, want)
		}
	}
}
