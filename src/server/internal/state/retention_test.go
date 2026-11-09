package state

import (
	"context"
	"server/internal/collections"
	"server/internal/collector"
	"testing"
	"time"

	_ "server/migrations"
)

func cycleAt(base time.Time, offset time.Duration) collector.CycleResult {
	c := successfulCycle("", "")
	c.StartedAt = base.Add(offset)
	c.FinishedAt = base.Add(offset).Add(time.Minute)
	c.Manifest.CollectedAt = c.StartedAt
	c.Normalized.CollectedAt = c.StartedAt
	return c
}

func TestPruneSnapshotPayloadsKeepsNewestClearsOlder(t *testing.T) {
	app := newOperationalTestApp(t)
	RegisterImmutableHooks(app)
	store := NewStore(app)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		if err := store.ProcessCycle(cycleAt(base, time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("process cycle %d: %v", i, err)
		}
	}
	allIDs, err := snapshotsBeyondRetention(app, 0)
	if err != nil || len(allIDs) != 5 {
		t.Fatalf("allIDs=%v err=%v", allIDs, err)
	}

	if err := store.PruneSnapshotPayloads(2); err != nil {
		t.Fatalf("prune: %v", err)
	}

	for rank, id := range allIDs {
		record, err := app.FindRecordById(collections.Snapshots, id)
		if err != nil {
			t.Fatalf("find snapshot %s: %v", id, err)
		}
		empty := record.GetString("payload") == ""
		wantEmpty := rank >= 2
		if empty != wantEmpty {
			t.Fatalf("snapshot rank %d payload empty=%v, want %v", rank, empty, wantEmpty)
		}
	}
}

func TestPruneSnapshotPayloadsNeverDeletesRows(t *testing.T) {
	app := newOperationalTestApp(t)
	RegisterImmutableHooks(app)
	store := NewStore(app)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := range 3 {
		if err := store.ProcessCycle(cycleAt(base, time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("process cycle %d: %v", i, err)
		}
	}
	if err := store.PruneSnapshotPayloads(1); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if countRecords(t, app, collections.CollectionCycles) != 3 {
		t.Fatalf("collection_cycles rows must never be deleted by pruning")
	}
	if countRecords(t, app, collections.Snapshots) != 3 {
		t.Fatalf("snapshots rows must never be deleted by pruning")
	}
}

func TestPruneSnapshotPayloadsIgnoresFailedCycles(t *testing.T) {
	app := newOperationalTestApp(t)
	RegisterImmutableHooks(app)
	store := NewStore(app)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := store.ProcessCycle(cycleAt(base, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessCycle(failedCycle("boom")); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessCycle(cycleAt(base, time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := store.PruneSnapshotPayloads(1); err != nil {
		t.Fatalf("prune with a failed cycle present: %v", err)
	}
	if countRecords(t, app, collections.CollectionCycles) != 3 {
		t.Fatalf("collection_cycles = %d, want 3 (failed cycle rows are never touched)", countRecords(t, app, collections.CollectionCycles))
	}
}

func TestSnapshotPayloadUpdateRequiresRetentionContext(t *testing.T) {
	app := newOperationalTestApp(t)
	RegisterImmutableHooks(app)
	store := NewStore(app)
	if err := store.ProcessCycle(successfulCycle("", "")); err != nil {
		t.Fatal(err)
	}
	records, err := app.FindAllRecords(collections.Snapshots)
	if err != nil || len(records) != 1 {
		t.Fatalf("snapshots=%d err=%v", len(records), err)
	}
	record := records[0]

	record.Set("payload", "")
	if err := app.Save(record); err == nil {
		t.Fatal("update without retention context unexpectedly allowed")
	}

	if err := app.SaveNoValidateWithContext(withRetentionUpdate(context.Background()), record); err != nil {
		t.Fatalf("retention-authorized update rejected: %v", err)
	}
}
