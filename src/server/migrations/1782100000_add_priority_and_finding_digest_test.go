package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAddPriorityAndFindingDigestMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	assertSubmissionsFieldsPresent(t, app, "missing field %s")

	// 1783100000_add_muted_and_up_to_date_digest_to_submissions.go indexes
	// effective_priority, so it must roll back before this migration can drop
	// that column, the same way a real `migrate down` unwinds migrations in
	// reverse order.
	if err := dropMutedAndUpToDateDigestFromSubmissions(app); err != nil {
		t.Fatal(err)
	}
	if err := dropPriorityAndFindingDigest(app); err != nil {
		t.Fatal(err)
	}
	assertSubmissionsFieldsAbsent(t, app, "field %s survived rollback")

	if err := addPriorityAndFindingDigest(app); err != nil {
		t.Fatal(err)
	}
	if err := addMutedAndUpToDateDigestToSubmissions(app); err != nil {
		t.Fatal(err)
	}
	assertSubmissionsFieldsPresent(t, app, "missing field %s after re-applying migration")
}

// assertSubmissionsFieldsPresent checks that effective_priority and
// finding_digest are present on ojs_submissions, using failMsg (which takes
// the field name) on mismatch.
func assertSubmissionsFieldsPresent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	submissions := findOjsSubmissions(t, app)
	for _, field := range []string{fieldEffectivePriority, fieldFindingDigest} {
		if submissions.Fields.GetByName(field) == nil {
			t.Fatalf(failMsg, field)
		}
	}
}

// assertSubmissionsFieldsAbsent checks that effective_priority and
// finding_digest are absent from ojs_submissions, using failMsg (which takes
// the field name) on mismatch.
func assertSubmissionsFieldsAbsent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	submissions := findOjsSubmissions(t, app)
	for _, field := range []string{fieldEffectivePriority, fieldFindingDigest} {
		if submissions.Fields.GetByName(field) != nil {
			t.Fatalf(failMsg, field)
		}
	}
}

func findOjsSubmissions(t *testing.T, app core.App) *core.Collection {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	return submissions
}
