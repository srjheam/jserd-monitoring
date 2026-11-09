package dberr_test

import (
	"database/sql"
	"errors"
	"fmt"
	"server/internal/dberr"
	"testing"

	_ "modernc.org/sqlite"
)

// uniqueViolation provokes a real driver error, rather than building one by
// hand. The point of IsConstraint is that it reads what this driver actually
// returns, so a hand-made value would test nothing.
func uniqueViolation(t *testing.T) error {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Fatalf("close: %v", closeErr)
		}
	}()
	if _, err = database.Exec("CREATE TABLE claims (key TEXT UNIQUE)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err = database.Exec("INSERT INTO claims (key) VALUES ('one')"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = database.Exec("INSERT INTO claims (key) VALUES ('one')")
	if err == nil {
		t.Fatal("second insert succeeded, so there is no constraint error to read")
	}
	return err
}

func TestIsConstraintReadsAUniqueViolation(t *testing.T) {
	if !dberr.IsConstraint(uniqueViolation(t)) {
		t.Error("a unique violation must read as a constraint error")
	}
}

// The claim paths wrap the save error before they test it, so IsConstraint has
// to see through a wrap.
func TestIsConstraintReadsAWrappedUniqueViolation(t *testing.T) {
	wrapped := fmt.Errorf("save claim: %w", uniqueViolation(t))
	if !dberr.IsConstraint(wrapped) {
		t.Error("a wrapped unique violation must read as a constraint error")
	}
}

func TestIsConstraintRejectsOtherErrors(t *testing.T) {
	cases := map[string]error{
		"no error":        nil,
		"a plain error":   errors.New("connection reset"),
		"a syntax error":  syntaxError(t),
		"a wrapped plain": fmt.Errorf("save claim: %w", errors.New("disk full")),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if dberr.IsConstraint(err) {
				t.Errorf("%v must not read as a constraint error", err)
			}
		})
	}
}

// syntaxError is a driver error that is not a constraint. It proves that
// IsConstraint reads the code rather than the driver's type alone.
func syntaxError(t *testing.T) error {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Fatalf("close: %v", closeErr)
		}
	}()
	_, err = database.Exec("SELECT FROM nowhere")
	if err == nil {
		t.Fatal("a broken statement succeeded, so there is no error to read")
	}
	return err
}
