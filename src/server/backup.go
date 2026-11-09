package main

import (
	"fmt"
	"os"
	"path/filepath"
	"server/internal/state"

	"github.com/pocketbase/pocketbase/core"
)

func registerBackupPolicy(app core.App) {
	app.OnBackupCreate().BindFunc(func(event *core.BackupEvent) error {
		return event.App.RunInTransaction(func(txApp core.App) error {
			payloadPath, err := state.LatestSnapshotPayloadPath(txApp)
			if err != nil {
				return err
			}
			exclusions, err := backupExclusions(txApp.DataDir(), payloadPath)
			if err != nil {
				return err
			}

			originalApp := event.App
			event.App = txApp
			event.Exclude = append(event.Exclude, exclusions...)
			defer func() { event.App = originalApp }()
			return event.Next()
		})
	})
}

func backupExclusions(dataDir, payloadPath string) ([]string, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("read PocketBase data directory: %w", err)
	}

	var exclusions []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "storage" && payloadPath != "" {
			storageExclusions, err := excludeStorage(dataDir, payloadPath)
			if err != nil {
				return nil, err
			}
			exclusions = append(exclusions, storageExclusions...)
		} else if !isDatabaseFile(name) {
			exclusions = append(exclusions, name)
		}
	}
	return exclusions, nil
}

func isDatabaseFile(name string) bool {
	switch name {
	case "data.db", "data.db-wal", "data.db-shm", "auxiliary.db", "auxiliary.db-wal", "auxiliary.db-shm":
		return true
	default:
		return false
	}
}

func excludeStorage(dataDir, payloadPath string) ([]string, error) {
	var exclusions []string
	keepPath := filepath.FromSlash(payloadPath)
	for keepPath != "storage" {
		parent := filepath.Dir(keepPath)
		entries, err := os.ReadDir(filepath.Join(dataDir, parent))
		if err != nil {
			return nil, fmt.Errorf("read PocketBase storage: %w", err)
		}
		for _, entry := range entries {
			path := filepath.Join(parent, entry.Name())
			if path != keepPath {
				exclusions = append(exclusions, filepath.ToSlash(path))
			}
		}
		keepPath = parent
	}
	return exclusions, nil
}
