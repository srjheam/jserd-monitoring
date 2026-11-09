package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/tools/osutils"
)

// requireEnvFile stops the server when the .env file is missing, because every
// setting comes from it.
func requireEnvFile() {
	_, err := os.Stat(".env")
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "arquivo .env ausente; copie src/server/.env.example para .env antes de iniciar o servidor")
		os.Exit(1)
	}
	log.Fatalf("erro ao verificar .env: %v", err)
}

// newApp builds the PocketBase instance. Under "go run" it points the data
// directory at the repo-root .runtime folder instead of the Go module, so a
// tool that copies the module directory does not also copy the database. A
// built binary keeps the stock behavior, because its working directory is
// already outside any Go module.
func newApp() *pocketbase.PocketBase {
	isGoRun := osutils.IsProbablyGoRun()
	cfg := pocketbase.Config{DefaultDev: isGoRun}
	if isGoRun {
		cfg.DefaultDataDir = filepath.Join("..", "..", ".runtime", "pb_data")
	}
	return pocketbase.NewWithConfig(cfg)
}
