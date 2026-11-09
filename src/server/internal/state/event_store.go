package state

import (
	"encoding/json"
	"fmt"
	"server/internal/collector"
	"server/internal/diff"
	"server/internal/events"
	"server/internal/normalizer"
	"server/internal/snapshotpayload"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// storeEvents saves every event this cycle produced, with the rule matches on
// each one.
func (s *Store) storeEvents(app core.App, snapshotID string, result collector.CycleResult, previous []*core.Record) error {
	domainEvents, err := cycleEvents(app, result, previous)
	if err != nil {
		return err
	}
	for _, event := range domainEvents {
		if err := s.storeEvent(app, snapshotID, enrichEvent(event, result.Normalized), result.FinishedAt); err != nil {
			return err
		}
	}
	return nil
}

// cycleEvents are the events this cycle produced: the time-based ones, plus
// the ones the difference against the previous snapshot reveals.
func cycleEvents(app core.App, result collector.CycleResult, previous []*core.Record) ([]events.Event, error) {
	domainEvents := events.Temporal(result.Normalized, result.FinishedAt)
	if len(previous) == 0 {
		return append(domainEvents, events.Copyediting(normalizer.NormalizedCycle{}, result.Normalized, result.FinishedAt)...), nil
	}
	before, err := readSnapshotCycle(app, previous[0])
	if err != nil {
		return nil, err
	}
	domainEvents = append(domainEvents, events.Copyediting(before, result.Normalized, result.FinishedAt)...)
	return append(domainEvents, events.Detect(diff.Compare(before, result.Normalized))...), nil
}

// readSnapshotCycle decodes the normalized cycle one snapshot holds.
func readSnapshotCycle(app core.App, snapshot *core.Record) (normalizer.NormalizedCycle, error) {
	var cycle normalizer.NormalizedCycle
	payload, err := snapshotpayload.Read(app, snapshot)
	if err != nil {
		return cycle, fmt.Errorf("read previous snapshot: %w", err)
	}
	if err := json.Unmarshal(payload, &cycle); err != nil {
		return cycle, fmt.Errorf("decode previous snapshot: %w", err)
	}
	return cycle, nil
}

// storeEvent saves one event and every rule match on it.
func (s *Store) storeEvent(app core.App, snapshotID string, event events.Event, occurredAt time.Time) error {
	storedEvent, err := s.saveEvent(app, snapshotID, event, occurredAt)
	if err != nil {
		return err
	}
	for _, match := range events.MatchRules(event) {
		if err := s.saveMatch(app, storedEvent.Id, match); err != nil {
			return err
		}
	}
	return nil
}
