package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// LatestSnapshotPayloadPath returns the latest payload path relative to DataDir.
// It returns an empty path when no snapshots exist.
func LatestSnapshotPayloadPath(app core.App) (string, error) {
	previous, err := findPreviousSnapshot(app)
	if err != nil || len(previous) == 0 {
		return "", err
	}

	record := previous[0]
	name := record.GetString("payload")
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("latest snapshot %s has no payload file", record.Id)
	}

	relative := filepath.Clean(filepath.Join("storage", record.BaseFilesPath(), name))
	if !filepath.IsLocal(relative) {
		return "", fmt.Errorf("latest snapshot %s has an invalid payload path", record.Id)
	}
	if err := checkSnapshotPath(app.DataDir(), relative); err != nil {
		return "", fmt.Errorf("latest snapshot %s payload: %w", record.Id, err)
	}

	return filepath.ToSlash(relative), nil
}

func checkSnapshotPath(dataDir, relative string) error {
	current := dataDir
	parts := strings.Split(relative, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("path contains a symbolic link")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return errors.New("path component is not a directory")
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return errors.New("payload is not a regular file")
		}
	}
	return nil
}
