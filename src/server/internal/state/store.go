package state

import (
	"encoding/json"
	"fmt"
	"server/internal/collector"
	"sync"

	"github.com/pocketbase/pocketbase/core"
)

type Store struct {
	App        core.App
	mu         sync.Mutex
	saveRecord func(core.App, core.Model) error
}

func NewStore(app core.App) *Store {
	return &Store{App: app, saveRecord: func(app core.App, model core.Model) error { return app.Save(model) }}
}

func (s *Store) ProcessCycle(result collector.CycleResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCycleStatus(result.Status); err != nil {
		return err
	}
	manifest := redactManifest(result.Manifest)
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal cycle manifest: %w", err)
	}
	if result.Status == collector.CycleStatusFailed {
		return s.recordFailedCycle(result, manifestJSON, manifest.Errors)
	}
	return s.recordSucceededCycle(result, manifestJSON, manifest.Errors)
}

// validateCycleStatus rejects a cycle status this store cannot record.
func validateCycleStatus(status string) error {
	if status != collector.CycleStatusOK && status != collector.CycleStatusFailed {
		return fmt.Errorf("unsupported cycle status %q", status)
	}
	return nil
}
