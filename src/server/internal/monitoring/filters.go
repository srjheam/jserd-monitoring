package monitoring

import (
	"cmp"
	"fmt"
	"server/internal/normalizer"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type filterSet struct {
	conditions []string
	params     dbx.Params
}

// addScope appends a queue scope's condition, if it has one. An unrestricted
// scope adds nothing.
func addScope(filters *filterSet, s queueScope) {
	condition, params := s.predicate()
	if condition == "" {
		return
	}
	filters.conditions = append(filters.conditions, condition)
	mergeParams(filters.params, params)
}

func buildFilters(app core.App, parsed listQuery) (filterSet, error) {
	filters := filterSet{params: dbx.Params{}}
	chips, folder, free, err := parseSearchQuery(parsed.queryText)
	if err != nil {
		return filters, err
	}
	// A folder: token in the query text sets the same queue scope the route
	// sets via parsed.queue, and wins when both are present. Absence of a
	// folder: token leaves parsed.queue as the scope, so an empty
	// parsed.queue (the all-folders mode from part B) still applies with no
	// folder: token typed.
	//
	// parseListQuery and parseSearchQuery both rejected an unknown slug already,
	// so this value is one the queue module knows.
	addScope(&filters, mustScope(cmp.Or(folder, parsed.queue)))
	if parsed.requiresAttention {
		filters.conditions = append(filters.conditions, requiresAttention("s"))
	}
	addFieldFilters(&filters, parsed)
	// Chips of the same kind combine with AND: a chief editor who names two
	// editors wants the submissions both of them share.
	chipConditions, chipParams, err := editorChipFilters(app, chips)
	if err != nil {
		return filters, err
	}
	filters.conditions = append(filters.conditions, chipConditions...)
	mergeParams(filters.params, chipParams)
	addFreeText(&filters, free)
	return filters, nil
}

// statusFieldName is the "status" spelling the package uses consistently: a
// stored column, a JSON field, a bound query param name and an OJS export
// key. Naming it once here avoids a duplicate literal everywhere it recurs.
const statusFieldName = "status"

// fieldFilters lists the single-value list filters, in the order they join the
// WHERE clause. Each entry names the bound param and the condition that reads
// it, and reads its value from the parsed query.
var fieldFilters = []struct {
	name      string
	condition string
	value     func(listQuery) string
}{
	{"conference", "EXISTS (SELECT 1 FROM submission_tracking ft WHERE ft.ojs_submission_id = s.ojs_submission_id AND ft.conference = {:conference})", func(q listQuery) string { return q.conference }},
	{statusFieldName, "s.status = {:status}", func(q listQuery) string { return q.status }},
	{"attention", "s.effective_attention = {:attention}", func(q listQuery) string { return q.attention }},
	{"editor", "EXISTS (SELECT 1 FROM json_each(s.evidence, '$.editors') fe WHERE fe.value = {:editor})", func(q listQuery) string { return q.editor }},
}

// addFieldFilters appends the condition of every single-value filter the
// request set. An empty value adds nothing.
func addFieldFilters(filters *filterSet, q listQuery) {
	for _, filter := range fieldFilters {
		value := filter.value(q)
		if value == "" {
			continue
		}
		filters.conditions = append(filters.conditions, filter.condition)
		filters.params[filter.name] = value
	}
}

// editorChipFilters builds one EXISTS condition per editor: chip, matching
// any raw display name that collides on the chip's slug (ADR-0006), so
// picking a merged suggestion returns every person hiding behind that slug
// instead of only the one raw name the chip was typed or selected as.
func editorChipFilters(app core.App, chips []string) ([]string, dbx.Params, error) {
	if len(chips) == 0 {
		return nil, dbx.Params{}, nil
	}
	aliasIndex, err := editorAliasIndex(app)
	if err != nil {
		return nil, nil, err
	}
	conditions := []string{}
	params := dbx.Params{}
	for i, chip := range chips {
		condition, chipParams := editorChipCondition(aliasIndex, i, chip)
		conditions = append(conditions, condition)
		mergeParams(params, chipParams)
	}
	return conditions, params, nil
}

// mergeParams copies every entry of src into dst.
func mergeParams(dst, src dbx.Params) {
	for key, value := range src {
		dst[key] = value
	}
}

// editorChipCondition builds the EXISTS condition for one editor: chip,
// matching any of the raw names aliasIndex resolved for its slug. A chip
// naming no known raw value (a name typed but never assigned in the data)
// falls back to matching itself, so it still behaves like an exact filter.
func editorChipCondition(aliasIndex map[string][]string, index int, chip string) (string, dbx.Params) {
	aliases := aliasIndex[normalizer.NameSlug(chip)]
	if len(aliases) == 0 {
		aliases = []string{chip}
	}
	placeholders, params := inListParams(fmt.Sprintf("chip_editor_%d", index), aliases)
	return "EXISTS (SELECT 1 FROM json_each(s.evidence, '$.editors') fe WHERE fe.value IN (" + placeholders + "))", params
}

// editorAliasIndex maps each assigned-editor slug (ADR-0006) to every raw
// display name that collides on it, across the whole projection.
func editorAliasIndex(app core.App) (map[string][]string, error) {
	type rawRow struct {
		Raw string `db:"raw"`
	}
	var rows []rawRow
	query := "SELECT DISTINCT je.value AS raw FROM ojs_submissions s, json_each(s.evidence, '$.editors') je"
	if err := app.DB().NewQuery(query).All(&rows); err != nil {
		return nil, fmt.Errorf("aggregate editor aliases: %w", err)
	}
	index := map[string][]string{}
	for _, row := range rows {
		slug := normalizer.NameSlug(row.Raw)
		// A raw value that slugs to nothing (pure punctuation) must not
		// collide with every other such value under one shared "" key: an
		// `editor:` chip that also slugs empty would otherwise match every
		// submission carrying any garbage editor value.
		if slug == "" {
			continue
		}
		index[slug] = append(index[slug], row.Raw)
	}
	return index, nil
}

// inListParams builds a `{:prefix_0}, {:prefix_1}, ...` placeholder list and
// its bound params for an IN (...) condition.
func inListParams(prefix string, values []string) (string, dbx.Params) {
	placeholders := make([]string, len(values))
	params := dbx.Params{}
	for i, value := range values {
		key := fmt.Sprintf("%s_%d", prefix, i)
		placeholders[i] = "{:" + key + "}"
		params[key] = value
	}
	return strings.Join(placeholders, ", "), params
}
