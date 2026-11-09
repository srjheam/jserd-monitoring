package editorial

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"server/internal/normalizer"
	"server/internal/snapshotpayload"

	"github.com/pocketbase/pocketbase/core"
)

// This file reads and decodes the latest collection snapshot, the input
// evaluateLatest (findings_publish.go) evaluates.

func (c *Coordinator) latestSnapshot() (normalizer.NormalizedCycle, string, string, error) {
	records, err := c.App.FindRecordsByFilter(collections.Snapshots, "", "-cycle.finished_at,-id", 1, 0)
	if err != nil || len(records) == 0 {
		return normalizer.NormalizedCycle{}, "", "", fmt.Errorf("load current snapshot: %w", err)
	}
	cycle, err := decodeSnapshot(c.App, records[0])
	if err != nil {
		return cycle, "", "", err
	}
	return cycle, records[0].GetString("cycle"), records[0].Id, nil
}

// decodeSnapshot reads the normalized cycle one snapshot record holds.
func decodeSnapshot(app core.App, record *core.Record) (normalizer.NormalizedCycle, error) {
	var cycle normalizer.NormalizedCycle
	payload, err := snapshotpayload.Read(app, record)
	if err != nil {
		return cycle, fmt.Errorf("read current snapshot: %w", err)
	}
	if err := json.Unmarshal(payload, &cycle); err != nil {
		return cycle, fmt.Errorf("decode current snapshot: %w", err)
	}
	return cycle, nil
}
