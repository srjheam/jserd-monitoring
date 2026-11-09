package main

import (
	"server/internal/audit"
	"server/internal/state"
)

// retentionKeepCycles is how many successful cycles' snapshot payloads and
// collector_runs artifacts survive on disk, by count rather than by date, so
// a period of downtime does not shrink the retained window. 240 is 10 days
// at the default hourly cron.
const retentionKeepCycles = 240

// newPruner deletes the snapshot payloads and the collector artifacts that
// retention no longer keeps.
func newPruner(store *state.Store, artifactStore audit.ArtifactStore) func() error {
	return func() error {
		if err := store.PruneSnapshotPayloads(retentionKeepCycles); err != nil {
			return err
		}
		return artifactStore.PruneRuns(retentionKeepCycles)
	}
}
