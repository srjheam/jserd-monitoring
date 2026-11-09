package monitoring

import (
	"fmt"
	"server/internal/normalizer"
	"sort"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// EditorFrequency is one assigned-editor slug's submission count within a
// scope (see loadEditorFrequencies), ranking candidates for the search
// combobox's editor: suggestions instead of the old alphabetical distinct
// list.
type EditorFrequency struct {
	Slug  string `json:"slug" db:"slug"`
	Label string `json:"label" db:"label"`
	Count int    `json:"count" db:"count"`
}

// editorFrequencyScope narrows loadEditorFrequencies to one of the three
// scopes spec.md's ranking fallback order composes: Queue alone (empty means
// every folder), Text alone, or both together.
type editorFrequencyScope struct {
	queue string
	text  string
}

// loadEditorFrequencies counts, for each assigned-editor slug (ADR-0006), how
// many matching submissions name that editor. Two raw display names that
// share a slug merge into one entry; its label is the alphabetically first
// raw form seen, which is deterministic across runs.
func loadEditorFrequencies(app core.App, scope editorFrequencyScope) ([]EditorFrequency, error) {
	names, err := scopedEditorNames(app, scope)
	if err != nil {
		return nil, err
	}
	return rankedFrequencies(names), nil
}

// scopedEditorNames lists every raw assigned-editor display name the scope
// matches, once for each submission that names it.
func scopedEditorNames(app core.App, scope editorFrequencyScope) ([]string, error) {
	// The same queue module the list reads, so a scope that narrows the list
	// narrows the ranking beside it the same way.
	filters := filterSet{params: dbx.Params{}}
	addScope(&filters, mustScope(scope.queue))
	// The same free-text predicate the list reads, so the words that narrow
	// the list are the words that scope this ranking (ADR-0014).
	addFreeText(&filters, scope.text)
	rawSQL := "SELECT je.value AS raw FROM ojs_submissions s, json_each(s.evidence, '$.editors') je" + whereClause(filters.conditions)
	type rawRow struct {
		Raw string `db:"raw"`
	}
	var rows []rawRow
	if err := app.DB().NewQuery(rawSQL).Bind(filters.params).All(&rows); err != nil {
		return nil, fmt.Errorf("aggregate editor frequency: %w", err)
	}
	names := make([]string, len(rows))
	for i, row := range rows {
		names[i] = row.Raw
	}
	return names, nil
}

// rankedFrequencies orders the merged editors by count, highest first, then by
// slug, so the order is the same across runs.
func rankedFrequencies(names []string) []EditorFrequency {
	frequencies := mergedEditors(names)
	sort.Slice(frequencies, func(i, j int) bool {
		if frequencies[i].Count != frequencies[j].Count {
			return frequencies[i].Count > frequencies[j].Count
		}
		return frequencies[i].Slug < frequencies[j].Slug
	})
	return frequencies
}

// mergedEditors folds the raw display names into one entry per slug (ADR-0006).
func mergedEditors(names []string) []EditorFrequency {
	bySlug := map[string]*EditorFrequency{}
	for _, raw := range names {
		countEditorName(bySlug, raw)
	}
	frequencies := make([]EditorFrequency, 0, len(bySlug))
	for _, entry := range bySlug {
		frequencies = append(frequencies, *entry)
	}
	return frequencies
}

// countEditorName adds one submission to the entry for a raw display name's
// slug. The label is the alphabetically first raw form seen. A raw value that
// slugs to nothing counts for nobody.
func countEditorName(bySlug map[string]*EditorFrequency, raw string) {
	slug := normalizer.NameSlug(raw)
	if slug == "" {
		return
	}
	entry, ok := bySlug[slug]
	if !ok {
		entry = &EditorFrequency{Slug: slug, Label: raw}
		bySlug[slug] = entry
	}
	entry.Label = min(entry.Label, raw)
	entry.Count++
}

// parseEditorFrequencyScope reads the `folder`/`text` query params the search
// combobox sends for live editor-suggestion ranking. `folder` carries the
// client's own vocabulary (`work`/`up_to_date`/`muted`/`archived`) and is
// passed through to parseScope, the same as a `folder:` token in the `query`
// wire format.
func parseEditorFrequencyScope(e *core.RequestEvent) (editorFrequencyScope, error) {
	params := e.Request.URL.Query()
	scope := editorFrequencyScope{text: params.Get("text")}
	if slug := params.Get("folder"); slug != "" {
		queue, ok := folderQueue(slug)
		if !ok {
			return scope, clientError{"unknown folder: " + slug}
		}
		scope.queue = queue
	}
	return scope, nil
}

// loadEditorFrequenciesRanked applies spec.md's third ranking-fallback rule:
// a scope narrowed by folder and/or free text that names no editor falls
// back to the unrestricted global ranking, instead of showing an empty list.
func loadEditorFrequenciesRanked(app core.App, scope editorFrequencyScope) ([]EditorFrequency, error) {
	editors, err := loadEditorFrequencies(app, scope)
	if err != nil {
		return nil, err
	}
	if len(editors) > 0 {
		return editors, nil
	}
	if scope == (editorFrequencyScope{}) {
		return editors, nil
	}
	return loadEditorFrequencies(app, editorFrequencyScope{})
}
