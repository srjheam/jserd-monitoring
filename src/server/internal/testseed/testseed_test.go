package testseed

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// withIsolatedSeed points seedRoot and migrationsDir at throwaway
// directories for the duration of a test, so the test never touches the
// real .runtime/test_seed and never depends on the repo's real migrations.
func withIsolatedSeed(t *testing.T, migration string) {
	t.Helper()
	root := t.TempDir()
	migrations := t.TempDir()
	writeMigration(t, migrations, migration)

	originalRoot, originalMigrations := seedRoot, migrationsDir
	seedRoot, migrationsDir = root, migrations
	t.Cleanup(func() { seedRoot, migrationsDir = originalRoot, originalMigrations })
}

func writeMigration(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "0001_seed.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDirReturnsAMigratedDataDirectory(t *testing.T) {
	withIsolatedSeed(t, "package migrations // v1")

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	app := core.NewBaseApp(core.BaseAppConfig{DataDir: dir, EncryptionEnv: "pb_test_env"})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.ResetBootstrapState(); err != nil {
			t.Fatal(err)
		}
	}()
	var applied int
	if err := app.DB().NewQuery("SELECT COUNT(*) AS c FROM _migrations").Row(&applied); err != nil {
		t.Fatal(err)
	}
	if applied == 0 {
		t.Fatal("seed has no applied migrations")
	}
}

func TestDirSecondCallReturnsTheSamePathAndDoesNotRebuild(t *testing.T) {
	withIsolatedSeed(t, "package migrations // v1")

	first, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(first, "sentinel")
	if err := os.WriteFile(sentinel, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("second call returned %q, want the same path %q", second, first)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("second call rebuilt the seed, sentinel gone: %v", err)
	}
}

func TestDirAfterAMigrationChangeReturnsADifferentPath(t *testing.T) {
	withIsolatedSeed(t, "package migrations // v1")

	first, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	writeMigration(t, migrationsDir, "package migrations // v2")

	second, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatalf("changed migration reused the same seed path %q", first)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("stale seed %q still exists after the hash changed", first)
	}
}

func TestDirConcurrentCallersAllGetOneWinningBuild(t *testing.T) {
	withIsolatedSeed(t, "package migrations // v1")

	const callers = 8
	var waitGroup sync.WaitGroup
	paths := make([]string, callers)
	errs := make([]error, callers)
	for i := range callers {
		waitGroup.Add(1)
		go func(i int) {
			defer waitGroup.Done()
			paths[i], errs[i] = Dir()
		}(i)
	}
	waitGroup.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Dir() call %d: %v", i, err)
		}
	}
	want := paths[0]
	for i, got := range paths {
		if got != want {
			t.Fatalf("call %d returned %q, want %q", i, got, want)
		}
	}
	entries, err := os.ReadDir(seedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = entry.Name()
		}
		t.Fatalf("seed root has %d entries after a concurrent build, want 1: %v", len(entries), names)
	}
}
