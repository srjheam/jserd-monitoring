// Package testseed builds a pre-migrated PocketBase data directory and
// hands its path to every caller that needs one. tests.NewTestApp() clones
// a data directory and then replays every migration, which costs about
// 1076 ms against 17 ms with a directory that is already migrated.
package testseed

import (
	"os"
	"path/filepath"
	"runtime"
)

var (
	seedRoot      = repoPath(".runtime", "test_seed")
	migrationsDir = repoPath("src", "server", "migrations")
)

// repoPath resolves a path relative to this file's location on disk, so the
// result does not depend on the working directory of whichever test binary
// calls Dir().
func repoPath(parts ...string) string {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	return filepath.Join(append([]string{repoRoot}, parts...)...)
}

// Dir returns a path to a migrated PocketBase data directory. The directory
// is content-addressed by every non-test migration file and the PocketBase
// version, so a changed migration can never serve a stale seed: it names a
// directory that does not exist yet, and Dir builds it.
func Dir() (string, error) {
	hash, err := seedHash()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(seedRoot, hash)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, nil
	}
	return build(dir)
}
