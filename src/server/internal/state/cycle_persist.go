package state

import (
	"encoding/json"
	"fmt"
	"server/internal/collector"

	"github.com/pocketbase/pocketbase/core"
)

// recordFailedCycle stores a cycle that failed. A failed cycle carries no
// snapshot, so it produces no event.
func (s *Store) recordFailedCycle(result collector.CycleResult, manifestJSON []byte, failures []string) error {
	record, err := saveCycle(s.App, result, manifestJSON, failures)
	if err != nil {
		return err
	}
	s.App.Logger().Info("collection cycle persisted", "cycleId", record.Id, "status", result.Status, "snapshotCount", 0)
	return nil
}

// recordSucceededCycle stores a cycle, its snapshot, and every event the
// snapshot implies, in one transaction.
func (s *Store) recordSucceededCycle(result collector.CycleResult, manifestJSON []byte, failures []string) error {
	payload, err := json.Marshal(canonicalCycle(result.Normalized))
	if err != nil {
		return fmt.Errorf("marshal snapshot payload: %w", err)
	}
	var cycleID, snapshotID string
	err = s.App.RunInTransaction(func(txApp core.App) error {
		var txErr error
		cycleID, snapshotID, txErr = s.storeCycle(txApp, result, manifestJSON, failures, payload)
		return txErr
	})
	if err != nil {
		return fmt.Errorf("persist successful cycle: %w", err)
	}
	s.App.Logger().Info("collection cycle persisted", "cycleId", cycleID, "snapshotId", snapshotID, "status", result.Status, "snapshotCount", 1)
	return nil
}

// storeCycle writes the cycle row, the snapshot and the events of one
// succeeded cycle.
func (s *Store) storeCycle(app core.App, result collector.CycleResult, manifestJSON []byte, failures []string, payload []byte) (string, string, error) {
	previous, cycle, err := s.startCycle(app, result, manifestJSON, failures)
	if err != nil {
		return "", "", err
	}
	snapshot, err := saveSnapshot(app, cycle.Id, payload, previous)
	if err != nil {
		return "", "", err
	}
	return cycle.Id, snapshot.Id, s.storeEvents(app, snapshot.Id, result, previous)
}

// startCycle syncs the recipients, finds the snapshot this cycle follows, and
// writes the cycle row.
func (s *Store) startCycle(app core.App, result collector.CycleResult, manifestJSON []byte, failures []string) ([]*core.Record, *core.Record, error) {
	if err := syncRecipients(app, result.Normalized); err != nil {
		return nil, nil, err
	}
	previous, err := findPreviousSnapshot(app)
	if err != nil {
		return nil, nil, fmt.Errorf("find previous snapshot: %w", err)
	}
	cycle, err := saveCycle(app, result, manifestJSON, failures)
	if err != nil {
		return nil, nil, err
	}
	return previous, cycle, nil
}
