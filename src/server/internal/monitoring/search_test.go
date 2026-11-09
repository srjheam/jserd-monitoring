package monitoring

import (
	"net/http"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// seedFreeTextFixture gives one submission a distinctive title word, and puts
// every other searchable value on a second submission: an assigned editor, a
// verified_by and a journal entry. Free text must find the first and not the
// second (ADR-0014).
func seedFreeTextFixture(t *testing.T, app core.App) {
	t.Helper()
	seedRowFull(t, app, "1", "A paper about kestrels", statusQueued, defaultQueue, "normal", []string{"Riley Park"})
	seedRowFull(t, app, "2", "An unrelated paper", statusQueued, defaultQueue, "normal", []string{"Kestrel Watcher"})
	saveJournalEntry(t, app, "2", time.Now().UTC(), "op1", "Editora", "kestrels were discussed at length")
}

// The list and the ranking beside it must answer the same question. Before the
// shared predicate they did not: the list matched the editors, verified_by and
// the journal body as well, so typing an editor's name narrowed the list while
// the suggestions fell back to the global ranking.
func TestFreeTextScopesTheListAndTheRankingAlike(t *testing.T) {
	app := monitoringTestApp(t)
	seedFreeTextFixture(t, app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	ids := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=all&query=kestrels")
	if !equalStrings(ids, []string{"1"}) {
		t.Fatalf("list for \"kestrels\" = %#v, want only submission 1", ids)
	}

	editors, err := loadEditorFrequencies(app, editorFrequencyScope{text: "kestrels"})
	if err != nil {
		t.Fatal(err)
	}
	if len(editors) != 1 || editors[0].Slug != "riley-park" {
		t.Fatalf("ranking for \"kestrels\" = %#v, want only riley-park: it must scope the same way the list does", editors)
	}
}

// ADR-0014: an editor's name typed as free text is no longer a way to find
// their submissions. The editor: chip is, and it slug-merges besides.
func TestFreeTextDoesNotMatchAnAssignedEditor(t *testing.T) {
	app := monitoringTestApp(t)
	seedFreeTextFixture(t, app)

	ids := workListIDs(t, serveMux(t, app), chiefEditorToken(t, app), "/api/jserd/monitoring/submissions?queue=all&query=Lovelace")
	if len(ids) != 0 {
		t.Fatalf("free text matched an assigned editor: %#v", ids)
	}
}

// ADR-0014: the journal is curation, not the submission. This capability has
// no replacement.
func TestFreeTextDoesNotMatchTheJournalBody(t *testing.T) {
	app := monitoringTestApp(t)
	seedRowFull(t, app, "2", "An unrelated paper", statusQueued, defaultQueue, "normal", []string{"Someone Else"})
	saveJournalEntry(t, app, "2", time.Now().UTC(), "op1", "Editora", "kestrels were discussed at length")

	ids := workListIDs(t, serveMux(t, app), chiefEditorToken(t, app), "/api/jserd/monitoring/submissions?queue=all&query=kestrels")
	if len(ids) != 0 {
		t.Fatalf("free text matched a journal body: %#v", ids)
	}
}

// Several words match any submission that hits at least one of them, which is
// the list's long-standing behavior and is unchanged.
func TestFreeTextMatchesAnyWord(t *testing.T) {
	app := monitoringTestApp(t)
	seedRowFull(t, app, "1", "Kestrels in flight", statusQueued, defaultQueue, "normal", nil)
	seedRowFull(t, app, "2", "Falcons at rest", statusQueued, defaultQueue, "normal", nil)
	seedRowFull(t, app, "3", "Something else", statusQueued, defaultQueue, "normal", nil)

	ids := workListIDs(t, serveMux(t, app), chiefEditorToken(t, app), "/api/jserd/monitoring/submissions?queue=all&sort=id&query=kestrels+falcons")
	if !equalStrings(ids, []string{"1", "2"}) {
		t.Fatalf("any-word match = %#v, want 1 and 2", ids)
	}
}

// A LIKE wildcard typed by a chief editor is a literal character, not a
// pattern: "100%" must not match every title.
func TestFreeTextKeepsWildcardsLiteral(t *testing.T) {
	app := monitoringTestApp(t)
	seedRowFull(t, app, "1", "Reaching 100% coverage", statusQueued, defaultQueue, "normal", nil)
	seedRowFull(t, app, "2", "No numbers here", statusQueued, defaultQueue, "normal", nil)

	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)
	for _, needle := range []string{"100%25", "%25", "_"} {
		ids := workListIDs(t, mux, token, "/api/jserd/monitoring/submissions?queue=all&sort=id&query="+needle)
		if len(ids) > 1 {
			t.Fatalf("query %q matched %#v: wildcards must stay literal", needle, ids)
		}
	}
}

func TestUnknownQueueStillRejected(t *testing.T) {
	app := monitoringTestApp(t)
	response := apiCall(serveMux(t, app), http.MethodGet, "/api/jserd/monitoring/submissions?queue=nope", nil, chiefEditorToken(t, app))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown queue = %d", response.Code)
	}
}
