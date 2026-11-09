package state

import (
	"context"
	"fmt"
	"server/internal/collections"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// PruneSnapshotPayloads clears the payload file of every snapshot older
// than the newest `keep` snapshots, freeing disk. The snapshots and
// collection_cycles rows are never deleted: events.snapshot is a required
// relation, so deleting a referenced snapshot is blocked by design, and a
// cycle's own row is cheap to keep forever. Cycles with no payload (failed
// cycles) are not counted against `keep` and are left untouched.
func (s *Store) PruneSnapshotPayloads(keep int) error {
	ids, err := snapshotsBeyondRetention(s.App, keep)
	if err != nil {
		return fmt.Errorf("find snapshots beyond retention: %w", err)
	}
	for _, id := range ids {
		if err := clearSnapshotPayload(s.App, id); err != nil {
			return fmt.Errorf("clear snapshot %s payload: %w", id, err)
		}
	}
	return nil
}

func snapshotsBeyondRetention(app core.App, keep int) ([]string, error) {
	var ids []string
	err := app.DB().NewQuery(`
		SELECT snapshots.id
		FROM snapshots
		INNER JOIN collection_cycles AS cycle ON cycle.id = snapshots.cycle
		WHERE snapshots.payload != ''
		ORDER BY cycle.finished_at DESC, snapshots.id DESC
		LIMIT -1 OFFSET {:keep}
	`).Bind(dbx.Params{"keep": keep}).Column(&ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func clearSnapshotPayload(app core.App, snapshotID string) error {
	record, err := app.FindRecordById(collections.Snapshots, snapshotID)
	if err != nil {
		return fmt.Errorf("find snapshot: %w", err)
	}
	record.Set("payload", "")
	return app.SaveNoValidateWithContext(withRetentionUpdate(context.Background()), record)
}
