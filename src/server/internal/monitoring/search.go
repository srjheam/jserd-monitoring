package monitoring

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
)

// freeTextPredicate builds the condition matching a chief editor's free text.
//
// It reads the submission id and the title, and nothing else (ADR-0014). Both
// the work list and the editor-frequency ranking call it, so the words that
// narrow the list are the same words that scope the `editor:` suggestions
// ranked beside it. Those two used to disagree: the list also matched the
// assigned editors, verified_by and the journal body, so typing an editor's
// name narrowed the list while the ranking fell back to its global order.
//
// The text splits on whitespace and the words are ORed, so any one of them can
// match. That is the list's long-standing behavior, unchanged.
//
// Empty text produces an empty condition, so a caller can skip the clause.
// The condition names the `s` alias, which every caller uses for
// ojs_submissions.
func freeTextPredicate(text string) (string, dbx.Params) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "", dbx.Params{}
	}
	conditions := make([]string, len(words))
	params := dbx.Params{}
	for i, word := range words {
		key := fmt.Sprintf("needle_%d", i)
		conditions[i] = wordCondition(key)
		params[key] = "%" + escapeLike(strings.ToLower(word)) + "%"
	}
	return "(" + strings.Join(conditions, " OR ") + ")", params
}

// wordCondition matches one word against the two fields that identify a
// submission. ESCAPE keeps a `%`, `_` or `\` a chief editor typed literal.
func wordCondition(key string) string {
	placeholder := "{:" + key + "}"
	return "(LOWER(s.ojs_submission_id) LIKE " + placeholder + " ESCAPE '\\'" +
		" OR LOWER(s.title) LIKE " + placeholder + " ESCAPE '\\')"
}

// addFreeText appends the free-text condition, if the text holds any word.
func addFreeText(filters *filterSet, text string) {
	condition, params := freeTextPredicate(text)
	if condition == "" {
		return
	}
	filters.conditions = append(filters.conditions, condition)
	mergeParams(filters.params, params)
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
