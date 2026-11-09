package monitoring

import (
	"net/http"
	"server/internal/collections"
	"server/internal/editorial"
	"server/internal/normalizer"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestParseSearchQuerySplitsEditorChipsFolderAndFreeText(t *testing.T) {
	for _, testCase := range []struct {
		raw    string
		chips  []string
		folder string
		free   string
	}{
		{"", []string{}, "", ""},
		{testCodeSmell, []string{}, "", "code smell"},
		{`editor:"Casey Lee" code smell editor:"Morgan Reed"`, []string{"Casey Lee", "Morgan Reed"}, "", "code smell"},
		{"editor:Casey", []string{testChiefCasey}, "", ""},
		{"  editor:Casey   600440  ", []string{testChiefCasey}, "", "600440"},
		// parseSearchQuery passes the slug through and only rejects an unknown
		// one. The work/worklist/daily alias is resolved by parseScope in
		// queue.go, so it is no longer applied twice on the way in.
		{"folder:work", []string{}, worklistKindWork, ""},
		{"folder:up_to_date", []string{}, queueUpToDate, ""},
		{"folder:muted", []string{}, queueMuted, ""},
		{"folder:archived", []string{}, queueArchived, ""},
		{`editor:"Casey" folder:muted overdue`, []string{testChiefCasey}, queueMuted, "overdue"},
	} {
		chips, folder, free, err := parseSearchQuery(testCase.raw)
		if err != nil {
			t.Fatalf("parseSearchQuery(%q) unexpected error: %v", testCase.raw, err)
		}
		if !equalStrings(chips, testCase.chips) || folder != testCase.folder || free != testCase.free {
			t.Fatalf("parseSearchQuery(%q) = %#v, %q, %q; want %#v, %q, %q", testCase.raw, chips, folder, free, testCase.chips, testCase.folder, testCase.free)
		}
	}
}

func TestParseSearchQueryRejectsUnknownFolder(t *testing.T) {
	if _, _, _, err := parseSearchQuery("folder:nope"); err == nil {
		t.Fatal("expected an error for an unrecognized folder: value")
	}
}

// seedWorkList projects five submissions and one completed editorial run:
// two rank 1, one rank 2, one muted and one with no finding at all.
func seedWorkList(t *testing.T, app core.App) {
	t.Helper()
	casey := normalizer.Editor{GivenName: "Casey", FamilyName: "Lee"}
	jordan := normalizer.Editor{GivenName: "Jordan", FamilyName: "Reed"}
	cycle := saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: rankRecent1, Title: "Recent rank one", Status: statusQueued, DateSubmitted: "2026-01-10", Editors: []normalizer.Editor{casey, jordan}},
		{OJSSubmissionID: rankOldest1, Title: "Oldest rank one", Status: statusQueued, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{casey}},
		{OJSSubmissionID: rankAncient2, Title: "Ancient rank two", Status: statusQueued, DateSubmitted: "2020-01-01", Editors: []normalizer.Editor{jordan}},
		{OJSSubmissionID: rankMutedOne, Title: "Muted submission", Status: statusQueued, DateSubmitted: testDate20260101},
		{OJSSubmissionID: "settled-one", Title: "Nothing to do", Status: statusQueued, DateSubmitted: testDate20260101},
	}}, time.Now().UTC())

	findings := []editorial.ArticleFinding{
		{SubmissionID: rankRecent1, Findings: []editorial.Finding{{Kind: editorial.FindingMissingEditor}}},
		{SubmissionID: rankOldest1, Findings: []editorial.Finding{{Kind: editorial.FindingMissingEditor}}},
		{SubmissionID: rankAncient2, Findings: []editorial.Finding{{Kind: editorial.FindingOverdueReview, Deadline: "2020-02-01"}}},
		{SubmissionID: rankMutedOne, Findings: []editorial.Finding{{Kind: editorial.FindingMissingReviewer}}},
	}
	if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, findings); err != nil {
		t.Fatal(err)
	}
	if err := OnFindingsReady(app, cycle.Id); err != nil {
		t.Fatal(err)
	}

	collection, err := app.FindCollectionByNameOrId(collections.SubmissionTracking)
	if err != nil {
		t.Fatal(err)
	}
	tracking := core.NewRecord(collection)
	tracking.Set("ojs_submission_id", rankMutedOne)
	tracking.Set(queueMuted, true)
	if err := app.Save(tracking); err != nil {
		t.Fatal(err)
	}
}

func workListIDs(t *testing.T, mux http.Handler, token, path string) []string {
	t.Helper()
	page := getJSON[submissionPage](t, mux, path, token)
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.OJSSubmissionID)
	}
	return ids
}

// The work list is one list, worst first: rank 1 above rank 2 whatever the
// elapsed time, and inside a rank the oldest submission first.
func TestWorkListHoldsOneRowPerSubmissionOrderedByPriorityThenElapsedDays(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)

	ids := workListIDs(t, serveMux(t, app), chiefEditorToken(t, app), "/api/jserd/monitoring/submissions?queue=worklist&sort=priority&limit=10")
	want := []string{rankOldest1, rankRecent1, rankAncient2}
	if !equalStrings(ids, want) {
		t.Fatalf("work list = %#v, want %#v", ids, want)
	}
}

// An empty dateSubmitted must not read as the oldest submission there is. The
// sort key coalesces a missing date to ”, and ” compares below every real
// YYYY-MM-DD, so an unknown date used to head its rank instead of trailing it.
func TestWorkListPutsAMissingDateSubmittedLastInItsRank(t *testing.T) {
	app := monitoringTestApp(t)
	casey := normalizer.Editor{GivenName: "Casey", FamilyName: "Lee"}
	cycle := saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: knownDateKey, Title: "Known date", Status: statusQueued, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{casey}},
		{OJSSubmissionID: unknownDateKey, Title: "Unknown date", Status: statusQueued, DateSubmitted: "", Editors: []normalizer.Editor{casey}},
	}}, time.Now().UTC())

	findings := []editorial.ArticleFinding{
		{SubmissionID: knownDateKey, Findings: []editorial.Finding{{Kind: editorial.FindingMissingEditor}}},
		{SubmissionID: unknownDateKey, Findings: []editorial.Finding{{Kind: editorial.FindingMissingEditor}}},
	}
	if err := editorial.NewStore(app).ReplaceFindings(cycle.Id, findings); err != nil {
		t.Fatal(err)
	}
	if err := OnFindingsReady(app, cycle.Id); err != nil {
		t.Fatal(err)
	}

	ids := workListIDs(t, serveMux(t, app), chiefEditorToken(t, app), "/api/jserd/monitoring/submissions?queue=worklist&sort=priority&limit=10")
	want := []string{knownDateKey, unknownDateKey}
	if !equalStrings(ids, want) {
		t.Fatalf("work list = %#v, want %#v", ids, want)
	}
}

func TestWorkListPaginatesInPriorityOrder(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	first := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?queue=worklist&sort=priority&limit=2", token)
	if len(first.Items) != 2 || first.NextCursor == "" || first.Total != 3 {
		t.Fatalf("first batch=%#v", first)
	}
	second := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=worklist&sort=priority&limit=2&cursor="+first.NextCursor)
	if !equalStrings(second, []string{rankAncient2}) {
		t.Fatalf("second batch=%#v", second)
	}
}

func TestSearchWireFormCombinesEditorChipsWithAnd(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	both := workListIDs(t, mux, token, `/api/jserd/monitoring/submissions?queue=worklist&sort=priority&query=editor%3A%22Casey+Lee%22+editor%3A%22Jordan+Reed%22`)
	if !equalStrings(both, []string{rankRecent1}) {
		t.Fatalf("two chips must intersect, got %#v", both)
	}
	one := workListIDs(t, mux, token, `/api/jserd/monitoring/submissions?queue=worklist&sort=priority&query=editor%3A%22Casey+Lee%22`)
	if !equalStrings(one, []string{rankOldest1, rankRecent1}) {
		t.Fatalf("one chip=%#v", one)
	}
	mixed := workListIDs(t, mux, token, `/api/jserd/monitoring/submissions?queue=worklist&sort=priority&query=editor%3A%22Casey+Lee%22+oldest`)
	if !equalStrings(mixed, []string{rankOldest1}) {
		t.Fatalf("chip and free text=%#v", mixed)
	}
}

// Two assigned editors who share a display name slug (ADR-0006) must both
// come back for the same editor: chip, since the search combobox's merged
// suggestion offers no way to pick one over the other.
func TestEditorChipMatchesEveryRawNameCollidingOnItsSlug(t *testing.T) {
	app := monitoringTestApp(t)
	seedRowFull(t, app, "1", "One", statusQueued, defaultQueue, "normal", []string{reviewerAndersonS})
	seedRowFull(t, app, "2", "Two", statusQueued, queueUpToDate, "normal", []string{"anderson s."})
	seedRowFull(t, app, "3", "Three", statusQueued, defaultQueue, "normal", []string{testEditorNameBlake})
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	ids := workListIDs(t, mux, token, `/api/jserd/monitoring/submissions?queue=all&sort=id&query=editor%3A%22Anderson+S.%22`)
	if !equalStrings(ids, []string{"1", "2"}) {
		t.Fatalf("chip must match every raw name colliding on the slug: %#v", ids)
	}
}

// queue=all (and its "" spelling) is the all-folders search mode: the client
// signals it explicitly, distinct from an absent queue param, which keeps the
// existing daily default.
func TestQueueAllReturnsRowsFromEveryQueue(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRowFull(t, app, "1", "Daily", statusQueued, defaultQueue, "normal", []string{})
	seedRowFull(t, app, "2", "Up to date", statusQueued, queueUpToDate, "normal", []string{})
	seedRowFull(t, app, "3", "Archived", statusPublished, queueArchived, "normal", []string{})
	seedRowFull(t, app, "4", "Muted", statusQueued, queueMuted, "normal", []string{})

	// The all-queue list is grouped by status first (Daily, Up to date, Muted,
	// Archived), sort=id only breaks ties inside a group.
	all := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=all&sort=id&limit=10")
	if !equalStrings(all, []string{"1", "2", "4", "3"}) {
		t.Fatalf("queue=all = %#v", all)
	}
	empty := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=&sort=id&limit=10")
	if !equalStrings(empty, []string{"1", "2", "4", "3"}) {
		t.Fatalf("queue= = %#v", empty)
	}
	stillDaily := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?sort=id&limit=10")
	if !equalStrings(stillDaily, []string{"1"}) {
		t.Fatalf("an absent queue param must keep the daily default: %#v", stillDaily)
	}
}

// The Daily/Up to date/Muted/Archived group order always wins over the
// requested column, in either sort direction: only the tie-break inside a
// group follows the column's own direction.
func TestQueueAllOrdersByStatusRegardlessOfSortDirection(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRowFull(t, app, "1", "B", statusQueued, defaultQueue, "normal", []string{})
	seedRowFull(t, app, "2", "A", statusQueued, defaultQueue, "normal", []string{})
	seedRowFull(t, app, "3", "Z", statusPublished, queueArchived, "normal", []string{})

	desc := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=all&sort=id&dir=desc&limit=10")
	if !equalStrings(desc, []string{"2", "1", "3"}) {
		t.Fatalf("queue=all&dir=desc = %#v, want the daily group first, id-descending within it", desc)
	}
}

func TestQueueAllKeysetPaginatesAcrossQueues(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRowFull(t, app, "1", "A", statusQueued, defaultQueue, "normal", []string{})
	seedRowFull(t, app, "2", "B", statusQueued, queueUpToDate, "normal", []string{})
	seedRowFull(t, app, "3", "C", statusPublished, queueArchived, "normal", []string{})

	first := getJSON[submissionPage](t, mux, "/api/jserd/monitoring/submissions?queue=all&sort=id&limit=2", token)
	if len(first.Items) != 2 || first.NextCursor == "" || first.Total != 3 {
		t.Fatalf("first=%#v", first)
	}
	second := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=all&sort=id&limit=2&cursor="+first.NextCursor)
	if !equalStrings(second, []string{"3"}) {
		t.Fatalf("second=%#v", second)
	}
}

// A folder: token in the query text sets the queue scope and wins over the
// queue param, which the client keeps sending from the route.
func TestFolderTokenSetsQueueScopeAndOverridesTheQueueParam(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRowFull(t, app, "1", "Daily", statusQueued, defaultQueue, "normal", []string{})
	seedRowFull(t, app, "2", "Also daily", statusQueued, defaultQueue, "normal", []string{})
	seedRowFull(t, app, "3", "Muted", statusQueued, queueMuted, "normal", []string{})

	work := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=all&sort=id&query=folder%3Awork")
	if !equalStrings(work, []string{"1", "2"}) {
		t.Fatalf("folder:work = %#v", work)
	}
	muted := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=daily&sort=id&query=folder%3Amuted")
	if !equalStrings(muted, []string{"3"}) {
		t.Fatalf("folder:muted overriding queue=daily = %#v", muted)
	}
}

func TestFolderTokenAndEditorChipsParseTogether(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	ids := workListIDs(t, mux, token, `/api/jserd/monitoring/submissions?queue=all&sort=priority&query=editor%3A%22Casey+Lee%22+folder%3Awork`)
	if !equalStrings(ids, []string{rankOldest1, rankRecent1}) {
		t.Fatalf("editor+folder = %#v", ids)
	}
}

func TestUnknownFolderTokenIsRejected(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/submissions?query=folder%3Anope", nil, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown folder = %d %s", response.Code, response.Body.String())
	}
}

func TestEditorFrequencyMergesCollidingNamesAndSupportsScopes(t *testing.T) {
	app := monitoringTestApp(t)
	seedRowFull(t, app, "1", "One", statusQueued, defaultQueue, "normal", []string{reviewerAndersonS})
	seedRowFull(t, app, "2", "Two", statusQueued, queueUpToDate, "normal", []string{"anderson s.", testEditorNameBlake})
	seedRowFull(t, app, "3", "Sunny day", statusQueued, defaultQueue, "normal", []string{testEditorNameBlake})

	global, err := loadEditorFrequencies(app, editorFrequencyScope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(global) != 2 {
		t.Fatalf("global=%#v", global)
	}
	var anderson EditorFrequency
	for _, entry := range global {
		if entry.Slug == reviewerAndersonSSlug {
			anderson = entry
		}
	}
	if anderson.Count != 2 || anderson.Label != reviewerAndersonS {
		t.Fatalf("colliding names must merge into one deterministic label: %#v", anderson)
	}

	scoped, err := loadEditorFrequencies(app, editorFrequencyScope{queue: queueUpToDate})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 2 {
		t.Fatalf("queue-scoped=%#v", scoped)
	}

	restricted, err := loadEditorFrequencies(app, editorFrequencyScope{text: "sunny"})
	if err != nil {
		t.Fatal(err)
	}
	if len(restricted) != 1 || restricted[0].Slug != reviewerBlakeSlug {
		t.Fatalf("text-restricted=%#v", restricted)
	}
}

func TestEditorFrequencyEndpointScopesByFolderAndFallsBackWhenEmpty(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	seedRowFull(t, app, "1", "One", statusQueued, defaultQueue, "normal", []string{reviewerAndersonS})
	seedRowFull(t, app, "2", "Two", statusQueued, queueMuted, "normal", []string{testEditorNameBlake})

	global := getJSON[[]EditorFrequency](t, mux, "/api/jserd/monitoring/aggregates/editors", token)
	if !equalStrings(slugsOf(global), []string{reviewerAndersonSSlug, reviewerBlakeSlug}) {
		t.Fatalf("global=%#v", global)
	}

	// `folder:work` (client vocabulary) must select the same queue as a
	// `folder:` token in the `query` wire format. parseScope resolves both.
	work := getJSON[[]EditorFrequency](t, mux, "/api/jserd/monitoring/aggregates/editors?folder=work", token)
	if !equalStrings(slugsOf(work), []string{reviewerAndersonSSlug}) {
		t.Fatalf("folder=work=%#v", work)
	}

	// A folder+text combination that names no editor falls back to the
	// unrestricted global ranking (spec.md rule 3), instead of an empty list.
	fallback := getJSON[[]EditorFrequency](t, mux, "/api/jserd/monitoring/aggregates/editors?folder=work&text=nomatch", token)
	if !equalStrings(slugsOf(fallback), []string{reviewerAndersonSSlug, reviewerBlakeSlug}) {
		t.Fatalf("fallback=%#v", fallback)
	}

	response := apiCall(mux, http.MethodGet, "/api/jserd/monitoring/aggregates/editors?folder=nope", nil, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown folder = %d %s", response.Code, response.Body.String())
	}
}

func slugsOf(frequencies []EditorFrequency) []string {
	slugs := make([]string, len(frequencies))
	for i, entry := range frequencies {
		slugs[i] = entry.Slug
	}
	return slugs
}

func TestWorkListResponseCarriesTheFindingDigestAndTheSideScreenCounts(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	page := getJSON[struct {
		Items []struct {
			OJSSubmissionID string `json:"ojsSubmissionId"`
			FindingDigest   string `json:"findingDigest"`
		} `json:"items"`
	}](t, mux, "/api/jserd/monitoring/submissions?queue=worklist&sort=priority", token)
	for _, item := range page.Items {
		if item.FindingDigest == "" {
			t.Fatalf("%s carries no finding digest", item.OJSSubmissionID)
		}
	}

	meta := getJSON[metaResponse](t, mux, "/api/jserd/monitoring", token)
	if meta.Aggregates.Triage[queueMuted] != 1 || meta.Aggregates.Triage[queueUpToDate] != 1 {
		t.Fatalf("side screen counts=%#v", meta.Aggregates.Triage)
	}
}

// Archived carries no actionable finding rank, so it orders by OJS's own
// last-activity timestamp instead, newest first. A submission OJS never
// reported activity for sorts last, not first, under that direction.
func TestArchivedOrdersByNewestActivityFirst(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	saveSnapshot(t, app, normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: "stale", Title: "Stale", Status: statusPublished, DateLastActivity: testDate20260101},
		{OJSSubmissionID: "recent", Title: "Recent", Status: statusPublished, DateLastActivity: testDate20260301},
		{OJSSubmissionID: testSubmissionIDUnknown, Title: "Unknown activity", Status: statusPublished},
	}}, time.Now().UTC())

	ids := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=archived&sort=activity&dir=desc&limit=10")
	if !equalStrings(ids, []string{"recent", "stale", testSubmissionIDUnknown}) {
		t.Fatalf("archived order = %#v", ids)
	}
}

// The two side screens are their own lists, so the counts in the work list
// footer have somewhere to lead. `queue=worklist` deliberately excludes both.
func TestSideScreenQueuesListTheirOwnSubmissions(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	muted := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=muted&limit=10")
	if !equalStrings(muted, []string{rankMutedOne}) {
		t.Fatalf("queue=muted returned %#v", muted)
	}
	upToDate := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=up_to_date&limit=10")
	if !equalStrings(upToDate, []string{"settled-one"}) {
		t.Fatalf("queue=up_to_date returned %#v", upToDate)
	}
}

// The Muted screen needs the stamp to mark a finding that appeared after the
// mute, so the stamp has to survive the trip to the client.
func TestMutedListCarriesTheMutedDigest(t *testing.T) {
	app := monitoringTestApp(t)
	seedWorkList(t, app)

	page := getJSON[struct {
		Items []struct {
			FindingDigest string `json:"findingDigest"`
			Tracking      struct {
				MutedDigest string `json:"mutedDigest"`
			} `json:"tracking"`
		} `json:"items"`
	}](t, serveMux(t, app), "/api/jserd/monitoring/submissions?queue=muted&limit=10", chiefEditorToken(t, app))
	if len(page.Items) != 1 {
		t.Fatalf("muted page=%#v", page.Items)
	}
	if page.Items[0].Tracking.MutedDigest != page.Items[0].FindingDigest {
		t.Fatalf("mutedDigest=%q findingDigest=%q; a mute with unchanged findings must match", page.Items[0].Tracking.MutedDigest, page.Items[0].FindingDigest)
	}
}
