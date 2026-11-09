package migrations

import (
	"slices"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestScopeActivityLookupMigrationAddsAndRemovesIndexes(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	t.Run("event indexes are added", func(t *testing.T) {
		assertHasIndexes(t, app, "events", activityLookupEventIndexes, "missing event index: %s")
	})
	t.Run("email_delivery_attempts indexes are added", func(t *testing.T) {
		assertHasIndexes(t, app, "email_delivery_attempts", activityLookupAttemptIndexes, "missing email_delivery_attempts index: %s")
	})

	if err := unscopeActivityLookup(app); err != nil {
		t.Fatal(err)
	}
	t.Run("rollback removes the event indexes", func(t *testing.T) {
		assertLacksIndexes(t, app, "events", activityLookupEventIndexes, "rollback must remove event index: %s")
	})

	if err := scopeActivityLookup(app); err != nil {
		t.Fatal(err)
	}
}

func assertHasIndexes(t *testing.T, app core.App, collectionName string, indexes []string, failMsg string) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if !slices.Contains(collection.Indexes, index) {
			t.Fatalf(failMsg, index)
		}
	}
}

func assertLacksIndexes(t *testing.T, app core.App, collectionName string, indexes []string, failMsg string) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if slices.Contains(collection.Indexes, index) {
			t.Fatalf(failMsg, index)
		}
	}
}
