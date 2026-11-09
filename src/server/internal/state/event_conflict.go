package state

import (
	"fmt"
	"server/internal/collections"
	"server/internal/dberr"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// resolveEventConflict reads back the event another writer stored first. A
// failure that is not a unique-key conflict is reported as it is.
func (s *Store) resolveEventConflict(app core.App, idempotencyKey string, saveErr error) (*core.Record, error) {
	if !dberr.IsConstraint(saveErr) {
		return nil, fmt.Errorf("save event: %w", saveErr)
	}
	existing, err := findEvent(app, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("save event constraint without matching idempotency key: %w", saveErr)
	}
	return existing, nil
}

func findEvent(app core.App, key string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter(collections.Events, "idempotency_key = {:key}", "", 1, 0, dbx.Params{"key": key})
	if err != nil {
		return nil, fmt.Errorf("find event by idempotency key: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}
