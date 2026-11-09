package state

import (
	"encoding/json"
	"server/internal/audit"
	"server/internal/collections"
	"server/internal/collector"
	"server/internal/config"
	"server/internal/events"
	"server/internal/normalizer"
	"server/internal/testseed"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "server/migrations"
)

const (
	testEditorGivenName  = "Ana"
	testEditorFamilyName = "Silva"
	testEditorEmail      = "ana@example.test"
	paddedEditorName     = "  Ana  "
)

func TestEventPayloadCarriesNotificationFields(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	baseline := successfulCycle("", "")
	baseline.Normalized.Submissions[0].URL = "https://ojs.example.test/submission/42"
	baseline.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 2, GivenName: "Bia", Email: "bia@example.test"}, {Sequence: 1, GivenName: testEditorGivenName, Email: testEditorEmail}}
	if err := store.ProcessCycle(baseline); err != nil {
		t.Fatal(err)
	}
	changed := successfulCycle("2026-06-01T10:00:00Z", "")
	changed.StartedAt = changed.StartedAt.Add(time.Hour)
	changed.FinishedAt = changed.FinishedAt.Add(time.Hour)
	changed.Normalized.Submissions = baseline.Normalized.Submissions
	changed.Normalized.Reviews[0].ResponseDueDate = "2026-06-10"
	if err := store.ProcessCycle(changed); err != nil {
		t.Fatal(err)
	}
	records, err := app.FindAllRecords(collections.Events)
	if err != nil || len(records) == 0 {
		t.Fatalf("events=%d err=%v", len(records), err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(records[0].GetString(fieldPayload)), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["submissionId"] != "42" || payload["title"] != "Paper" || payload["ojs_url"] != "https://ojs.example.test/submission/42" || payload["reviewerName"] != "Revisora Silva" {
		t.Fatalf("payload=%#v", payload)
	}
	keys, ok := payload["editorTargetKeys"].([]any)
	if !ok || len(keys) != 2 || keys[0] != "ana" || keys[1] != "bia" {
		t.Fatalf("editorTargetKeys=%#v", payload["editorTargetKeys"])
	}
}

func TestEnrichEventComposesReviewerName(t *testing.T) {
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{{Key: "review-1", ReviewerGivenName: paddedEditorName, ReviewerFamilyName: "  Silva  "}}}
	for _, eventType := range []string{events.EventReviewInvitationAccepted, events.EventReviewInvitationRejected, events.EventReviewResponseDue, events.EventReviewSubmitted} {
		for _, testCase := range []struct {
			given, family, want string
		}{
			{given: paddedEditorName, family: "  Silva  ", want: "Ana Silva"},
			{given: paddedEditorName, want: testEditorGivenName},
			{family: " Silva  ", want: testEditorFamilyName},
		} {
			cycle.Reviews[0].ReviewerGivenName = testCase.given
			cycle.Reviews[0].ReviewerFamilyName = testCase.family
			event := enrichEvent(events.Event{Type: eventType, EntityType: "review", EntityKey: "review-1"}, cycle)
			if event.ReviewerName != testCase.want {
				t.Fatalf("%s reviewer name=%q, want %q", eventType, event.ReviewerName, testCase.want)
			}
		}
	}
}

func TestEnrichEventResolvesSubmissionIDForReviewRound(t *testing.T) {
	cycle := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Editors: []normalizer.Editor{{Sequence: 1, GivenName: testEditorGivenName}}}},
	}
	event := enrichEvent(events.Event{Type: events.EventAllReviewsCompleted, EntityType: "review_round", EntityKey: "42|Review|1"}, cycle)
	if event.SubmissionID != "42" {
		t.Fatalf("submissionId=%q, want %q", event.SubmissionID, "42")
	}
	if len(event.EditorTargetKeys) != 1 || event.EditorTargetKeys[0] != "ana" {
		t.Fatalf("editorTargetKeys=%#v", event.EditorTargetKeys)
	}
}

func TestProcessCycleRejectsNewReviewerEventsWithoutName(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*normalizer.ReviewAssignment)
	}{
		{name: ruleCodeRN1, mutate: func(review *normalizer.ReviewAssignment) { review.DateConfirmed = "2026-06-01T10:00:00Z" }},
		{name: "RN2", mutate: func(review *normalizer.ReviewAssignment) {
			review.ReviewerUsername = "reviewer-1"
			review.ResponseDueDate = "2026-05-31"
			review.RoundHasOverdueResponse = true
		}},
		{name: "RN3", mutate: func(review *normalizer.ReviewAssignment) { review.DateCompleted = "2026-06-01T10:00:00Z" }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			app := newOperationalTestApp(t)
			store := NewStore(app)
			if err := store.ProcessCycle(successfulCycle("", "")); err != nil {
				t.Fatal(err)
			}
			changed := successfulCycle("", "")
			changed.StartedAt = changed.StartedAt.Add(time.Hour)
			changed.FinishedAt = changed.FinishedAt.Add(time.Hour)
			changed.Normalized.Reviews[0].ReviewerGivenName = ""
			changed.Normalized.Reviews[0].ReviewerFamilyName = ""
			testCase.mutate(&changed.Normalized.Reviews[0])
			if err := store.ProcessCycle(changed); err == nil {
				t.Fatal("accepted new reviewer event without name")
			}
			assertCounts(t, app, 1, 1, 0, 0)
		})
	}
}

func TestEditorReplacementUsesCurrentCycleAssociations(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	baseline := successfulCycle("", "")
	baseline.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: "Antiga", Email: "old@example.test"}}
	if err := store.ProcessCycle(baseline); err != nil {
		t.Fatal(err)
	}
	changed := successfulCycle("2026-06-01T10:00:00Z", "")
	changed.StartedAt = changed.StartedAt.Add(time.Hour)
	changed.FinishedAt = changed.FinishedAt.Add(time.Hour)
	changed.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 2, GivenName: "Atual", Email: "new@example.test"}}
	if err := store.ProcessCycle(changed); err != nil {
		t.Fatal(err)
	}
	records, _ := app.FindAllRecords(collections.Events)
	var event events.Event
	if len(records) == 0 || json.Unmarshal([]byte(records[0].GetString(fieldPayload)), &event) != nil {
		t.Fatalf("event payload unavailable")
	}
	if len(event.EditorTargetKeys) != 1 || event.EditorTargetKeys[0] != "atual" {
		t.Fatalf("editor keys=%v", event.EditorTargetKeys)
	}
}

func TestSyncRecipientsMergesEditorAndReviewerSharingAName(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	cycle := successfulCycle("", "")
	cycle.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: testEditorGivenName, FamilyName: testEditorFamilyName, Email: testEditorEmail}}
	cycle.Normalized.Reviews[0].ReviewerUsername = "ana-username"
	cycle.Normalized.Reviews[0].ReviewerGivenName = testEditorGivenName
	cycle.Normalized.Reviews[0].ReviewerFamilyName = testEditorFamilyName
	cycle.Normalized.Reviews[0].ReviewerEmail = testEditorEmail
	if err := store.ProcessCycle(cycle); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Recipients); got != 1 {
		t.Fatalf("recipients=%d, want exactly one row for a person holding both roles", got)
	}
	records, err := app.FindAllRecords(collections.Recipients)
	if err != nil {
		t.Fatal(err)
	}
	if !records[0].GetBool("is_editor") || !records[0].GetBool("is_reviewer") {
		t.Fatalf("recipient=%#v, want both roles flagged", records[0])
	}
	if records[0].GetString("identity") != "ana-username" {
		t.Fatalf("identity=%q, want the reviewer username to win over the slug", records[0].GetString("identity"))
	}
}

func TestSyncRecipientsRenameKeepsSameRow(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	first := successfulCycle("", "")
	first.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: "Nome A", Email: "same@example.test"}}
	if err := store.ProcessCycle(first); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Recipients); got != 1 {
		t.Fatalf("recipients=%d after first sync", got)
	}
	second := successfulCycle("", "")
	second.StartedAt = second.StartedAt.Add(time.Hour)
	second.FinishedAt = second.FinishedAt.Add(time.Hour)
	second.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: "Nome B", Email: "same@example.test"}}
	if err := store.ProcessCycle(second); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Recipients); got != 1 {
		t.Fatalf("recipients=%d after rename, want the same row reused, not a duplicate", got)
	}
	records, err := app.FindAllRecords(collections.Recipients)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].GetString("name") != "Nome B" || records[0].GetString("name_slug") != "nome-b" {
		t.Fatalf("recipient=%#v, want renamed in place by the shared e-mail fallback", records[0])
	}
}

func TestSyncRecipientsSkipsObservationsWithNoValidEmail(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	cycle := successfulCycle("", "")
	cycle.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: "Sem Email", Email: ""}}
	cycle.Normalized.Reviews[0].ReviewerUsername = "reviewer-no-email"
	cycle.Normalized.Reviews[0].ReviewerEmail = "not-an-email"
	if err := store.ProcessCycle(cycle); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Recipients); got != 0 {
		t.Fatalf("recipients=%d, want 0: no editor or reviewer here has a valid e-mail", got)
	}
}

// TestSyncRecipientsAcceptsNameSlugCollisionBetweenTwoRealEditors documents an
// accepted risk (ADR 0006, extended by ADR 0015): two different real editors
// who happen to share a display name collide onto one recipients row. This
// is intentional, not a bug to fix.
func TestSyncRecipientsAcceptsNameSlugCollisionBetweenTwoRealEditors(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	cycle := successfulCycle("", "")
	cycle.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: "João Silva", Email: "joao1@example.test"}}
	cycle.Normalized.Submissions = append(cycle.Normalized.Submissions, normalizer.Submission{
		OJSSubmissionID: "43", Title: "Outro artigo",
		Editors: []normalizer.Editor{{Sequence: 1, GivenName: "João Silva", Email: "joao2@example.test"}},
	})
	if err := store.ProcessCycle(cycle); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Recipients); got != 1 {
		t.Fatalf("recipients=%d, want 1: two different real João Silvas collide on the same slug by design", got)
	}
}

func TestPersistCycleFailedCreatesCycleOnly(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	result := failedCycle("download failed")
	if err := store.ProcessCycle(result); err != nil {
		t.Fatalf("ProcessCycle: %v", err)
	}
	assertCounts(t, app, 1, 0, 0, 0)
}

func TestRecipientSyncOnlyAfterValidCollection(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	valid := successfulCycle("", "")
	valid.Normalized.Submissions[0].Editors = []normalizer.Editor{{Sequence: 1, GivenName: testEditorGivenName, Email: testEditorEmail}}
	valid.Normalized.Reviews[0].ReviewerUsername = "reviewer-1"
	valid.Normalized.Reviews[0].ReviewerEmail = "reviewer@example.test"
	if err := store.ProcessCycle(valid); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Recipients); got != 2 {
		t.Fatalf("recipients=%d", got)
	}
	failed := failedCycle("download failed")
	failed.Normalized = valid.Normalized
	failed.Normalized.Submissions[0].Editors[0].Email = "changed@example.test"
	if err := store.ProcessCycle(failed); err != nil {
		t.Fatal(err)
	}
	records, err := app.FindAllRecords(collections.Recipients)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.GetString("email") == "changed@example.test" {
			t.Fatal("failed cycle changed recipient")
		}
	}
}

func TestPersistCycleSuccessCreatesBaselineAndRetainsSnapshots(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	first := successfulCycle("", "")
	if err := store.ProcessCycle(first); err != nil {
		t.Fatal(err)
	}
	assertCounts(t, app, 1, 1, 0, 0)
	second := successfulCycle("", "")
	second.StartedAt = second.StartedAt.Add(time.Hour)
	second.FinishedAt = second.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(second); err != nil {
		t.Fatal(err)
	}
	assertCounts(t, app, 2, 2, 0, 0)
}

func TestFindPreviousSnapshotReturnsNewest(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	first := successfulCycle("", "")
	if err := store.ProcessCycle(first); err != nil {
		t.Fatal(err)
	}
	second := successfulCycle("", "")
	second.StartedAt = second.StartedAt.Add(time.Hour)
	second.FinishedAt = second.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(second); err != nil {
		t.Fatal(err)
	}

	snapshots, err := findPreviousSnapshot(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots=%d", len(snapshots))
	}
	cycle, err := app.FindRecordById(collections.CollectionCycles, snapshots[0].GetString("cycle"))
	if err != nil {
		t.Fatal(err)
	}
	if !cycle.GetDateTime("finished_at").Time().Equal(second.FinishedAt) {
		t.Fatalf("finished_at=%s want=%s", cycle.GetDateTime("finished_at"), second.FinishedAt)
	}
}

func TestProcessCycleCreatesEventsMatchesAndDeduplicatesReplay(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	if err := store.ProcessCycle(successfulCycle("", "")); err != nil {
		t.Fatal(err)
	}
	changed := successfulCycle("2026-06-01T10:00:00Z", "2026-06-02T10:00:00Z")
	changed.StartedAt = changed.StartedAt.Add(time.Hour)
	changed.FinishedAt = changed.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(changed); err != nil {
		t.Fatal(err)
	}
	if got := countRecords(t, app, collections.Events); got == 0 {
		t.Fatal("second success produced no domain events")
	}
	if got := countRecords(t, app, collections.EventRuleMatches); got == 0 {
		t.Fatal("second success produced no pending rule matches")
	}
	eventsBefore, matchesBefore := countRecords(t, app, collections.Events), countRecords(t, app, collections.EventRuleMatches)
	replay := changed
	replay.StartedAt = replay.StartedAt.Add(time.Hour)
	replay.FinishedAt = replay.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(replay); err != nil {
		t.Fatal(err)
	}
	if countRecords(t, app, collections.Events) != eventsBefore || countRecords(t, app, collections.EventRuleMatches) != matchesBefore {
		t.Fatal("replay duplicated event or match")
	}
	if countRecords(t, app, collections.Snapshots) != 3 {
		t.Fatal("replay must retain its snapshot")
	}
}

func TestProcessCycleSerializesConcurrentCalls(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	if err := store.ProcessCycle(successfulCycle("", "")); err != nil {
		t.Fatal(err)
	}
	changed := successfulCycle("2026-06-01T10:00:00Z", "")
	var waitGroup sync.WaitGroup
	errs := make(chan error, 2)
	for goroutineIndex := range 2 {
		waitGroup.Add(1)
		go func(offset int) {
			defer waitGroup.Done()
			result := changed
			result.StartedAt = result.StartedAt.Add(time.Duration(offset+1) * time.Hour)
			result.FinishedAt = result.FinishedAt.Add(time.Duration(offset+1) * time.Hour)
			errs <- store.ProcessCycle(result)
		}(goroutineIndex)
	}
	waitGroup.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ProcessCycle: %v", err)
		}
	}
	assertCounts(t, app, 3, 3, 1, 1)
}

func TestProcessCycleRecoversExactUniqueConstraint(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	if err := store.ProcessCycle(successfulCycle("", "")); err != nil {
		t.Fatal(err)
	}
	realSave := store.saveRecord
	injected := false
	store.saveRecord = func(txApp core.App, model core.Model) error {
		record, ok := model.(*core.Record)
		if ok && record.Collection().Name == collections.Events && !injected {
			injected = true
			if err := realSave(txApp, cloneEventRecord(record, record.GetString("idempotency_key"))); err != nil {
				return err
			}
			return insertEventDirect(txApp, record, record.GetString("idempotency_key"))
		}
		return realSave(txApp, model)
	}
	changed := successfulCycle("2026-06-01T10:00:00Z", "")
	changed.StartedAt = changed.StartedAt.Add(time.Hour)
	changed.FinishedAt = changed.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(changed); err != nil {
		t.Fatalf("ProcessCycle recovered unique constraint: %v", err)
	}
	assertCounts(t, app, 2, 2, 1, 1)
}

func TestProcessCycleRollsBackUnmatchedUniqueConstraint(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	if err := store.ProcessCycle(successfulCycle("", "")); err != nil {
		t.Fatal(err)
	}
	realSave := store.saveRecord
	store.saveRecord = func(txApp core.App, model core.Model) error {
		record, ok := model.(*core.Record)
		if !ok || record.Collection().Name != collections.Events {
			return realSave(txApp, model)
		}
		first := cloneEventRecord(record, "different-key")
		if err := realSave(txApp, first); err != nil {
			return err
		}
		return insertEventDirect(txApp, record, "different-key")
	}
	changed := successfulCycle("2026-06-01T10:00:00Z", "")
	changed.StartedAt = changed.StartedAt.Add(time.Hour)
	changed.FinishedAt = changed.FinishedAt.Add(time.Hour)
	if err := store.ProcessCycle(changed); err == nil {
		t.Fatal("ProcessCycle accepted constraint without exact event key")
	}
	assertCounts(t, app, 1, 1, 0, 0)
}

func insertEventDirect(app core.App, source *core.Record, idempotencyKey string) error {
	record := cloneEventRecord(source, idempotencyKey)
	data, err := record.DBExport(app)
	if err != nil {
		return err
	}
	_, err = app.DB().Insert(record.TableName(), data).Execute()
	return err
}

func cloneEventRecord(source *core.Record, idempotencyKey string) *core.Record {
	record := core.NewRecord(source.Collection())
	for _, field := range []string{"snapshot", "type", "entity_type", "entity_key", "transition", "occurrence_key", fieldPayload, "occurred_at"} {
		record.Set(field, source.GetRaw(field))
	}
	record.Set("idempotency_key", idempotencyKey)
	return record
}

func TestRedactionPersistsNoSensitiveValues(t *testing.T) {
	app := newOperationalTestApp(t)
	store := NewStore(app)
	result := failedCycle("password=hunter2 token=abc cookie=session")
	if err := store.ProcessCycle(result); err != nil {
		t.Fatal(err)
	}
	records, err := app.FindAllRecords(collections.CollectionCycles)
	if err != nil {
		t.Fatal(err)
	}
	serialized := strings.ToLower(records[0].GetString("manifest") + records[0].GetString("error"))
	for _, secret := range []string{"hunter2", "abc", "session"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("persisted secret %q", secret)
		}
	}
}

func newOperationalTestApp(t *testing.T) *tests.TestApp {
	t.Helper()
	dir, err := testseed.Dir()
	if err != nil {
		t.Fatal(err)
	}
	app, err := tests.NewTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func failedCycle(message string) collector.CycleResult {
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	return collector.CycleResult{SourceMode: config.SourceModeFixtures, StartedAt: now, FinishedAt: now.Add(time.Minute), Status: collector.CycleStatusFailed, Manifest: audit.Manifest{SourceMode: sourceModeFixtures, CollectedAt: now, Errors: []string{message}}}
}

func successfulCycle(confirmed, completed string) collector.CycleResult {
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	review := normalizer.ReviewAssignment{Key: "42|Review|1|reviewer@example.test|2026-05-01", SubmissionID: "42", Stage: "Review", Round: 1, ReviewerEmail: "reviewer@example.test", ReviewerGivenName: "Revisora", ReviewerFamilyName: testEditorFamilyName, DateAssigned: "2026-05-01", DateConfirmed: confirmed, DateCompleted: completed}
	return collector.CycleResult{SourceMode: config.SourceModeFixtures, StartedAt: now, FinishedAt: now.Add(time.Minute), Status: collector.CycleStatusOK, Manifest: audit.Manifest{SourceMode: sourceModeFixtures, CollectedAt: now}, Normalized: normalizer.NormalizedCycle{SourceMode: sourceModeFixtures, CollectedAt: now, Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: "Paper"}}, Reviews: []normalizer.ReviewAssignment{review}}}
}

func countRecords(t *testing.T, app *tests.TestApp, name string) int {
	t.Helper()
	records, err := app.FindAllRecords(name)
	if err != nil {
		t.Fatal(err)
	}
	return len(records)
}

func assertCounts(t *testing.T, app *tests.TestApp, cycles, snapshots, events, matches int) {
	t.Helper()
	wants := map[string]int{collections.CollectionCycles: cycles, collections.Snapshots: snapshots, collections.Events: events, collections.EventRuleMatches: matches}
	for name, want := range wants {
		if got := countRecords(t, app, name); got != want {
			t.Fatalf("%s count = %d, want %d", name, got, want)
		}
	}
}
