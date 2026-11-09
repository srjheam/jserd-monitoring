package testseed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

const pocketbaseModule = "github.com/pocketbase/pocketbase"

// seedHash covers every non-test migration file and the PocketBase version,
// because PocketBase runs its own core migrations from a version-pinned
// directory.
func seedHash() (string, error) {
	names, err := migrationFileNames()
	if err != nil {
		return "", err
	}
	return hashFiles(names)
}

// migrationFileNames lists the non-test Go source files in migrationsDir,
// sorted so the hash does not depend on directory read order.
func migrationFileNames() ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if isMigrationSource(entry) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// isMigrationSource reports whether entry is a migration file that changes
// the seed's schema, as opposed to a directory or a test file.
func isMigrationSource(entry os.DirEntry) bool {
	if entry.IsDir() {
		return false
	}
	name := entry.Name()
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

// hashFiles hashes every named file under migrationsDir, in order, plus the
// PocketBase version.
func hashFiles(names []string) (string, error) {
	hasher := sha256.New()
	for _, name := range names {
		if err := hashFile(hasher, name); err != nil {
			return "", err
		}
	}
	_, _ = fmt.Fprintf(hasher, "pocketbase@%s", pocketbaseVersion())
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// hashFile feeds one migration file's name and content into hasher.
func hashFile(hasher io.Writer, name string) error {
	data, err := os.ReadFile(filepath.Join(migrationsDir, name))
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	_, _ = fmt.Fprintf(hasher, "%s\n", name)
	_, _ = hasher.Write(data)
	return nil
}

// pocketbaseVersion is the pinned dependency version, read from the build
// info that "go build" and "go test" embed in every binary.
func pocketbaseVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == pocketbaseModule {
			return dep.Version
		}
	}
	return "unknown"
}

// isHashName reports whether name has the shape seedHash produces.
func isHashName(name string) bool {
	if len(name) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(name)
	return err == nil
}
