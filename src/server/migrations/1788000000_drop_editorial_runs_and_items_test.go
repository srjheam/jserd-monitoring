package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDropEditorialRunsAndItemsMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	assertRunsAndItemsCollectionsAbsent(t, app, "%s survived the up migration")
	assertRunFieldsAfterUpMigration(t, app)

	if err := restoreEditorialRunsAndItems(app); err != nil {
		t.Fatal(err)
	}
	assertRunsAndItemsCollectionsPresent(t, app)
	assertRunFieldsAfterDownMigration(t, app)

	if err := dropEditorialRunsAndItems(app); err != nil {
		t.Fatal(err)
	}
	assertRunsAndItemsCollectionsAbsent(t, app, "%s survived re-applying the up migration")
}

func assertRunsAndItemsCollectionsAbsent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	for _, name := range []string{collectionEditorialRuns, collectionEditorialItems} {
		if _, err := app.FindCollectionByNameOrId(name); err == nil {
			t.Fatalf(failMsg, name)
		}
	}
}

func assertRunsAndItemsCollectionsPresent(t *testing.T, app core.App) {
	t.Helper()
	for _, name := range []string{collectionEditorialRuns, collectionEditorialItems} {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Fatalf("%s not restored by the down migration: %v", name, err)
		}
	}
}

func assertRunFieldsAfterUpMigration(t *testing.T, app core.App) {
	t.Helper()
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Fields.GetByName(fieldRun) != nil {
		t.Fatal("manual_collection_requests.run survived the up migration")
	}
	attempts, err := app.FindCollectionByNameOrId("editorial_delivery_attempts")
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Fields.GetByName(fieldRun) != nil {
		t.Fatal("editorial_delivery_attempts.run survived the up migration")
	}
	if attempts.Fields.GetByName(fieldCycle) == nil {
		t.Fatal("editorial_delivery_attempts.cycle missing after the up migration")
	}
}

func assertRunFieldsAfterDownMigration(t *testing.T, app core.App) {
	t.Helper()
	requests, err := app.FindCollectionByNameOrId("manual_collection_requests")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Fields.GetByName(fieldRun) == nil {
		t.Fatal("manual_collection_requests.run not restored by the down migration")
	}
	attempts, err := app.FindCollectionByNameOrId("editorial_delivery_attempts")
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Fields.GetByName(fieldRun) == nil {
		t.Fatal("editorial_delivery_attempts.run not restored by the down migration")
	}
	if attempts.Fields.GetByName(fieldCycle) != nil {
		t.Fatal("editorial_delivery_attempts.cycle survived the down migration")
	}
}
