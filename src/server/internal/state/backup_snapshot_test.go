package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestLatestSnapshotPayloadPathIsEmptyWithoutSnapshots(t *testing.T) {
	app := newOperationalTestApp(t)
	got, err := LatestSnapshotPayloadPath(app)
	if err != nil || got != "" {
		t.Fatalf("LatestSnapshotPayloadPath() = %q, %v, want empty path without error", got, err)
	}
}

func TestLatestSnapshotPayloadPathMatchesPreviousSnapshotSelection(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	first := successfulCycle("", "")
	if err := store.ProcessCycle(first); err != nil {
		t.Fatal(err)
	}
	latest := successfulCycle("", "")
	latest.StartedAt = latest.StartedAt.Add(time.Hour)
	latest.FinishedAt = latest.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(latest); err != nil {
		t.Fatal(err)
	}
	tied := successfulCycle("", "")
	tied.StartedAt = latest.StartedAt
	tied.FinishedAt = latest.FinishedAt
	if err := store.ProcessCycle(tied); err != nil {
		t.Fatal(err)
	}

	previous, err := findPreviousSnapshot(app)
	if err != nil || len(previous) != 1 {
		t.Fatalf("findPreviousSnapshot() = %d records, %v", len(previous), err)
	}
	want := filepath.ToSlash(filepath.Join("storage", previous[0].BaseFilesPath(), previous[0].GetString("payload")))
	got, err := LatestSnapshotPayloadPath(app)
	if err != nil || got != want {
		t.Fatalf("LatestSnapshotPayloadPath() = %q, %v, want %q", got, err, want)
	}
}

func TestLatestSnapshotPayloadPathRejectsBlankFilename(t *testing.T) {
	app := newOperationalTestApp(t)
	record := seedSnapshotForBackupPathTest(t, app)
	if _, err := app.DB().NewQuery("UPDATE snapshots SET payload = '' WHERE id = {:id}").Bind(map[string]any{"id": record.Id}).Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := LatestSnapshotPayloadPath(app); err == nil {
		t.Fatal("LatestSnapshotPayloadPath() succeeded with a blank filename")
	}
}

func TestLatestSnapshotPayloadPathRejectsMissingFile(t *testing.T) {
	app := newOperationalTestApp(t)
	seedSnapshotForBackupPathTest(t, app)
	path, err := LatestSnapshotPayloadPath(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(app.DataDir(), filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
	if _, err := LatestSnapshotPayloadPath(app); err == nil {
		t.Fatal("LatestSnapshotPayloadPath() succeeded with a missing file")
	}
}

func TestLatestSnapshotPayloadPathRejectsSymlink(t *testing.T) {
	app := newOperationalTestApp(t)
	seedSnapshotForBackupPathTest(t, app)
	path, err := LatestSnapshotPayloadPath(app)
	if err != nil {
		t.Fatal(err)
	}
	fullPath := filepath.Join(app.DataDir(), filepath.FromSlash(path))
	if err := os.Remove(fullPath); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, fullPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LatestSnapshotPayloadPath(app); err == nil {
		t.Fatal("LatestSnapshotPayloadPath() succeeded with a symbolic link")
	}
}

func seedSnapshotForBackupPathTest(t *testing.T, app *tests.TestApp) *core.Record {
	t.Helper()
	if err := NewStore(app).ProcessCycle(successfulCycle("", "")); err != nil {
		t.Fatal(err)
	}
	previous, err := findPreviousSnapshot(app)
	if err != nil || len(previous) != 1 {
		t.Fatalf("findPreviousSnapshot() = %d records, %v", len(previous), err)
	}
	return previous[0]
}
