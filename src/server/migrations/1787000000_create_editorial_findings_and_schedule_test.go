package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestCreateEditorialFindingsAndScheduleMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	t.Run("editorial_findings has its fields", func(t *testing.T) {
		assertEditorialFindingsFields(t, app)
	})
	t.Run("editorial_schedule has its field", func(t *testing.T) {
		assertEditorialScheduleField(t, app)
	})

	if err := dropEditorialFindingsAndSchedule(app); err != nil {
		t.Fatal(err)
	}
	t.Run("down migration drops both collections", func(t *testing.T) {
		assertEditorialFindingsAndScheduleAbsent(t, app)
	})

	if err := createEditorialFindingsAndSchedule(app); err != nil {
		t.Fatal(err)
	}
	t.Run("re-applying the up migration restores both collections", func(t *testing.T) {
		assertEditorialFindingsAndSchedulePresent(t, app)
	})
}

func assertEditorialFindingsFields(t *testing.T, app core.App) {
	t.Helper()
	findings, err := app.FindCollectionByNameOrId("editorial_findings")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"submission_key", "date_submitted", fieldPayload} {
		if findings.Fields.GetByName(field) == nil {
			t.Fatalf("editorial_findings missing field %q", field)
		}
	}
}

func assertEditorialScheduleField(t *testing.T, app core.App) {
	t.Helper()
	schedule, err := app.FindCollectionByNameOrId("editorial_schedule")
	if err != nil {
		t.Fatal(err)
	}
	if schedule.Fields.GetByName("next_summary_due_at") == nil {
		t.Fatal("editorial_schedule missing field next_summary_due_at")
	}
}

func assertEditorialFindingsAndScheduleAbsent(t *testing.T, app core.App) {
	t.Helper()
	if _, err := app.FindCollectionByNameOrId("editorial_findings"); err == nil {
		t.Fatal("editorial_findings survived the down migration")
	}
	if _, err := app.FindCollectionByNameOrId("editorial_schedule"); err == nil {
		t.Fatal("editorial_schedule survived the down migration")
	}
}

func assertEditorialFindingsAndSchedulePresent(t *testing.T, app core.App) {
	t.Helper()
	if _, err := app.FindCollectionByNameOrId("editorial_findings"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("editorial_schedule"); err != nil {
		t.Fatal(err)
	}
}
