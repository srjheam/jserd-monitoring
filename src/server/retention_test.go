package main

import (
	"server/internal/audit"
	"server/internal/state"
	"testing"
)

func TestNewPrunerPrunesSnapshotsAndArtifacts(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()

	prune := newPruner(state.NewStore(app), audit.ArtifactStore{Root: t.TempDir()})
	if err := prune(); err != nil {
		t.Fatal(err)
	}
}

func TestNewPrunerReportsSnapshotPruneFailureWithoutPruningArtifacts(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()

	// Drop the snapshots table so the prune query fails cleanly, without
	// tearing down the app the way app.Cleanup() does.
	if _, err := app.DB().NewQuery("DROP TABLE snapshots").Execute(); err != nil {
		t.Fatal(err)
	}

	prune := newPruner(state.NewStore(app), audit.ArtifactStore{Root: t.TempDir()})
	if err := prune(); err == nil {
		t.Fatal("expected the pruner to surface the snapshot query failure")
	}
}
