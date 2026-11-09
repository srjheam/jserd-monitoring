package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestExpandSnapshotPayloadMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	snapshots, err := app.FindCollectionByNameOrId("snapshots")
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := snapshots.Fields.GetByName(fieldPayload).(*core.FileField)
	if !ok || payload.MaxSize != snapshotPayloadMaxSize {
		t.Fatalf("payload max size = %d, want %d", payload.MaxSize, snapshotPayloadMaxSize)
	}
}
