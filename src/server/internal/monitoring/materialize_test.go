package monitoring

import (
	"server/internal/collections"
	"server/internal/editorial"
	"server/internal/normalizer"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestFindingDigestIgnoresDeadlinesAndOverdueCounts(t *testing.T) {
	first := []editorial.Finding{{Kind: editorial.FindingOverdueReview, Responsible: targetKindReviewer, Deadline: testDate20260105}}
	second := []editorial.Finding{{Kind: editorial.FindingOverdueReview, Responsible: targetKindReviewer, Deadline: "2026-02-20"}}
	if editorial.DigestFor(first) != editorial.DigestFor(second) {
		t.Fatalf("digest changed with only the deadline: %s vs %s", editorial.DigestFor(first), editorial.DigestFor(second))
	}
}

func TestFindingDigestChangesWhenAKindIsAdded(t *testing.T) {
	before := []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}
	after := []editorial.Finding{{Kind: editorial.FindingMissingReviewer}, {Kind: editorial.FindingOverdueReview}}
	if editorial.DigestFor(before) == editorial.DigestFor(after) {
		t.Fatal("digest did not change when a finding kind was added")
	}
}

func TestFindingDigestChangesWhenAKindIsRemoved(t *testing.T) {
	before := []editorial.Finding{{Kind: editorial.FindingMissingReviewer}, {Kind: editorial.FindingOverdueReview}}
	after := []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}
	if editorial.DigestFor(before) == editorial.DigestFor(after) {
		t.Fatal("digest did not change when a finding kind was removed")
	}
}

func TestEmptyFindingDigestIsStable(t *testing.T) {
	a := editorial.DigestFor(nil)
	b := editorial.DigestFor([]editorial.Finding{})
	if a == "" || a != b {
		t.Fatalf("empty digest is not stable: %q vs %q", a, b)
	}
}

func TestEffectivePriorityMatchesEditorialRank(t *testing.T) {
	kinds := editorial.Kinds()
	fixtures := [][]editorial.Finding{
		nil,
		{{Kind: editorial.FindingMissingEditor}},
		{{Kind: editorial.FindingMissingReviewer}},
		{{Kind: editorial.FindingOverdueReview}, {Kind: editorial.FindingMissingEditor}},
		{{Kind: editorial.FindingMissingRevisionIndication}, {Kind: editorial.FindingUnsentReviewCompletion}},
	}
	for _, findings := range fixtures {
		worst := editorial.Rank(findings)
		want := len(kinds) + 1
		for i, kind := range kinds {
			if string(kind) == worst {
				want = i + 1
				break
			}
		}
		if got := editorial.PriorityFor(findings); got != want {
			t.Fatalf("editorial.PriorityFor(%#v)=%d want %d (editorial.Rank=%s)", findings, got, want, worst)
		}
	}
}

// TestRecomputeEffectiveStoresPriorityAndDigest exercises the full hook path:
// a tracking change recomputes effective_priority and finding_digest from the
// submission's current findings, keeping the digest stable when only the run
// changes and moving it when a finding kind is added or removed.
func TestRecomputeEffectiveStoresPriorityAndDigest(t *testing.T) {
	app := monitoringTestApp(t)

	normalized := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitle, Status: statusQueued, DateSubmitted: testDate20260101}},
	}
	cycle := saveSnapshot(t, app, normalized, time.Now().UTC())
	saveRun := func(findings []editorial.Finding) {
		t.Helper()
		if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, []editorial.ArticleFinding{{SubmissionID: "42", Findings: findings}}); err != nil {
			t.Fatal(err)
		}
	}
	loadRow := func() *core.Record {
		t.Helper()
		row, err := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", "42")
		if err != nil {
			t.Fatal(err)
		}
		return row
	}

	wantPriority, wantDigest := seedMildFindingAndAssertBaseline(t, app, saveRun, loadRow)
	tracking := seedSameKindNewerRunAndAssertDigestUnchanged(t, app, saveRun, loadRow, wantDigest)
	addedDigest := seedWorseFindingAndAssertEscalation(t, app, saveRun, loadRow, tracking, wantPriority, wantDigest)
	seedResolvedFindingAndAssertReturnToBaseline(t, app, saveRun, loadRow, tracking, wantPriority, wantDigest, addedDigest)
}

// seedMildFindingAndAssertBaseline saves the mildest finding kind, which
// leaves room to test an escalation to a worse kind in later steps, and
// returns the priority/digest it produces as the baseline for those steps.
func seedMildFindingAndAssertBaseline(t *testing.T, app core.App, saveRun func([]editorial.Finding), loadRow func() *core.Record) (int, string) {
	t.Helper()
	saveRun([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	saveTracking(t, app, "42", "")

	row := loadRow()
	wantPriority := editorial.PriorityFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	wantDigest := editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	if row.GetInt("effective_priority") != wantPriority || row.GetString("finding_digest") != wantDigest {
		t.Fatalf("priority=%d digest=%s want priority=%d digest=%s", row.GetInt("effective_priority"), row.GetString("finding_digest"), wantPriority, wantDigest)
	}
	return wantPriority, wantDigest
}

// seedSameKindNewerRunAndAssertDigestUnchanged re-runs with the same finding
// kind, as if a day passed and the deadline moved on: the digest must stay put.
func seedSameKindNewerRunAndAssertDigestUnchanged(t *testing.T, app core.App, saveRun func([]editorial.Finding), loadRow func() *core.Record, wantDigest string) *core.Record {
	t.Helper()
	saveRun([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	tracking := saveTracking(t, app, "42", "")
	tracking.Set("editor_note", "checked again")
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	row := loadRow()
	if row.GetString("finding_digest") != wantDigest {
		t.Fatalf("digest changed with only the run recency: %s want %s", row.GetString("finding_digest"), wantDigest)
	}
	return tracking
}

// seedWorseFindingAndAssertEscalation adds a worse finding kind, which must
// move the digest and worsen (lower) the priority, and returns the new digest.
func seedWorseFindingAndAssertEscalation(t *testing.T, app core.App, saveRun func([]editorial.Finding), loadRow func() *core.Record, tracking *core.Record, wantPriority int, wantDigest string) string {
	t.Helper()
	saveRun([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}, {Kind: editorial.FindingMissingEditor}})
	tracking.Set("editor_note", "escalated")
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	row := loadRow()
	if row.GetString("finding_digest") == wantDigest {
		t.Fatal("digest did not change when a finding kind was added")
	}
	if row.GetInt("effective_priority") >= wantPriority {
		t.Fatalf("priority did not worsen after a more severe finding was added: %d vs %d", row.GetInt("effective_priority"), wantPriority)
	}
	return row.GetString("finding_digest")
}

// seedResolvedFindingAndAssertReturnToBaseline removes the worse finding
// kind, which must move the digest again, back to where it started.
func seedResolvedFindingAndAssertReturnToBaseline(t *testing.T, app core.App, saveRun func([]editorial.Finding), loadRow func() *core.Record, tracking *core.Record, wantPriority int, wantDigest, addedDigest string) {
	t.Helper()
	saveRun([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	tracking.Set("editor_note", "resolved")
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	row := loadRow()
	if row.GetString("finding_digest") != wantDigest || row.GetString("finding_digest") == addedDigest {
		t.Fatalf("digest did not return to the missing_reviewer-only digest after removal: %s", row.GetString("finding_digest"))
	}
	if row.GetInt("effective_priority") != wantPriority {
		t.Fatalf("priority did not return to its starting value after removal: %d want %d", row.GetInt("effective_priority"), wantPriority)
	}
}

// muteUpToDateFixture seeds one submission and returns helpers to run an
// editorial cycle over it and to read back its materialized row, matching
// the pattern of TestRecomputeEffectiveStoresPriorityAndDigest.
func muteUpToDateFixture(t *testing.T, app core.App, submissionID string) (runFindings func(seq int, findings []editorial.Finding), row func() *core.Record) {
	t.Helper()
	normalized := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{OJSSubmissionID: submissionID, Title: testSubmissionTitle, Status: statusQueued, DateSubmitted: testDate20260101}},
	}
	cycle := saveSnapshot(t, app, normalized, time.Now().UTC())
	runFindings = func(seq int, findings []editorial.Finding) {
		t.Helper()
		if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, []editorial.ArticleFinding{{SubmissionID: submissionID, Findings: findings}}); err != nil {
			t.Fatal(err)
		}
		if err := OnFindingsReady(app, cycle.Id); err != nil {
			t.Fatal(err)
		}
	}
	row = func() *core.Record {
		t.Helper()
		r, err := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", submissionID)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	return runFindings, row
}

// TestUpToDateStaysUpToDateWhenFindingsUnchanged covers
// .scratch/dashboard-redesign/issues/03-muted-and-up-to-date.md: a
// re-collection with the same finding kinds must keep a submission Up to
// date.
func TestUpToDateStaysUpToDateWhenFindingsUnchanged(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, row := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	tracking := saveTracking(t, app, "1", "")
	tracking.Set("up_to_date_at", time.Now().UTC())
	tracking.Set("up_to_date_digest", editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}}))
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	if got := row().GetString("effective_queue"); got != queueUpToDate {
		t.Fatalf("effective_queue=%s want up_to_date", got)
	}

	// Re-collection with the same finding kind: still Up to date.
	runFindings(2, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	if got := row().GetString("effective_queue"); got != queueUpToDate {
		t.Fatalf("effective_queue after unchanged re-collection=%s want up_to_date", got)
	}
}

// TestUpToDateReturnsToWorkListWhenFindingsChange covers the same ticket: a
// changed finding kind must return the submission to the work list.
func TestUpToDateReturnsToWorkListWhenFindingsChange(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, row := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	tracking := saveTracking(t, app, "1", "")
	tracking.Set("up_to_date_at", time.Now().UTC())
	tracking.Set("up_to_date_digest", editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}}))
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}

	// A different finding kind changes the digest and must return the
	// submission to the work list.
	runFindings(2, []editorial.Finding{{Kind: editorial.FindingOverdueReview}})
	if got := row().GetString("effective_queue"); got != defaultQueue {
		t.Fatalf("effective_queue after changed findings=%s want daily", got)
	}
}

// TestNoFindingsIsUpToDateWithNoHumanAct covers the automatic case: a
// submission with no findings is Up to date without any chief editor action.
func TestNoFindingsIsUpToDateWithNoHumanAct(t *testing.T) {
	app := monitoringTestApp(t)
	normalized := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{OJSSubmissionID: "1", Title: testSubmissionTitle, Status: statusQueued, DateSubmitted: testDate20260101}},
	}
	saveSnapshot(t, app, normalized, time.Now().UTC())
	row, err := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got := row.GetString("effective_queue"); got != queueUpToDate {
		t.Fatalf("effective_queue=%s want up_to_date", got)
	}
}

// TestMutedSubmissionWithChangedFindingsStaysMutedAndReportsChange covers: a
// muted submission whose finding_digest differs from muted_digest stays
// Muted, but the data needed to report the change (the two digests) is
// available for the client marker.
func TestMutedSubmissionWithChangedFindingsStaysMutedAndReportsChange(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, row := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	tracking := saveTracking(t, app, "1", "")
	mutedDigest := editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	tracking.Set(queueMuted, true)
	tracking.Set("muted_digest", mutedDigest)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	if got := row().GetString("effective_queue"); got != queueMuted {
		t.Fatalf("effective_queue=%s want muted", got)
	}

	runFindings(2, []editorial.Finding{{Kind: editorial.FindingOverdueReview}})
	current := row()
	if got := current.GetString("effective_queue"); got != queueMuted {
		t.Fatalf("effective_queue after changed findings=%s want muted (still delegated)", got)
	}
	if current.GetString("finding_digest") == mutedDigest {
		t.Fatal("finding_digest did not change, so there is nothing to report")
	}
	fresh, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.GetString("muted_digest") != mutedDigest {
		t.Fatalf("muted_digest was mutated: %s want %s", fresh.GetString("muted_digest"), mutedDigest)
	}
}

// TestExpiredUpToDateUntilReturnsToWorkList covers the daily expiry: an
// up_to_date_until in the past returns the submission to the work list even
// though its findings did not change.
func TestExpiredUpToDateUntilReturnsToWorkList(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, row := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	tracking := saveTracking(t, app, "1", "")
	tracking.Set("up_to_date_at", time.Now().UTC().Add(-48*time.Hour))
	tracking.Set("up_to_date_digest", editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}}))
	tracking.Set("up_to_date_until", time.Now().UTC().Add(-time.Hour))
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	if got := row().GetString("effective_queue"); got != defaultQueue {
		t.Fatalf("effective_queue with an expired return date=%s want daily", got)
	}
}

// TestMutedOutranksUpToDate covers precedence: a submission that qualifies
// for both placements is Muted.
func TestMutedOutranksUpToDate(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, row := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	digest := editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	tracking := saveTracking(t, app, "1", "")
	tracking.Set(queueMuted, true)
	tracking.Set("muted_digest", digest)
	tracking.Set("up_to_date_at", time.Now().UTC())
	tracking.Set("up_to_date_digest", digest)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	if got := row().GetString("effective_queue"); got != queueMuted {
		t.Fatalf("effective_queue=%s want muted (muted outranks up to date)", got)
	}
}

// TestClearExpiredUpToDate covers the daily task: it clears the up-to-date
// fields of a tracking record whose return date has passed, and leaves one
// whose return date has not passed untouched.
func TestClearExpiredUpToDate(t *testing.T) {
	app := monitoringTestApp(t)
	expired := saveTracking(t, app, "1", "")
	expired.Set("up_to_date_at", time.Now().UTC().Add(-48*time.Hour))
	expired.Set("up_to_date_digest", "some-digest")
	expired.Set("up_to_date_until", time.Now().UTC().Add(-time.Hour))
	if err := app.Save(expired); err != nil {
		t.Fatal(err)
	}
	pending := saveTracking(t, app, "2", "")
	pending.Set("up_to_date_at", time.Now().UTC())
	pending.Set("up_to_date_digest", "some-digest")
	pending.Set("up_to_date_until", time.Now().UTC().Add(48*time.Hour))
	if err := app.Save(pending); err != nil {
		t.Fatal(err)
	}

	if err := clearExpiredUpToDate(app); err != nil {
		t.Fatal(err)
	}

	after, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if after.GetString("up_to_date_at") != "" || after.GetString("up_to_date_digest") != "" || after.GetString("up_to_date_until") != "" {
		t.Fatalf("expired up-to-date fields were not cleared: at=%q digest=%q until=%q", after.GetString("up_to_date_at"), after.GetString("up_to_date_digest"), after.GetString("up_to_date_until"))
	}
	stillPending, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "2")
	if err != nil {
		t.Fatal(err)
	}
	if stillPending.GetString("up_to_date_digest") != "some-digest" {
		t.Fatal("a return date in the future must not be cleared")
	}
}

// TestMutingStampsTheFindingDigest covers the write side of `muted_digest`,
// which ticket 03 specifies as the finding digest at the moment of the mute.
// Without it the Muted screen can never tell a finding that appeared after
// the mute from one that was already there.
func TestMutingStampsTheFindingDigest(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, _ := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})
	want := editorial.DigestFor([]editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	tracking := saveTracking(t, app, "1", "")
	tracking.Set(queueMuted, true)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	stamped, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got := stamped.GetString("muted_digest"); got != want {
		t.Fatalf("muted_digest=%q want %q", got, want)
	}

	// The findings change while the submission stays muted. The stamp records
	// the state at mute time, so it must not follow them.
	runFindings(2, []editorial.Finding{{Kind: editorial.FindingOverdueReview}})
	kept, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got := kept.GetString("muted_digest"); got != want {
		t.Fatalf("muted_digest followed the findings: %q want %q", got, want)
	}
}

// TestUnmutingClearsTheFindingDigest covers the other edge of the same stamp.
// A conference has no lifecycle, so the only way out of Muted is a
// chief editor; a stale stamp left behind would mark nothing as new the next
// time the same submission is muted.
func TestUnmutingClearsTheFindingDigest(t *testing.T) {
	app := monitoringTestApp(t)
	runFindings, _ := muteUpToDateFixture(t, app, "1")
	runFindings(1, []editorial.Finding{{Kind: editorial.FindingMissingReviewer}})

	tracking := saveTracking(t, app, "1", "")
	tracking.Set(queueMuted, true)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	tracking.Set(queueMuted, false)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
	cleared, err := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got := cleared.GetString("muted_digest"); got != "" {
		t.Fatalf("muted_digest=%q want empty after an unmute", got)
	}
}

// derivedColumns are the seven columns effectiveState carries. The list is
// spelled here so the test fails loudly when a column joins effectiveState and
// nothing carries it into the projection.
var derivedColumns = []string{
	"effective_final_state",
	"effective_queue",
	"effective_attention",
	"effective_priority",
	"finding_digest",
	"muted",
	"up_to_date_digest",
}

// The row the materialize path stores and the state applyEffective derives must
// agree on every derived column. They used to be computed in three places:
// acceptedSubmissions did three columns, storeSubmission did four, and
// applyEffective did all seven. A column added to one of the three and not the
// others caused a stored row
// and a projection that disagreed depending on which route served the request.
//
// The second saveSnapshot is what makes this test real: it re-materializes the
// row through storeSubmission with the curation already in place, so the values
// compared below are the ones the store path wrote, not the ones a tracking
// hook wrote through applyEffective.
func TestStoredRowAndRecomputedStateAgreeOnEveryDerivedColumn(t *testing.T) {
	app := monitoringTestApp(t)
	normalized := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{OJSSubmissionID: "1", Title: testSubmissionTitle, Status: statusQueued, DateSubmitted: testDate20260101}},
	}
	cycle := saveSnapshot(t, app, normalized, time.Now().UTC())
	findings := []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}
	if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, []editorial.ArticleFinding{{SubmissionID: "1", Findings: findings}}); err != nil {
		t.Fatal(err)
	}

	tracking := saveTracking(t, app, "1", "")
	tracking.Set("muted", true)
	tracking.Set("up_to_date_at", time.Now().UTC())
	tracking.Set("up_to_date_digest", editorial.DigestFor(findings))
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}

	saveSnapshot(t, app, normalized, time.Now().UTC().Add(time.Minute))
	row := func() *core.Record {
		t.Helper()
		record, err := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", "1")
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	stored := map[string]any{}
	for _, column := range derivedColumns {
		stored[column] = row().Get(column)
	}

	if err := recomputeAll(app); err != nil {
		t.Fatal(err)
	}
	for _, column := range derivedColumns {
		if got := row().Get(column); got != stored[column] {
			t.Errorf("%s: the stored row says %v, the recomputed state says %v", column, stored[column], got)
		}
	}
}

func TestCopyeditorAssignmentSurvivesCurationAndRecollection(t *testing.T) {
	app := monitoringTestApp(t)
	falseValue, trueValue := false, true
	findings := []editorial.Finding{{Kind: editorial.FindingMissingCopyeditor}}
	for index, phase := range []struct {
		assigned *bool
		findings []editorial.Finding
		want     string
	}{{&falseValue, findings, defaultQueue}, {&trueValue, nil, queueArchived}, {&falseValue, findings, defaultQueue}, {nil, nil, queueUpToDate}} {
		cycle := saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "copyedit", Title: testSubmissionTitle, Status: statusCopyediting, CopyeditorAssigned: phase.assigned}}}, time.Now().UTC().Add(time.Duration(index)*time.Minute))
		if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, []editorial.ArticleFinding{{SubmissionID: "copyedit", Findings: phase.findings}}); err != nil {
			t.Fatal(err)
		}
		if err := OnFindingsReady(app, cycle.Id); err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			saveTracking(t, app, "copyedit", "")
		}
		detail, found, err := FindSubmission(app, "copyedit")
		if err != nil || !found {
			t.Fatalf("phase %d: detail=%#v found=%v err=%v", index, detail, found, err)
		}
		assignmentMatches := detail.CopyeditorAssigned == nil && phase.assigned == nil || detail.CopyeditorAssigned != nil && phase.assigned != nil && *detail.CopyeditorAssigned == *phase.assigned
		if detail.EffectiveQueue != phase.want || detail.EffectiveFinalState != "" || !assignmentMatches {
			t.Fatalf("phase %d: detail=%#v found=%v err=%v", index, detail, found, err)
		}
	}
}
