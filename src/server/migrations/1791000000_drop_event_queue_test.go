package migrations

import (
	"encoding/json"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDropEventQueueMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	// The migration already ran once during app bootstrap, so "event" must be
	// restored before a row can be seeded with it.
	if err := restoreEventQueue(app); err != nil {
		t.Fatal(err)
	}
	seedEventQueueRow(t, app, "1")

	if err := dropEventQueue(app); err != nil {
		t.Fatal(err)
	}
	t.Run("event rows fall through to daily", func(t *testing.T) {
		assertEventQueueRowFallsThrough(t, app)
	})
	t.Run("effective_queue no longer allows event", func(t *testing.T) {
		assertEffectiveQueueLacksEvent(t, app)
	})

	if err := restoreEventQueue(app); err != nil {
		t.Fatal(err)
	}
	t.Run("rollback restores event to effective_queue", func(t *testing.T) {
		assertEffectiveQueueHasEvent(t, app)
	})

	if err := dropEventQueue(app); err != nil {
		t.Fatal(err)
	}
}

func assertEventQueueRowFallsThrough(t *testing.T, app core.App) {
	t.Helper()
	var queueValue string
	if err := app.DB().NewQuery("SELECT effective_queue FROM ojs_submissions WHERE ojs_submission_id = '1'").Row(&queueValue); err != nil {
		t.Fatal(err)
	}
	if queueValue != queueValueDaily {
		t.Fatalf("expected the 'event' row to fall through to 'daily', got %s", queueValue)
	}
}

func effectiveQueueField(t *testing.T, app core.App) *core.SelectField {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	queue, isSelectField := submissions.Fields.GetByName("effective_queue").(*core.SelectField)
	if !isSelectField {
		t.Fatal("effective_queue must be a select field")
	}
	return queue
}

func assertEffectiveQueueLacksEvent(t *testing.T, app core.App) {
	t.Helper()
	queue := effectiveQueueField(t, app)
	for _, value := range queue.Values {
		if value == fieldEvent {
			t.Fatal("effective_queue must no longer allow 'event'")
		}
	}
}

func assertEffectiveQueueHasEvent(t *testing.T, app core.App) {
	t.Helper()
	queue := effectiveQueueField(t, app)
	found := false
	for _, value := range queue.Values {
		if value == fieldEvent {
			found = true
		}
	}
	if !found {
		t.Fatal("rollback must restore 'event' to effective_queue's allowed values")
	}
}

func seedEventQueueRow(t *testing.T, app core.App, submissionID string) {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId("ojs_submissions")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := json.Marshal(map[string]any{"editors": []string{}, "decisions": []any{}, "reviews": []any{}, "provenance": map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(evidence, &decoded); err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(submissions)
	record.Set(fieldOJSSubmissionID, submissionID)
	record.Set("title", "Sample submission")
	record.Set(fieldStatus, "Queued")
	record.Set("effective_queue", fieldEvent)
	record.Set("effective_attention", "normal")
	record.Set(fieldCycle, "cycle-"+submissionID)
	record.Set("evidence", decoded)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
}
