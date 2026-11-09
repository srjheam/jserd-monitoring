package state

import (
	"fmt"
	"server/internal/collections"
	"server/internal/collector"

	"github.com/pocketbase/pocketbase/core"
)

func saveCycle(app core.App, result collector.CycleResult, manifestJSON []byte, errors []string) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId(collections.CollectionCycles)
	if err != nil {
		return nil, fmt.Errorf("find collection cycles: %w", err)
	}
	record := core.NewRecord(collection)
	record.Set("source_mode", string(result.SourceMode))
	record.Set("status", result.Status)
	record.Set("started_at", result.StartedAt)
	record.Set("finished_at", result.FinishedAt)
	record.Set("manifest", manifestJSON)
	if len(errors) > 0 {
		record.Set("error", errors[0])
	}
	if err := app.Save(record); err != nil {
		return nil, fmt.Errorf("save collection cycle: %w", err)
	}
	return record, nil
}
