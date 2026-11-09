package state

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"server/internal/collections"
	"server/internal/snapshotpayload"

	"github.com/pocketbase/pocketbase/core"
)

const snapshotSchemaVersion = "snapshot:v1"

// previousSnapshotID is the snapshot this one follows, or an empty string for
// the first snapshot there is.
func previousSnapshotID(previous []*core.Record) string {
	if len(previous) == 0 {
		return ""
	}
	return previous[0].Id
}

// saveSnapshot writes the snapshot of one cycle, linked to the snapshot before
// it.
func saveSnapshot(app core.App, cycleID string, payload []byte, previous []*core.Record) (*core.Record, error) {
	snapshots, err := app.FindCollectionByNameOrId(collections.Snapshots)
	if err != nil {
		return nil, fmt.Errorf("find snapshots collection: %w", err)
	}
	payloadFile, err := snapshotpayload.NewFile(payload)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(payload)
	snapshot := core.NewRecord(snapshots)
	snapshot.Set("cycle", cycleID)
	snapshot.Set("schema_version", snapshotSchemaVersion)
	snapshot.Set("payload", payloadFile)
	snapshot.Set("sha256", hex.EncodeToString(hash[:]))
	snapshot.Set("previous_snapshot", previousSnapshotID(previous))
	if err := app.Save(snapshot); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}
	return snapshot, nil
}

func findPreviousSnapshot(app core.App) ([]*core.Record, error) {
	// Base collections have no created field; cycle.finished_at plus immutable id
	// provides a deterministic newest-first order without rewriting history.
	var snapshotID string
	err := app.DB().NewQuery(`
		SELECT snapshots.id
		FROM snapshots
		LEFT JOIN collection_cycles AS cycle ON cycle.id = snapshots.cycle
		ORDER BY cycle.finished_at DESC, snapshots.id DESC
		LIMIT 1
	`).Row(&snapshotID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record, err := app.FindRecordById(collections.Snapshots, snapshotID)
	if err != nil {
		return nil, err
	}
	return []*core.Record{record}, nil
}
