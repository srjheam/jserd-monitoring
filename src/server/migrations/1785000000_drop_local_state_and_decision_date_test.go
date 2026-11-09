package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDropLocalStateAndDecisionDateMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	assertLocalStateFieldsAbsent(t, app, "field %s survived the up migration")

	if err := restoreLocalStateAndDecisionDate(app); err != nil {
		t.Fatal(err)
	}
	assertLocalStateFieldsPresent(t, app, "down migration did not re-add field %s")

	if err := dropLocalStateAndDecisionDate(app); err != nil {
		t.Fatal(err)
	}
	assertLocalStateFieldsAbsent(t, app, "field %s survived re-applying the up migration")
}

var localStateAndDecisionDateFields = []string{fieldLocalState, fieldDecisionDate}

func assertLocalStateFieldsAbsent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range localStateAndDecisionDateFields {
		if tracking.Fields.GetByName(field) != nil {
			t.Fatalf(failMsg, field)
		}
	}
}

func assertLocalStateFieldsPresent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	tracking, err := app.FindCollectionByNameOrId("submission_tracking")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range localStateAndDecisionDateFields {
		if tracking.Fields.GetByName(field) == nil {
			t.Fatalf(failMsg, field)
		}
	}
}
