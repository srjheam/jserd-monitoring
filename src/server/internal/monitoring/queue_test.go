package monitoring

import "testing"

// The list and the editor-frequency ranking must read a queue scope the same
// way. Before the queue module they did not: buildFilters knew the "active"
// scope and loadEditorFrequencies did not, so scoping the suggestions to
// active silently returned the unscoped ranking, including archived editors.
func TestActiveScopeExcludesArchivedEverywhere(t *testing.T) {
	app := monitoringTestApp(t)
	seedRowFull(t, app, "1", "One", statusQueued, defaultQueue, "normal", []string{"Working Editor"})
	seedRowFull(t, app, "2", "Two", statusQueued, queueArchived, "normal", []string{"Closed Editor"})

	editors, err := loadEditorFrequencies(app, editorFrequencyScope{queue: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if len(editors) != 1 || editors[0].Slug != "working-editor" {
		t.Fatalf("active scope = %#v, want only working-editor: an archived submission's editor must not rank", editors)
	}
}

func TestParseScopeRejectsAnUnknownSlug(t *testing.T) {
	if _, ok := parseScope("nope"); ok {
		t.Fatal("parseScope accepted an unknown slug")
	}
	for _, slug := range []string{"all", "active", worklistKindWork, "worklist", defaultQueue, queueUpToDate, queueMuted, queueArchived} {
		if _, ok := parseScope(slug); !ok {
			t.Fatalf("parseScope rejected %q", slug)
		}
	}
}

// work, worklist and daily are three spellings of one queue. The alias lives in
// the queue module alone, so all three must produce the same predicate.
func TestWorkAliasesAgree(t *testing.T) {
	seen := make([]string, 0, 3)
	for _, slug := range []string{worklistKindWork, "worklist", defaultQueue} {
		scope, ok := parseScope(slug)
		if !ok {
			t.Fatalf("parseScope(%q) failed", slug)
		}
		sql, _ := scope.predicate()
		seen = append(seen, sql)
	}
	if seen[0] != seen[1] || seen[1] != seen[2] {
		t.Fatalf("work/worklist/daily disagree: %#v", seen)
	}
}

// An unrestricted scope adds no condition at all, so a caller can skip it.
func TestAllScopeHasNoPredicate(t *testing.T) {
	scope, ok := parseScope("all")
	if !ok {
		t.Fatal("parseScope(all) failed")
	}
	if sql, _ := scope.predicate(); sql != "" {
		t.Fatalf("all scope predicate = %q, want empty", sql)
	}
}

// queueNames drives the display order and the aggregate keys together, so a
// queue added to that one list reaches both without a second edit.
func TestQueueNamesDriveTheDisplayOrder(t *testing.T) {
	names := queueNames()
	want := []string{defaultQueue, queueUpToDate, queueMuted, queueArchived}
	if len(names) != len(want) {
		t.Fatalf("queueNames() = %#v, want %#v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("queueNames()[%d] = %q, want %q", i, names[i], name)
		}
	}
	ascending := orderRank(false)
	for _, name := range names {
		if !contains(ascending, "'"+name+"'") {
			t.Fatalf("orderRank does not rank %q: %s", name, ascending)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0
}

func stringIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
