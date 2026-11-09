package state

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"server/internal/events"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func (s *Store) saveEvent(app core.App, snapshotID string, event events.Event, occurredAt time.Time) (*core.Record, error) {
	existing, err := findEvent(app, event.IdempotencyKey)
	if err != nil || existing != nil {
		return existing, err
	}
	return s.insertEvent(app, snapshotID, event, occurredAt)
}

// insertEvent writes one new event row. A unique-key conflict means that
// another writer stored the same event first, and that row wins.
func (s *Store) insertEvent(app core.App, snapshotID string, event events.Event, occurredAt time.Time) (*core.Record, error) {
	if err := validateReviewerName(event); err != nil {
		return nil, err
	}
	record, err := newEventRecord(app, snapshotID, event, occurredAt)
	if err != nil {
		return nil, err
	}
	if err := s.saveRecord(app, record); err != nil {
		return s.resolveEventConflict(app, event.IdempotencyKey, err)
	}
	return record, nil
}

// validateReviewerName rejects a reviewer event that names no reviewer, because
// the e-mail it leads to addresses that reviewer by name.
func validateReviewerName(event events.Event) error {
	if requiresReviewerName(event.Type) && strings.TrimSpace(event.ReviewerName) == "" {
		return fmt.Errorf("event %q is missing reviewer name", event.Type)
	}
	return nil
}

// newEventRecord builds one event row.
func newEventRecord(app core.App, snapshotID string, event events.Event, occurredAt time.Time) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId(collections.Events)
	if err != nil {
		return nil, fmt.Errorf("find events collection: %w", err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	record := core.NewRecord(collection)
	record.Set("snapshot", snapshotID)
	record.Set("type", event.Type)
	record.Set("entity_type", event.EntityType)
	record.Set("entity_key", event.EntityKey)
	record.Set("transition", event.Transition)
	record.Set("occurrence_key", event.OccurrenceKey)
	record.Set("idempotency_key", event.IdempotencyKey)
	record.Set("payload", payload)
	record.Set("occurred_at", occurredAt)
	return record, nil
}
