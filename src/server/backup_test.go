package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"server/internal/audit"
	"server/internal/collector"
	"server/internal/config"
	"server/internal/normalizer"
	"server/internal/state"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	pbarchive "github.com/pocketbase/pocketbase/tools/archive"
)

func TestBackupContainsDatabasesAndLatestSnapshotOnly(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	registerBackupPolicy(app)

	firstTime := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	oldPath := seedBackupSnapshot(t, app, firstTime, "old")
	latestPath := seedBackupSnapshot(t, app, firstTime.Add(time.Hour), "latest")
	writeBackupTestFile(t, filepath.Join(app.DataDir(), "collector_runs", "debug.json"), "debug")
	writeBackupTestFile(t, filepath.Join(app.DataDir(), "orphan.txt"), "orphan")

	const backupName = "minimal.zip"
	if err := app.CreateBackup(context.Background(), backupName); err != nil {
		t.Fatal(err)
	}
	assertMinimalBackup(t, app, backupName, latestPath, oldPath, "collector_runs/debug.json", "orphan.txt")
	for _, relative := range []string{oldPath, latestPath} {
		if _, err := os.Stat(filepath.Join(app.DataDir(), filepath.FromSlash(relative))); err != nil {
			t.Errorf("backup removed live snapshot %q: %v", relative, err)
		}
	}
}

func TestBackupWithoutSnapshotsContainsOnlyDatabases(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	registerBackupPolicy(app)
	writeBackupTestFile(t, filepath.Join(app.DataDir(), "storage", "orphan", "file"), "orphan")
	if err := app.CreateBackup(context.Background(), "first-instance.zip"); err != nil {
		t.Fatal(err)
	}
	assertMinimalBackup(t, app, "first-instance.zip", "")
}

func TestBackupRestoresLatestSnapshotForNextCycle(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	registerBackupPolicy(app)

	finished := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	seedBackupSnapshot(t, app, finished, "old")
	latestPath := seedBackupSnapshot(t, app, finished.Add(time.Hour), "latest")
	const backupName = "recovery.zip"
	if err := app.CreateBackup(context.Background(), backupName); err != nil {
		t.Fatal(err)
	}

	extractedDir := filepath.Join(t.TempDir(), "pb_data")
	if err := pbarchive.Extract(filepath.Join(app.DataDir(), core.LocalBackupsDirName, backupName), extractedDir); err != nil {
		t.Fatal(err)
	}
	restored, err := tests.NewTestApp(extractedDir)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Cleanup()
	if got, err := state.LatestSnapshotPayloadPath(restored); err != nil || got != latestPath {
		t.Fatalf("restored latest payload path = %q, err = %v, want %q", got, err, latestPath)
	}
	if err := state.NewStore(restored).ProcessCycle(backupCycle(finished.Add(2*time.Hour), "after restore")); err != nil {
		t.Fatalf("next cycle after restore: %v", err)
	}
}

func TestBackupFailsWhenLatestSnapshotPayloadIsMissing(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()
	registerBackupPolicy(app)
	if err := state.NewStore(app).ProcessCycle(backupCycle(time.Now().UTC(), "latest")); err != nil {
		t.Fatal(err)
	}
	payloadPath, err := state.LatestSnapshotPayloadPath(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(app.DataDir(), filepath.FromSlash(payloadPath))); err != nil {
		t.Fatal(err)
	}
	if err := app.CreateBackup(context.Background(), "missing-payload.zip"); err == nil {
		t.Fatal("backup succeeded without the latest snapshot payload")
	}
	if _, err := os.Stat(filepath.Join(app.DataDir(), core.LocalBackupsDirName, "missing-payload.zip")); !os.IsNotExist(err) {
		t.Fatalf("backup archive exists after failure: %v", err)
	}
}

func backupCycle(finished time.Time, title string) collector.CycleResult {
	started := finished.Add(-time.Minute)
	return collector.CycleResult{
		SourceMode: config.SourceModeFixtures,
		StartedAt:  started,
		FinishedAt: finished,
		Status:     collector.CycleStatusOK,
		Manifest:   audit.Manifest{SourceMode: string(config.SourceModeFixtures), CollectedAt: finished},
		Normalized: normalizer.NormalizedCycle{
			SourceMode:  string(config.SourceModeFixtures),
			CollectedAt: finished,
			Submissions: []normalizer.Submission{{OJSSubmissionID: "1", Title: title}},
		},
	}
}

func seedBackupSnapshot(t *testing.T, app core.App, finished time.Time, title string) string {
	t.Helper()
	if err := state.NewStore(app).ProcessCycle(backupCycle(finished, title)); err != nil {
		t.Fatal(err)
	}
	path, err := state.LatestSnapshotPayloadPath(app)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeBackupTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertMinimalBackup(t *testing.T, app core.App, archive, latest string, excluded ...string) {
	t.Helper()
	files := backupFiles(t, app, archive)
	if !files["data.db"] || !files["auxiliary.db"] || (latest != "" && !files[latest]) {
		t.Fatalf("backup is missing a database or latest snapshot: %v", files)
	}
	for _, name := range excluded {
		if files[name] {
			t.Errorf("backup contains excluded data %q", name)
		}
	}
	for name := range files {
		if !isDatabaseFile(name) && name != latest {
			t.Errorf("unexpected backup file %q", name)
		}
	}
}

func backupFiles(t *testing.T, app core.App, name string) map[string]bool {
	t.Helper()
	archive, err := zip.OpenReader(filepath.Join(app.DataDir(), core.LocalBackupsDirName, name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	files := make(map[string]bool, len(archive.File))
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, reader); err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		files[file.Name] = true
	}
	return files
}
