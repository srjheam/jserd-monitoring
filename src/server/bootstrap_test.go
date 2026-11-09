package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRequireEnvFileReturnsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("X=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	requireEnvFile()
}

// TestRequireEnvFileExitsWhenMissing re-execs this test binary in a directory
// with no .env file: requireEnvFile calls os.Exit(1) on that path, which
// would otherwise kill the real test process.
func TestRequireEnvFileExitsWhenMissing(t *testing.T) {
	if os.Getenv("REQUIRE_ENV_FILE_SUBPROCESS") == "1" {
		requireEnvFile()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRequireEnvFileExitsWhenMissing")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "REQUIRE_ENV_FILE_SUBPROCESS=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.Success() {
		t.Fatalf("expected a failing exit when .env is missing, got %v", err)
	}
}

func TestNewAppBuildsAPocketBaseInstance(t *testing.T) {
	app := newApp()
	if app == nil {
		t.Fatal("newApp returned nil")
	}
	if app.DataDir() == "" {
		t.Fatal("newApp must configure a data directory")
	}
}
