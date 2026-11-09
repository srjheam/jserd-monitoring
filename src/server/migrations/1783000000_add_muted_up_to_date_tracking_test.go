package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAddMutedUpToDateTrackingMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	assertMutedUpToDateFieldsPresent(t, app, "missing field %s")
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tracking.Fields.GetByName(fieldMuted).(*core.BoolField); !ok {
		t.Fatal("muted must be a bool field")
	}

	if err := dropMutedUpToDateTracking(app); err != nil {
		t.Fatal(err)
	}
	assertMutedUpToDateFieldsAbsent(t, app, "field %s survived rollback")

	if err := addMutedUpToDateTracking(app); err != nil {
		t.Fatal(err)
	}
	assertMutedUpToDateFieldsPresent(t, app, "missing field %s after re-applying migration")
}

var mutedUpToDateTrackingFields = []string{fieldMuted, fieldMutedDigest, fieldUpToDateAt, fieldUpToDateDigest, fieldUpToDateUntil}

func assertMutedUpToDateFieldsPresent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range mutedUpToDateTrackingFields {
		if tracking.Fields.GetByName(field) == nil {
			t.Fatalf(failMsg, field)
		}
	}
}

func assertMutedUpToDateFieldsAbsent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range mutedUpToDateTrackingFields {
		if tracking.Fields.GetByName(field) != nil {
			t.Fatalf(failMsg, field)
		}
	}
}
