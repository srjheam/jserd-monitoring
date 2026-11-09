package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultArtifactRoot = "../../.runtime/pb_data/collector_runs"

const (
	artifactDirPerm  = 0o755
	artifactFilePerm = 0o644
)

type ArtifactStore struct {
	Root string
}

func (s ArtifactStore) WriteRaw(runID, kind, ext string, body []byte) (string, error) {
	if err := validateArtifactName(runID, kind); err != nil {
		return "", err
	}
	dir := filepath.Join(s.root(), runID, "raw")
	if err := os.MkdirAll(dir, artifactDirPerm); err != nil {
		return "", err
	}
	path := filepath.Join(dir, kind+fileExtension(ext))
	if err := os.WriteFile(path, body, artifactFilePerm); err != nil {
		return "", err
	}
	return path, nil
}

// root is the directory every run directory lives under.
func (s ArtifactStore) root() string {
	root := strings.TrimSpace(s.Root)
	if root == "" {
		return DefaultArtifactRoot
	}
	return root
}

// validateArtifactName rejects an artifact that cannot be filed under a run.
func validateArtifactName(runID, kind string) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("runID is required")
	}
	if strings.TrimSpace(kind) == "" {
		return fmt.Errorf("kind is required")
	}
	return nil
}

// fileExtension is the file extension to write an artifact under. An artifact
// with no extension is written as .raw.
func fileExtension(ext string) string {
	if strings.TrimSpace(ext) == "" {
		return ".raw"
	}
	if strings.HasPrefix(ext, ".") {
		return ext
	}
	return "." + ext
}

// PruneRuns keeps only the `keep` most recent run directories under Root,
// deleting older ones. Run directory names are UTC timestamps
// (20060102T150405Z, see collector.go), which sort correctly as plain
// strings.
func (s ArtifactStore) PruneRuns(keep int) error {
	root := s.root()
	names, err := runDirNames(root)
	if err != nil || len(names) <= keep {
		return err
	}
	sort.Strings(names)
	return removeRuns(root, names[:len(names)-keep])
}

// removeRuns deletes the named run directories.
func removeRuns(root string, names []string) error {
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return fmt.Errorf("remove run %s: %w", name, err)
		}
	}
	return nil
}

// readRunEntries lists the entries under the artifact root. A root that does
// not exist yet holds no run.
func readRunEntries(root string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(root)
	if err == nil {
		return entries, nil
	}
	if os.IsNotExist(err) {
		return nil, nil
	}
	return nil, fmt.Errorf("read artifact root: %w", err)
}

func runDirNames(root string) ([]string, error) {
	entries, err := readRunEntries(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
