package state

import (
	"encoding/json"
	"server/internal/collections"
	"server/internal/events"
	"server/internal/normalizer"
	"testing"
	"time"
)

func TestCopyeditingFirstCyclePersistsOneEnrichedEvent(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	cycle := successfulCycle("", "")
	absent := false
	cycle.Normalized.Reviews = nil
	cycle.Normalized.Submissions[0].Status = "Copyediting"
	cycle.Normalized.Submissions[0].DateSubmitted = "2026-09-01"
	cycle.Normalized.Submissions[0].CopyeditorAssigned = &absent
	cycle.Normalized.Submissions[0].Editors = []normalizer.Editor{{GivenName: testEditorGivenName, FamilyName: testEditorFamilyName, Email: testEditorEmail}}
	for range 2 {
		if err := store.ProcessCycle(cycle); err != nil {
			t.Fatal(err)
		}
		cycle.StartedAt = cycle.StartedAt.Add(time.Hour)
		cycle.FinishedAt = cycle.FinishedAt.Add(time.Hour)
	}
	records, err := app.FindAllRecords(collections.Events)
	if err != nil || len(records) != 1 {
		t.Fatalf("events=%d err=%v", len(records), err)
	}
	var event events.Event
	if err := json.Unmarshal([]byte(records[0].GetString(fieldPayload)), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != events.EventMissingCopyeditor || event.SubmissionID != "42" || len(event.EditorTargetKeys) != 1 || event.EditorTargetKeys[0] != "ana-silva" {
		t.Fatalf("event=%#v", event)
	}
	matches, err := app.FindAllRecords(collections.EventRuleMatches)
	if err != nil || len(matches) != 1 || matches[0].GetString("rule_code") != events.RuleRN18 {
		t.Fatalf("matches=%#v err=%v", matches, err)
	}
}
