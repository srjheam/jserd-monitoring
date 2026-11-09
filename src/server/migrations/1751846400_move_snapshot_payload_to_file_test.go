package migrations

import (
	"bytes"
	"server/internal/snapshotpayload"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestMoveSnapshotPayloadToFileMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := restoreSnapshotPayloadJSON(app); err != nil {
		t.Fatal(err)
	}
	record, want := seedSnapshotWithPayload(t, app)
	if err := moveSnapshotPayloadToFile(app); err != nil {
		t.Fatal(err)
	}

	t.Run("payload field becomes a protected file field", func(t *testing.T) {
		assertSnapshotPayloadIsFileField(t, app)
	})
	t.Run("payload content is preserved on disk", func(t *testing.T) {
		assertMigratedSnapshotPayload(t, app, record, want)
	})
	t.Run("database no longer stores the raw payload", func(t *testing.T) {
		assertSnapshotPayloadNotInDatabase(t, app, record, want)
	})
}

func assertSnapshotPayloadIsFileField(t *testing.T, app core.App) {
	t.Helper()
	snapshots, err := app.FindCollectionByNameOrId("snapshots")
	if err != nil {
		t.Fatal(err)
	}
	field, ok := snapshots.Fields.GetByName(fieldPayload).(*core.FileField)
	if !ok || !field.Protected || !field.Required || field.MaxSize != snapshotpayload.MaxSize {
		t.Fatalf("payload field = %#v", field)
	}
}

func assertMigratedSnapshotPayload(t *testing.T, app core.App, record *core.Record, want []byte) {
	t.Helper()
	migrated, err := app.FindRecordById("snapshots", record.Id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := snapshotpayload.Read(app, migrated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload = %s, want %s", got, want)
	}
}

func assertSnapshotPayloadNotInDatabase(t *testing.T, app core.App, record *core.Record, want []byte) {
	t.Helper()
	migrated, err := app.FindRecordById("snapshots", record.Id)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := app.DB().NewQuery("SELECT payload FROM snapshots WHERE id = {:id}").Bind(map[string]any{"id": record.Id}).Row(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != migrated.GetString(fieldPayload) || bytes.Contains([]byte(stored), want) {
		t.Fatalf("database payload = %q, want file name %q", stored, migrated.GetString(fieldPayload))
	}
}

// seedSnapshotWithPayload creates a cycle and a snapshot record that still
// stores its payload inline, and returns the record and the payload bytes.
func seedSnapshotWithPayload(t *testing.T, app core.App) (*core.Record, []byte) {
	t.Helper()
	cycles, err := app.FindCollectionByNameOrId("collection_cycles")
	if err != nil {
		t.Fatal(err)
	}
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", sourceModeFixtures)
	cycle.Set(fieldStatus, "succeeded")
	cycle.Set("started_at", time.Now())
	cycle.Set("finished_at", time.Now())
	cycle.Set("manifest", map[string]any{"source_mode": sourceModeFixtures})
	if err := app.Save(cycle); err != nil {
		t.Fatal(err)
	}
	snapshots, err := app.FindCollectionByNameOrId("snapshots")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"sourceMode":"fixtures"}`)
	record := core.NewRecord(snapshots)
	record.Set(fieldCycle, cycle.Id)
	record.Set("schema_version", "snapshot:v1")
	record.Set(fieldPayload, want)
	record.Set("sha256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	return record, want
}
