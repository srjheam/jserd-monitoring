package monitoring

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"server/internal/audit"
	"server/internal/collections"
	"server/internal/collector"
	"server/internal/config"
	"server/internal/editorial"
	"server/internal/normalizer"
	"server/internal/state"
	"server/internal/testseed"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "server/migrations"
)

func TestCurrentStatusOwnsFinalOutcome(t *testing.T) {
	decisionHistory := []normalizer.Editor{{Decisions: []normalizer.EditorDecision{{Decision: "Accepted", DateDecided: testDate20260301}, {Decision: statusRejected, DateDecided: "2026-03-02"}}}}
	finding := []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}
	active := acceptedSubmissions(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "1", Status: "Review", Editors: decisionHistory}}}, nil, map[string][]editorial.Finding{"1": finding})[0]
	if active.EffectiveQueue != defaultQueue || active.EffectiveFinalState != "" || len(active.Decisions) != 2 {
		t.Fatalf("decision history must remain active and visible: %#v", active)
	}

	for _, testCase := range []struct{ status, final string }{{statusPublished, finalAccepted}, {statusDeclined, finalRejected}, {statusRejected, finalRejected}, {statusWithdrawn, finalWithdrawn}} {
		got := acceptedSubmissions(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: testCase.status, Status: testCase.status, Editors: decisionHistory}}}, nil, nil)[0]
		if got.EffectiveQueue != queueArchived || got.EffectiveFinalState != testCase.final {
			t.Errorf("status %q => queue=%q final=%q", testCase.status, got.EffectiveQueue, got.EffectiveFinalState)
		}
	}
}

func TestCopyeditingPlacementRequiresConfirmedAssignment(t *testing.T) {
	finding := editorial.Finding{Kind: editorial.FindingMissingCopyeditor}
	falseValue := false
	acceptedHistory := []normalizer.Editor{{Decisions: []normalizer.EditorDecision{{Decision: "Accept Submission", DateDecided: testDate20260301}}}}
	assignedFalse := acceptedSubmissions(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "false", Status: statusCopyediting, CopyeditorAssigned: &falseValue, Editors: acceptedHistory}}}, nil, map[string][]editorial.Finding{"false": {finding}})[0]
	if assignedFalse.EffectiveQueue != defaultQueue || assignedFalse.EffectiveFinalState != "" || len(assignedFalse.Decisions) != 1 {
		t.Fatalf("false assignment with accepted history=%#v", assignedFalse)
	}
	unknownHistory := acceptedSubmissions(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: testSubmissionIDUnknown, Status: statusCopyediting, Editors: acceptedHistory}}}, nil, nil)[0]
	if unknownHistory.EffectiveQueue != queueUpToDate || unknownHistory.EffectiveFinalState != "" {
		t.Fatalf("unknown assignment with accepted history=%#v", unknownHistory)
	}
	for _, testCase := range []struct {
		name      string
		assigned  *bool
		tracking  *LocalTracking
		findings  []editorial.Finding
		wantQueue string
	}{
		{"false muted", &falseValue, &LocalTracking{Muted: true}, []editorial.Finding{finding}, queueMuted},
		{"false set aside", &falseValue, &LocalTracking{UpToDateDigest: editorial.DigestFor([]editorial.Finding{finding})}, []editorial.Finding{finding}, queueUpToDate},
		{"unknown without findings", nil, nil, nil, queueUpToDate},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			state := deriveEffective(statusCopyediting, testCase.assigned, testCase.tracking, testCase.findings)
			if state.queue != testCase.wantQueue || state.finalState != "" {
				t.Fatalf("queue=%q final=%q, want queue=%q and no final state", state.queue, state.finalState, testCase.wantQueue)
			}
		})
	}
	trueValue := true
	if state := deriveEffective(statusCopyediting, &trueValue, nil, []editorial.Finding{finding}); state.queue != queueArchived || state.finalState != "" {
		t.Fatalf("confirmed assignment queue=%q final=%q, want archived with no final state", state.queue, state.finalState)
	}
}

// OJS's live "Reviews" export marks Declined true on almost every row
// regardless of the reviewer's real status (see isOpenForOverdueCheck in
// internal/editorial/review_status.go). safeReview must not read that
// field either, or the dashboard mislabels an accepted or completed review
// as "declined".
func TestSafeReviewIgnoresTheAlwaysTrueDeclinedFlag(t *testing.T) {
	completed := normalizer.ReviewAssignment{Round: 2, Declined: true, DateCompleted: "2026-08-01"}
	if got := safeReview(completed); got.Status != "completed" {
		t.Fatalf("completed review must report completed, got %q", got.Status)
	}
	accepted := normalizer.ReviewAssignment{Round: 2, Declined: true, DateConfirmed: "2026-08-01"}
	if got := safeReview(accepted); got.Status != "awaiting_review" {
		t.Fatalf("accepted, not-yet-due review must report awaiting_review, got %q", got.Status)
	}
	overdue := normalizer.ReviewAssignment{Round: 2, Declined: true, ReviewOverdueDays: 3}
	if got := safeReview(overdue); got.Status != "overdue" {
		t.Fatalf("overdue review must report overdue, got %q", got.Status)
	}
}

func TestCycleSuccessHookMaterializesSubmissionsAndRoutesServeThem(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	assertInitialCycleMaterializesWithAttention(t, app, mux, token)
	assertConferenceAssignmentKeepsQueue(t, app, mux, token)
	assertRejectionArchivesAndFailureDoesNotLeak(t, app, mux, token)
	assertPublishedArchivesAndMissingSubmission404s(t, app, mux, token)
}

// assertInitialCycleMaterializesWithAttention saves the first cycle and an
// editorial run, then checks the derived attention, the meta/page routes,
// and that no private field leaks into either response.
func assertInitialCycleMaterializesWithAttention(t *testing.T, app core.App, mux http.Handler, token string) {
	t.Helper()
	seedInitialCycleAndRun(t, app)
	assertInitialCycleRoutesServeMaterializedData(t, mux, token)
}

// seedInitialCycleAndRun saves the first cycle, checks the derived
// attention starts at "normal" with no findings, then saves an editorial
// run and checks the completed run alone moves the materialized queue and
// attention (no tracking record exists for 42 yet).
func seedInitialCycleAndRun(t *testing.T, app core.App) {
	t.Helper()
	first := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{
			OJSSubmissionID: "42", Title: testSubmissionTitleFull, Status: statusQueued, URL: "https://ojs.example.test/submission/42", DateSubmitted: testDate20260101,
			Authors: []normalizer.Author{{Email: "author@example.test"}},
			Editors: []normalizer.Editor{{GivenName: "Editor", FamilyName: "Name", Email: "editor@example.test"}},
		}},
		Reviews: []normalizer.ReviewAssignment{{SubmissionID: "42", ReviewerEmail: testReviewerEmail, ReviewDueDate: testDate20260105}},
	}
	cycle := saveSnapshot(t, app, first, time.Now().UTC())
	if row, _ := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", "42"); row.GetString("effective_attention") != "normal" {
		t.Fatalf("attention without findings=%s", row.GetString("effective_attention"))
	}
	saveEditorialRun(t, app, cycle.Id, "42", []editorial.Finding{
		{Kind: editorial.FindingOverdueReview, Responsible: targetKindReviewer, Deadline: testDate20260105, Contact: testReviewerEmail},
		{Kind: editorial.FindingMissingReviewer, Responsible: targetKindEditor},
	})
	rows, err := app.FindAllRecords(collections.OJSSubmissions)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GetString("ojs_submission_id") != "42" || rows[0].GetString("effective_queue") != defaultQueue {
		t.Fatalf("materialized=%#v", rows)
	}
	if rows[0].GetString("effective_attention") != findingOverdueReview {
		t.Fatalf("run completion did not derive attention: %s", rows[0].GetString("effective_attention"))
	}
}

func assertInitialCycleRoutesServeMaterializedData(t *testing.T, mux http.Handler, token string) {
	t.Helper()
	meta := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", token)
	if meta.Availability != StateAvailable || meta.Aggregates.Total != 1 || meta.Aggregates.Triage[defaultQueue] != 1 || meta.Aggregates.DailyRequiringAttention != 1 {
		t.Fatalf("meta=%#v", meta)
	}
	if len(meta.Aggregates.Statuses) != 1 || meta.Aggregates.Statuses[0] != statusQueued {
		t.Fatalf("statuses=%#v", meta.Aggregates.Statuses)
	}
	if len(meta.Aggregates.Editors) != 1 || meta.Aggregates.Editors[0] != (EditorFrequency{Slug: "editor-name", Label: "Editor Name", Count: 1}) {
		t.Fatalf("editors=%#v", meta.Aggregates.Editors)
	}
	page := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=10&queue=daily", token)
	if page.Total != 1 || page.NextCursor != "" || len(page.Items) != 1 {
		t.Fatalf("page=%#v", page)
	}
	item := page.Items[0]
	if item.URL != "https://ojs.example.test/submission/42" || item.DateSubmitted != testDate20260101 || len(item.Findings) != 2 || item.Findings[0].Kind != findingOverdueReview || item.EffectiveAttention != findingOverdueReview || item.Tracking != nil {
		t.Fatalf("item=%#v", item)
	}
	serialized := rawGet(t, mux, "/api/jserd/monitoring/submissions", token) + rawGet(t, mux, "/api/jserd/monitoring", token)
	for _, forbidden := range []string{"author@example.test", "editor@example.test", testReviewerEmail, "contact", "snapshot.json"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("private value %q leaked", forbidden)
		}
	}
}

func assertConferenceAssignmentKeepsQueue(t *testing.T, app core.App, mux http.Handler, token string) {
	t.Helper()
	conference := saveConference(t, app, "Conference issue")
	saveTracking(t, app, "42", conference.Id)
	row, err := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", "42")
	if err != nil {
		t.Fatal(err)
	}
	if row.GetString("effective_queue") != defaultQueue {
		t.Fatalf("assigning a conference must not move a row out of daily: %s", row.GetString("effective_queue"))
	}
	conferencePage := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?conference="+conference.Id, token)
	if conferencePage.Total != 1 || len(conferencePage.Items) != 1 || conferencePage.Items[0].OJSSubmissionID != "42" {
		t.Fatalf("conference filter=%#v", conferencePage)
	}
	if conferencePage.Items[0].Tracking == nil || conferencePage.Items[0].Tracking.Conference != conference.Id {
		t.Fatalf("conference relation wire value=%#v", conferencePage.Items[0].Tracking)
	}
	meta := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", token)
	if len(meta.Conferences) != 1 || meta.Conferences[0].ID != conference.Id {
		t.Fatalf("conference metadata=%#v", meta.Conferences)
	}
}

func assertRejectionArchivesAndFailureDoesNotLeak(t *testing.T, app core.App, mux http.Handler, token string) {
	t.Helper()
	rejected := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{
		OJSSubmissionID: "42", Title: testSubmissionTitleFull, Status: statusDeclined,
		Editors: []normalizer.Editor{{Decisions: []normalizer.EditorDecision{{Decision: statusRejected, DateDecided: "2026-01-03"}}}},
	}}}
	saveSnapshot(t, app, rejected, time.Now().UTC().Add(time.Second))
	single := getJSON[submissionResponse](t, mux, "/api/jserd/monitoring/submissions/42", token)
	if single.EffectiveQueue != queueArchived || single.EffectiveFinalState != finalRejected || single.Tracking == nil {
		t.Fatalf("single=%#v", single)
	}

	failed := saveCycle(t, app, "failed", time.Now().UTC().Add(time.Second), "token=secret")
	afterFailure := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", token)
	if afterFailure.Availability != StateAvailable || afterFailure.LatestAttempt == nil || afterFailure.LatestAttempt.ID != failed.Id {
		t.Fatalf("after failure=%#v", afterFailure)
	}
	if strings.Contains(rawGet(t, mux, "/api/jserd/monitoring", token), "secret") {
		t.Fatal("failure leaked secret")
	}
}

func assertPublishedArchivesAndMissingSubmission404s(t *testing.T, app core.App, mux http.Handler, token string) {
	t.Helper()
	second := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitleFull, Status: statusPublished}}}
	saveSnapshot(t, app, second, time.Now().UTC().Add(2*time.Second))
	final := getJSON[submissionResponse](t, mux, "/api/jserd/monitoring/submissions/42", token)
	if final.EffectiveQueue != queueArchived || final.EffectiveFinalState != finalAccepted || final.Tracking == nil {
		t.Fatalf("OJS archive/tracking survival=%#v", final)
	}
	if response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/submissions/missing", nil, token); response.Code != http.StatusNotFound {
		t.Fatalf("missing submission=%d", response.Code)
	}
}

// TestIncompleteOJSSubmissionIsExcludedFromMaterialization covers that OJS
// has not finished accepting a submission whenever submissionProgress != 0,
// so Rebuild must not materialize it into ojs_submissions at all.
func TestIncompleteOJSSubmissionIsExcludedFromMaterialization(t *testing.T) {
	app := monitoringTestApp(t)
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: "complete", Title: "Finished submission", Status: statusQueued},
		{OJSSubmissionID: "incomplete", Title: "", Status: "", SubmissionProgress: 2},
	}}, time.Now().UTC())

	rows, err := app.FindAllRecords(collections.OJSSubmissions)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GetString("ojs_submission_id") != "complete" {
		t.Fatalf("materialized=%#v, want only the complete submission", rows)
	}
}

// TestOnFindingsReadyReflectsTheLatestReplace covers that editorial_findings
// holds one row per submission, unconditionally the latest cycle's findings:
// a replace tied to a cycle other than the one that just materialized this
// submission still wins the moment OnFindingsReady runs, since nothing scopes
// the lookup to a specific cycle.
func TestOnFindingsReadyReflectsTheLatestReplace(t *testing.T) {
	app := monitoringTestApp(t)
	cycle := saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitle, Status: statusQueued}}}, time.Now().UTC())
	store := editorial.NewStore(app)
	if err := store.ReplaceFindings(cycle.Id, []editorial.ArticleFinding{{SubmissionID: "42", Findings: []editorial.Finding{{Kind: editorial.FindingMissingEditor}}}}); err != nil {
		t.Fatal(err)
	}
	if err := OnFindingsReady(app, cycle.Id); err != nil {
		t.Fatal(err)
	}
	other := saveCycle(t, app, "succeeded", time.Now().UTC().Add(-3*time.Hour), "")
	if err := store.ReplaceFindings(other.Id, []editorial.ArticleFinding{{SubmissionID: "42", Findings: []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}}}); err != nil {
		t.Fatal(err)
	}
	if err := OnFindingsReady(app, other.Id); err != nil {
		t.Fatal(err)
	}
	row, err := app.FindFirstRecordByData(collections.OJSSubmissions, "ojs_submission_id", "42")
	if err != nil {
		t.Fatal(err)
	}
	if row.GetString("effective_attention") != "missing_reviewer" {
		t.Fatalf("the latest replace did not win: %s", row.GetString("effective_attention"))
	}
}

func TestEmptySuccessWithNewerFailedAttemptStaysExplicitlyEmpty(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	finished := time.Now().UTC().Add(-time.Minute)
	cycle := saveSnapshot(t, app, normalizer.NormalizedCycle{}, finished)
	failed := saveCycle(t, app, "failed", finished.Add(time.Second), "token=secret")
	meta := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", token)
	if meta.Availability != StateEmpty || meta.LatestSuccess == nil || meta.LatestSuccess.ID != cycle.Id || meta.LatestAttempt == nil || meta.LatestAttempt.ID != failed.Id {
		t.Fatalf("meta=%#v", meta)
	}
}

func TestStaleSuccessIsFlaggedWhenOlderThanTwoDays(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "7", Title: "Old", Status: statusQueued}}}, time.Now().UTC().Add(-49*time.Hour))
	meta := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", chiefEditorToken(t, app))
	if meta.Availability != StateStale || meta.Aggregates.Total != 1 {
		t.Fatalf("stale meta=%#v", meta)
	}
}

// TestSuperuserCredentialRejectedByEveryRoute checks
// decision that a Superuser no longer signs in to the dashboard: all eight
// routes require a chief_editors token now, and a _superusers one is
// forbidden even though it used to be the only accepted credential.
func TestSuperuserCredentialRejectedByEveryRoute(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := superuserToken(t, app)
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitle, Status: statusQueued}}}, time.Now().UTC())
	routes := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/jserd/monitoring"},
		{http.MethodGet, "/api/jserd/monitoring/submissions"},
		{http.MethodGet, "/api/jserd/monitoring/aggregates/editors"},
		{http.MethodGet, "/api/jserd/monitoring/submissions/42"},
		{http.MethodGet, "/api/jserd/monitoring/submissions/42/activity"},
		{http.MethodPost, "/api/jserd/monitoring/submissions/42/journal"},
		{http.MethodPost, "/api/jserd/monitoring/submissions/42/up-to-date"},
		{http.MethodPost, "/api/jserd/monitoring/submissions/42/send-back-to-work"},
	}
	for _, route := range routes {
		if response := apiCall(mux, route.method, route.path, []byte(`{"body":"x"}`), token); response.Code != http.StatusForbidden {
			t.Fatalf("%s %s with a superuser token = %d, want %d", route.method, route.path, response.Code, http.StatusForbidden)
		}
	}
}

func TestListSubmissionsKeysetPaginatesAcrossPages(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRow(t, app, "1", "Bravo")
	seedRow(t, app, "2", "Alpha")
	seedRow(t, app, "3", "Alpha")
	seedRow(t, app, "4", "Charlie")
	seedRow(t, app, "5", "Echo")

	firstNextCursor := assertKeysetPagesInOrder(t, mux, token)
	assertInvalidPaginationQueriesReject(t, mux, token, firstNextCursor)
}

func submissionPageIDs(page submissionPage) []string {
	values := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		values = append(values, item.OJSSubmissionID)
	}
	return values
}

// assertKeysetPagesInOrder walks the five seeded rows page by page (both
// directions, plus an unpaginated id-sorted fetch) and returns the first
// page's next cursor for the invalid-query checks that follow.
func assertKeysetPagesInOrder(t *testing.T, mux http.Handler, token string) string {
	t.Helper()
	first := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=2&sort=title&dir=asc", token)
	if got := submissionPageIDs(first); !equalStrings(got, []string{"2", "3"}) || first.NextCursor == "" || first.Total != 5 {
		t.Fatalf("first page=%#v", first)
	}
	second := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=2&sort=title&dir=asc&cursor="+first.NextCursor, token)
	if got := submissionPageIDs(second); !equalStrings(got, []string{"1", "4"}) || second.NextCursor == "" {
		t.Fatalf("second page=%#v", second)
	}
	third := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=2&sort=title&dir=asc&cursor="+second.NextCursor, token)
	if got := submissionPageIDs(third); !equalStrings(got, []string{"5"}) || third.NextCursor != "" {
		t.Fatalf("third page=%#v", third)
	}
	descending := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=2&sort=title&dir=desc", token)
	if got := submissionPageIDs(descending); !equalStrings(got, []string{"5", "4"}) {
		t.Fatalf("descending page=%#v", descending)
	}
	byID := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=100&sort=id", token)
	if got := submissionPageIDs(byID); !equalStrings(got, []string{"1", "2", "3", "4", "5"}) {
		t.Fatalf("id page=%#v", byID)
	}
	return first.NextCursor
}

func assertInvalidPaginationQueriesReject(t *testing.T, mux http.Handler, token, firstNextCursor string) {
	t.Helper()
	if response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/submissions?cursor=@@bad@@", nil, token); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor=%d", response.Code)
	}
	response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/submissions?limit=2&sort=id&dir=asc&cursor="+firstNextCursor, nil, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("mismatched cursor sort=%d %s", response.Code, response.Body.String())
	}
	if response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/submissions?limit=abc", nil, token); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit=%d", response.Code)
	}
	if response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/submissions?sort=nope", nil, token); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid sort=%d", response.Code)
	}
}

func TestListSubmissionsAppliesFilters(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	conference := seedListFiltersFixture(t, app)
	count := func(query string) int {
		page := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?"+query, token)
		return page.Total
	}

	assertQueueAttentionAndStatusFilters(t, count, conference.Id)
	assertFreeTextAndEditorFilters(t, app, count)
}

// seedListFiltersFixture seeds four rows across the daily/archive queues,
// with one assigned to a conference, and returns that conference.
func seedListFiltersFixture(t *testing.T, app core.App) *core.Record {
	t.Helper()
	seedRowFull(t, app, "1", "Daily one", statusQueued, defaultQueue, findingOverdueReview, []string{testEditorNameAlex})
	seedRowFull(t, app, "2", "Daily two", statusQueued, defaultQueue, "normal", []string{testEditorNameBlake})
	seedRowFull(t, app, "3", "Event one", statusQueued, defaultQueue, "normal", []string{testEditorNameAlex})
	seedRowFull(t, app, "4", "Archive", statusPublished, queueArchived, "normal", []string{"Chris"})
	conference := saveConference(t, app, "Issue")
	saveTracking(t, app, "3", conference.Id)
	// saveTracking's hook recomputes effective_queue from findings, and this
	// fixture-only test never seeds any: submission 3 lands on "up_to_date"
	// (see queueFor), unmoved by the conference it now carries.
	return conference
}

func assertQueueAttentionAndStatusFilters(t *testing.T, count func(string) int, conferenceID string) {
	t.Helper()
	if count("") != 2 {
		t.Fatalf("default daily=%d", count(""))
	}
	if count("queue=active") != 3 || count("queue=archived") != 1 || count("queue=up_to_date") != 1 {
		t.Fatalf("queues daily=%d active=%d archived=%d up_to_date=%d", count(""), count("queue=active"), count("queue=archived"), count("queue=up_to_date"))
	}
	if count("requiresAttention=true") != 1 {
		t.Fatalf("requiresAttention=%d", count("requiresAttention=true"))
	}
	if count("attention=overdue_review") != 1 || count("queue=archived&status=Published") != 1 {
		t.Fatalf("attention/status=%d/%d", count("attention=overdue_review"), count("queue=archived&status=Published"))
	}
	if count("queue=active&editor=Alex") != 2 || count("editor=Alex") != 1 || count("queue=active&editor=alex") != 0 {
		t.Fatalf("editor filter active=%d daily=%d lowercase=%d", count("queue=active&editor=Alex"), count("editor=Alex"), count("queue=active&editor=alex"))
	}
	if count("queue=all&conference="+conferenceID) != 1 {
		t.Fatalf("conference filter=%d", count("queue=all&conference="+conferenceID))
	}
}

func assertFreeTextAndEditorFilters(t *testing.T, app core.App, count func(string) int) {
	t.Helper()
	// Free text reads the submission id and the title, and nothing else
	// (ADR-0014). "two" is in submission 2's title; "issue" is only a
	// conference name and was never searchable.
	if count("query=two") != 1 || count("query=issue") != 0 || count("query=nomatch") != 0 {
		t.Fatalf("query filters=%d/%d/%d", count("query=two"), count("query=issue"), count("query=nomatch"))
	}
	// An assigned editor's name is no longer free text. The editor: chip is the
	// way to ask that question, and it slug-merges colliding names besides.
	if count("query=blake") != 0 {
		t.Fatalf("free text must not match an assigned editor: %d", count("query=blake"))
	}
	if count(`query=editor%3A%22Blake%22`) != 1 {
		t.Fatalf("the editor: chip must still find Blake: %d", count(`query=editor%3A%22Blake%22`))
	}
	// The journal is curation, not the submission. This capability has no
	// replacement (ADR-0014).
	saveJournalEntry(t, app, "2", time.Now().UTC(), "operator", "Editora", "special-observation-marker")
	if count("query=special-observation-marker") != 0 {
		t.Fatalf("free text must not match a journal body: %d", count("query=special-observation-marker"))
	}
}

func TestListDropsProvenanceButSingleSubmissionKeepsIt(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: testSubmissionTitle, Status: statusQueued}}}, time.Now().UTC())

	if listBody := rawGet(t, mux, "/api/jserd/monitoring/submissions", token); strings.Contains(listBody, "provenance") {
		t.Fatalf("list response must not carry provenance: %s", listBody)
	}
	if singleBody := rawGet(t, mux, "/api/jserd/monitoring/submissions/42", token); !strings.Contains(singleBody, "provenance") {
		t.Fatalf("single submission response must keep provenance: %s", singleBody)
	}
}

// listPageQueryCount seeds totalSubmissions rows of submission_tracking and a
// single editorial run whose findings cover every one of them, then counts
// how many SQL queries a 10-row page listing issues.
func listPageQueryCount(t *testing.T, totalSubmissions int) int {
	t.Helper()
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	cycle := saveSnapshot(t, app, normalizer.NormalizedCycle{}, time.Now().UTC())
	findings := make([]editorial.ArticleFinding, 0, totalSubmissions)
	for i := 0; i < totalSubmissions; i++ {
		id := fmt.Sprintf("s%03d", i)
		seedRow(t, app, id, "Title "+id)
		saveTracking(t, app, id, "")
		findings = append(findings, editorial.ArticleFinding{SubmissionID: id, Findings: []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}})
	}
	if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, findings); err != nil {
		t.Fatal(err)
	}
	if err := OnFindingsReady(app, cycle.Id); err != nil {
		t.Fatal(err)
	}

	rawDB, ok := app.ConcurrentDB().(*dbx.DB)
	if !ok {
		t.Fatalf("expected *dbx.DB, got %T", app.ConcurrentDB())
	}
	queries := 0
	rawDB.QueryLogFunc = func(_ context.Context, _ time.Duration, _ string, _ *sql.Rows, _ error) {
		queries++
	}
	defer func() { rawDB.QueryLogFunc = nil }()

	page := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?limit=10", token)
	if len(page.Items) != 10 || page.Total != totalSubmissions {
		t.Fatalf("page=%#v", page)
	}
	return queries
}

func TestListSubmissionsQueryCountIsBoundedByPageNotByTableSize(t *testing.T) {
	small := listPageQueryCount(t, 60)
	large := listPageQueryCount(t, 240)
	// The query count is fixed per request (auth, the list query, the count
	// query, latestSuccess, and one scoped lookup each for submission_tracking
	// and editorial_findings): it must not grow when submission_tracking and
	// editorial_findings grow 4x.
	if small != large {
		t.Fatalf("query count grew with table size: 60 rows=%d queries, 240 rows=%d queries", small, large)
	}
	if small == 0 || small > 12 {
		t.Fatalf("unexpected query count for a single page listing: %d", small)
	}
}

func TestAggregatesCoverMetricsStatusesEditorsAndTriage(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	// Aggregates are only served once a succeeded cycle exists; the hook's
	// rebuild clears the table, so handcrafted rows come after the snapshot.
	saveSnapshot(t, app, normalizer.NormalizedCycle{}, time.Now().UTC())
	seedRowFull(t, app, "1", "One", statusQueued, defaultQueue, findingOverdueReview, []string{testEditorNameAlex})
	seedRowFull(t, app, "2", "Two", statusPublished, defaultQueue, "normal", []string{testEditorNameAlex})
	seedRowFull(t, app, "3", "Three", statusQueued, queueMuted, "missing_editor", []string{})
	meta := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", token)
	if meta.Aggregates.Triage[defaultQueue] != 2 || meta.Aggregates.Triage[queueMuted] != 1 || meta.Aggregates.Triage[queueArchived] != 0 || meta.Aggregates.DailyRequiringAttention != 1 {
		t.Fatalf("triage=%#v meta=%s", meta.Aggregates.Triage, rawGet(t, mux, "/api/jserd/monitoring", token))
	}
	if meta.Aggregates.Metrics[findingOverdueReview] != 1 || meta.Aggregates.Metrics["normal"] != 1 {
		t.Fatalf("metrics=%#v", meta.Aggregates.Metrics)
	}
	if !equalStrings(meta.Aggregates.Statuses, []string{statusPublished, statusQueued}) {
		t.Fatalf("statuses=%#v", meta.Aggregates.Statuses)
	}
	if len(meta.Aggregates.Editors) != 1 || meta.Aggregates.Editors[0] != (EditorFrequency{Slug: "alex", Label: testEditorNameAlex, Count: 2}) {
		t.Fatalf("editors=%#v", meta.Aggregates.Editors)
	}
}

type metaResponse struct {
	Availability  State                `json:"availability"`
	Conferences   []Conference         `json:"conferences"`
	LatestAttempt *struct{ ID string } `json:"latestAttempt"`
	LatestSuccess *struct{ ID string } `json:"latestSuccess"`
	Failure       string               `json:"failure,omitempty"`
	Aggregates    struct {
		Metrics                 map[string]int    `json:"metrics"`
		Triage                  map[string]int    `json:"triage"`
		DailyRequiringAttention int               `json:"dailyRequiringAttention"`
		Statuses                []string          `json:"statuses"`
		Editors                 []EditorFrequency `json:"editors"`
		Total                   int               `json:"total"`
	} `json:"aggregates"`
}

type submissionResponse struct {
	OJSSubmissionID     string         `json:"ojsSubmissionId"`
	URL                 string         `json:"url"`
	DateSubmitted       string         `json:"dateSubmitted"`
	EffectiveQueue      string         `json:"effectiveQueue"`
	EffectiveFinalState string         `json:"effectiveFinalState"`
	EffectiveAttention  string         `json:"effectiveAttention"`
	Findings            []Finding      `json:"findings"`
	Tracking            *LocalTracking `json:"tracking"`
}

type submissionPage struct {
	Items      []submissionResponse `json:"items"`
	NextCursor string               `json:"nextCursor"`
	Total      int                  `json:"total"`
}

func getJSON[T any](t *testing.T, mux http.Handler, path string, token string) T {
	t.Helper()
	response := apiCall(mux, http.MethodGet, path, nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, response.Code, response.Body.String())
	}
	var decoded T
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func rawGet(t *testing.T, mux http.Handler, path string, token string) string {
	t.Helper()
	response := apiCall(mux, http.MethodGet, path, nil, token)
	return response.Body.String()
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func monitoringTestApp(t *testing.T) *tests.TestApp {
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
	Register(app)
	return app
}

func serveMux(t *testing.T, app *tests.TestApp) http.Handler {
	t.Helper()
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	event := &core.ServeEvent{App: app, Router: router}
	if err := app.OnServe().Trigger(event, func(*core.ServeEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	mux, err := event.Router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return mux
}

func superuserToken(t *testing.T, app *tests.TestApp) string {
	t.Helper()
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	operator := core.NewRecord(superusers)
	operator.SetEmail("operator@example.test")
	operator.SetPassword("valid-password")
	if err := app.Save(operator); err != nil {
		t.Fatal(err)
	}
	token, err := operator.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func chiefEditorToken(t *testing.T, app *tests.TestApp) string {
	t.Helper()
	chiefEditors, err := app.FindCollectionByNameOrId(collections.ChiefEditors)
	if err != nil {
		t.Fatal(err)
	}
	editor := core.NewRecord(chiefEditors)
	editor.SetEmail("operator@example.test")
	editor.SetPassword("valid-password")
	if err := app.Save(editor); err != nil {
		t.Fatal(err)
	}
	token, err := editor.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func apiCall(mux http.Handler, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", token)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func seedRow(t *testing.T, app core.App, id, title string) {
	t.Helper()
	seedRowFull(t, app, id, title, statusQueued, defaultQueue, "normal", []string{})
}

func seedRowFull(t *testing.T, app core.App, ojsSubmissionID, title, status, queue, attention string, editors []string) {
	t.Helper()
	submissions, err := app.FindCollectionByNameOrId(collections.OJSSubmissions)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := json.Marshal(Evidence{Editors: editors, Decisions: []Decision{}, Reviews: []Review{}, Provenance: Provenance{}})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(evidence, &decoded); err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(submissions)
	record.Set("ojs_submission_id", ojsSubmissionID)
	record.Set("title", title)
	record.Set(statusFieldName, status)
	record.Set("effective_queue", queue)
	record.Set("effective_attention", attention)
	record.Set("cycle", "cycle-"+ojsSubmissionID)
	record.Set("evidence", decoded)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
}

func saveCycle(t *testing.T, app core.App, status string, finished time.Time, failure string) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(collections.CollectionCycles)
	if err != nil {
		t.Fatal(err)
	}
	cycleRecord := core.NewRecord(c)
	cycleRecord.Set("source_mode", sourceModeFixtures)
	cycleRecord.Set(statusFieldName, status)
	cycleRecord.Set("started_at", finished.Add(-time.Minute))
	cycleRecord.Set("finished_at", finished)
	cycleRecord.Set("manifest", map[string]any{"source_mode": sourceModeFixtures})
	cycleRecord.Set("error", failure)
	if err := app.Save(cycleRecord); err != nil {
		t.Fatal(err)
	}
	return cycleRecord
}

func saveSnapshot(t *testing.T, app core.App, normalized normalizer.NormalizedCycle, finished time.Time) *core.Record {
	t.Helper()
	result := collector.CycleResult{SourceMode: config.SourceModeFixtures, StartedAt: finished.Add(-time.Minute), FinishedAt: finished, Status: collector.CycleStatusOK, Manifest: audit.Manifest{SourceMode: sourceModeFixtures, CollectedAt: finished}, Normalized: normalized}
	if err := state.NewStore(app).ProcessCycle(result); err != nil {
		t.Fatal(err)
	}
	cycles, err := app.FindRecordsByFilter(collections.CollectionCycles, "status = 'succeeded'", "-finished_at", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	return cycles[0]
}

func saveConference(t *testing.T, app core.App, name string) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(collections.Conferences)
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	r.Set("name", name)
	if err := app.Save(r); err != nil {
		t.Fatal(err)
	}
	return r
}

func saveTracking(t *testing.T, app core.App, submissionID, conference string) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collections.SubmissionTracking)
	if err != nil {
		t.Fatal(err)
	}
	existing, _ := app.FindFirstRecordByData(collections.SubmissionTracking, "ojs_submission_id", submissionID)
	record := existing
	if record == nil {
		record = core.NewRecord(collection)
	}
	record.Set("ojs_submission_id", submissionID)
	record.Set("conference", conference)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func saveEditorialRun(t *testing.T, app core.App, cycleID, submissionID string, findings []editorial.Finding) {
	t.Helper()
	if err := editorial.NewStore(app).ReplaceFindings(cycleID, []editorial.ArticleFinding{{SubmissionID: submissionID, Findings: findings}}); err != nil {
		t.Fatal(err)
	}
	if err := OnFindingsReady(app, cycleID); err != nil {
		t.Fatal(err)
	}
}

// badRequestOrInternal serves two routes. It used to answer every server-side
// failure with "Unable to list submissions", which sent an operator reading the
// /aggregates/editors answer to ListSubmissions, a function not on that path.
func TestBadRequestOrInternalNamesTheOperationThatFailed(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		operation string
		wantCode  int
		// PocketBase capitalises the message and adds a full stop, so these are
		// the words an operator actually reads.
		wantMsg string
	}{
		{"a server failure on the list route", errors.New("boom"), "Unable to list submissions", http.StatusInternalServerError, "Unable to list submissions."},
		{"a server failure on the editor-frequency route", errors.New("boom"), "Unable to load editor frequencies", http.StatusInternalServerError, "Unable to load editor frequencies."},
		{"a client mistake keeps its own message", clientError{"unknown folder: nope"}, "Unable to list submissions", http.StatusBadRequest, "Unknown folder: nope."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := badRequestOrInternal(&core.RequestEvent{}, testCase.err, testCase.operation)
			var apiErr *router.ApiError
			if !errors.As(err, &apiErr) {
				t.Fatalf("got %v, want an API error", err)
			}
			if apiErr.Status != testCase.wantCode || apiErr.Message != testCase.wantMsg {
				t.Fatalf("status=%d message=%q, want %d %q", apiErr.Status, apiErr.Message, testCase.wantCode, testCase.wantMsg)
			}
		})
	}
}
