package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAddMutedAndUpToDateDigestToSubmissionsMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	assertMutedDigestFieldsPresent(t, app, "missing field %s")
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("effective_queue gets the muted and up_to_date values", func(t *testing.T) {
		assertEffectiveQueueHasValues(t, submissions)
	})
	t.Run("composite index is created", func(t *testing.T) {
		assertHasMutedUpToDateIndex(t, submissions)
	})

	if err := dropMutedAndUpToDateDigestFromSubmissions(app); err != nil {
		t.Fatal(err)
	}
	assertMutedDigestFieldsAbsent(t, app, "field %s survived rollback")

	if err := addMutedAndUpToDateDigestToSubmissions(app); err != nil {
		t.Fatal(err)
	}
	assertMutedDigestFieldsPresent(t, app, "missing field %s after re-applying migration")
}

func assertEffectiveQueueHasValues(t *testing.T, submissions *core.Collection) {
	t.Helper()
	queue, ok := submissions.Fields.GetByName("effective_queue").(*core.SelectField)
	if !ok {
		t.Fatal("effective_queue must be a select field")
	}
	for _, value := range []string{queueValueDaily, queueValueArchived, fieldMuted, queueValueUpToDate} {
		found := false
		for _, existing := range queue.Values {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("effective_queue is missing value %s", value)
		}
	}
}

func assertHasMutedUpToDateIndex(t *testing.T, submissions *core.Collection) {
	t.Helper()
	found := false
	for _, index := range submissions.Indexes {
		if index == "CREATE INDEX `idx_ojs_submissions_muted_up_to_date` ON `ojs_submissions` (`muted`, `up_to_date_digest`, `effective_priority`, `ojs_submission_id` COLLATE NOCASE)" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing muted/up_to_date composite index")
	}
}

func assertMutedDigestFieldsPresent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{fieldMuted, fieldUpToDateDigest} {
		if submissions.Fields.GetByName(field) == nil {
			t.Fatalf(failMsg, field)
		}
	}
}

func assertMutedDigestFieldsAbsent(t *testing.T, app core.App, failMsg string) {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{fieldMuted, fieldUpToDateDigest} {
		if submissions.Fields.GetByName(field) != nil {
			t.Fatalf(failMsg, field)
		}
	}
}
