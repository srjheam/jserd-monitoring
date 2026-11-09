package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactStoreWritesLiveBytesUnderRunRawDir(t *testing.T) {
	root := t.TempDir()
	store := ArtifactStore{Root: root}
	body := []byte("live csv bytes")

	path, err := store.WriteRaw("run-123", "articles", ".csv", body)
	if err != nil {
		t.Fatalf("write raw artifact: %v", err)
	}

	wantSuffix := filepath.Join("run-123", "raw", "articles.csv")
	if !strings.HasSuffix(path, wantSuffix) {
		t.Fatalf("path = %q, want suffix %q", path, wantSuffix)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written artifact: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("written body = %q, want %q", got, body)
	}
}

func TestArtifactStorePruneRunsKeepsNewestDeletesOlder(t *testing.T) {
	root := t.TempDir()
	store := ArtifactStore{Root: root}
	runIDs := []string{"20260601T000000Z", "20260601T010000Z", "20260601T020000Z", "20260601T030000Z"}
	for _, runID := range runIDs {
		if _, err := store.WriteRaw(runID, "articles", ".csv", []byte("x")); err != nil {
			t.Fatalf("write raw artifact: %v", err)
		}
	}

	if err := store.PruneRuns(2); err != nil {
		t.Fatalf("prune runs: %v", err)
	}

	for i, runID := range runIDs {
		_, err := os.Stat(filepath.Join(root, runID))
		wantGone := i < 2
		if wantGone && !os.IsNotExist(err) {
			t.Fatalf("run %s should have been pruned, stat err=%v", runID, err)
		}
		if !wantGone && err != nil {
			t.Fatalf("run %s should still exist, stat err=%v", runID, err)
		}
	}
}

func TestArtifactStorePruneRunsMissingRootIsNoop(t *testing.T) {
	store := ArtifactStore{Root: filepath.Join(t.TempDir(), "does-not-exist")}
	if err := store.PruneRuns(2); err != nil {
		t.Fatalf("prune runs on missing root: %v", err)
	}
}
