package testseed

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pocketbase/pocketbase/core"
	_ "server/migrations"
)

// seedRootPerm is the permission bits for seedRoot and its build
// directories. Only the current user needs access.
const seedRootPerm = 0o755

// build creates the seed in a temporary directory and commits it onto dir.
func build(dir string) (string, error) {
	tmp, err := makeBuildDir()
	if err != nil {
		return "", err
	}
	if err := migrate(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	return commit(tmp, dir)
}

// makeBuildDir reserves a fresh, uniquely named directory under seedRoot to
// build a candidate seed in.
func makeBuildDir() (string, error) {
	if err := os.MkdirAll(seedRoot, seedRootPerm); err != nil {
		return "", fmt.Errorf("make seed root: %w", err)
	}
	tmp, err := os.MkdirTemp(seedRoot, "build-*")
	if err != nil {
		return "", fmt.Errorf("make build dir: %w", err)
	}
	return tmp, nil
}

// migrate bootstraps a PocketBase app at dir and runs every migration
// against it, then releases the app's resources without deleting dir.
func migrate(dir string) error {
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: dir, EncryptionEnv: "pb_test_env"})
	if err := app.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap seed: %w", err)
	}
	return runMigrations(app)
}

// runMigrations applies every migration and then always releases the app's
// resources, reporting whichever of the two steps failed first.
func runMigrations(app *core.BaseApp) error {
	migrateErr := app.RunAllMigrations()
	resetErr := app.ResetBootstrapState()
	if migrateErr != nil {
		return fmt.Errorf("run seed migrations: %w", migrateErr)
	}
	if resetErr != nil {
		return fmt.Errorf("run seed migrations: %w", resetErr)
	}
	return nil
}

// commit renames tmp onto dir. A rename onto a name that does not exist yet
// is atomic. A rename onto a directory that another caller just built fails
// with ENOTEMPTY, so the loser deletes its own copy and uses the winner's.
func commit(tmp, dir string) (string, error) {
	if err := os.Rename(tmp, dir); err != nil {
		_ = os.RemoveAll(tmp)
		return winnerOrError(dir, err)
	}
	removeSiblings(dir)
	return dir, nil
}

// winnerOrError is the result for a caller that lost the rename race: the
// winner's path, if the winner's build is the reason the rename failed.
func winnerOrError(dir string, renameErr error) (string, error) {
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, nil
	}
	return "", fmt.Errorf("rename build dir: %w", renameErr)
}

// removeSiblings deletes every other hash-named directory under seedRoot. A
// stale seed has no readers once the hash changes.
func removeSiblings(winner string) {
	entries, err := os.ReadDir(seedRoot)
	if err != nil {
		return
	}
	winnerName := filepath.Base(winner)
	for _, entry := range entries {
		removeIfStaleSeed(entry.Name(), winnerName)
	}
}

// removeIfStaleSeed deletes name if it is a previous seed's hash directory.
// A name that is not shaped like a hash is a losing racer's in-progress
// build directory, and removeIfStaleSeed leaves it alone.
func removeIfStaleSeed(name, winnerName string) {
	if name == winnerName || !isHashName(name) {
		return
	}
	_ = os.RemoveAll(filepath.Join(seedRoot, name))
}
