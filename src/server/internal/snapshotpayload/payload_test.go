package snapshotpayload_test

import (
	"server/internal/collections"
	"server/internal/snapshotpayload"
	"server/internal/testseed"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "server/migrations"
)

func TestNewFileCreatesAJSONNamedFileOfTheRightSize(t *testing.T) {
	body := []byte(`{"sourceMode":"fixtures"}`)
	file, err := snapshotpayload.NewFile(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(file.Name, "snapshot") || !strings.HasSuffix(file.Name, ".json") {
		t.Fatalf("name=%q, want it to look like a snapshot json file", file.Name)
	}
	if file.Size != int64(len(body)) {
		t.Fatalf("size=%d, want %d", file.Size, len(body))
	}
}

func TestNewFileRejectsAnEmptyPayload(t *testing.T) {
	if _, err := snapshotpayload.NewFile(nil); err == nil {
		t.Fatal("expected an error for an empty payload")
	}
}

func TestReadRoundTripsTheStoredPayload(t *testing.T) {
	app := snapshotTestApp(t)
	body := []byte(`{"sourceMode":"fixtures","submissions":[]}`)
	record := saveSnapshotWithPayload(t, app, body)

	got, err := snapshotpayload.Read(app, record)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("read=%q, want %q", got, body)
	}
}

func TestReadRejectsASnapshotWithNoPayloadFile(t *testing.T) {
	app := snapshotTestApp(t)
	cycles, _ := app.FindCollectionByNameOrId(collections.CollectionCycles)
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", "fixtures")
	cycle.Set("status", "succeeded")
	cycle.Set("started_at", time.Now())
	cycle.Set("finished_at", time.Now())
	cycle.Set("manifest", map[string]any{"source_mode": "fixtures"})
	if err := app.Save(cycle); err != nil {
		t.Fatal(err)
	}
	snapshots, _ := app.FindCollectionByNameOrId(collections.Snapshots)
	snapshot := core.NewRecord(snapshots)
	snapshot.Set("cycle", cycle.Id)
	snapshot.Set("schema_version", "snapshot:v1")
	snapshot.Set("sha256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	if _, err := snapshotpayload.Read(app, snapshot); err == nil {
		t.Fatal("expected an error for a snapshot with no payload file")
	}
}

func snapshotTestApp(t *testing.T) *tests.TestApp {
	t.Helper()
	dir, err := testseed.Dir()
	if err != nil {
		t.Fatal(err)
	}
	app, err := tests.NewTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func saveSnapshotWithPayload(t *testing.T, app core.App, body []byte) *core.Record {
	t.Helper()
	cycles, _ := app.FindCollectionByNameOrId(collections.CollectionCycles)
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", "fixtures")
	cycle.Set("status", "succeeded")
	cycle.Set("started_at", time.Now())
	cycle.Set("finished_at", time.Now())
	cycle.Set("manifest", map[string]any{"source_mode": "fixtures"})
	if err := app.Save(cycle); err != nil {
		t.Fatal(err)
	}
	file, err := snapshotpayload.NewFile(body)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, _ := app.FindCollectionByNameOrId(collections.Snapshots)
	snapshot := core.NewRecord(snapshots)
	snapshot.Set("cycle", cycle.Id)
	snapshot.Set("schema_version", "snapshot:v1")
	snapshot.Set("payload", file)
	snapshot.Set("sha256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := app.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
