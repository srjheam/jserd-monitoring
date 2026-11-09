package monitoring

import (
	"server/internal/editorial"
	"strings"
)

// prioritySortExpr folds the two levels of the work list order into one
// sortable text key, so the existing single-column keyset cursor still
// applies. Level 1 is the rank of the worst finding, zero padded so it
// compares as text; level 2 is elapsed days descending, which is the
// submission date ascending.
//
// editorial.PriorityFor already ranks a submission with no findings last, at
// len(Kinds())+1. The CASE only catches a priority outside that range, which
// means a row materialized before ticket 02 wrote the column: it holds 0, and
// 0 would otherwise sort ahead of the worst finding there is.
//
// A missing dateSubmitted coalesces to a sentinel above every real
// YYYY-MM-DD, so it trails its rank instead of heading it. An empty string
// would otherwise compare below every real date and read as the oldest
// submission there is, which is the opposite of "unknown". A missing OJS
// date normalizes to "" rather than to a JSON null, so NULLIF is needed
// beside COALESCE: json_extract("") is "", and COALESCE alone only replaces
// a SQL NULL.
const prioritySortExpr = "(printf('%02d', CASE WHEN s.effective_priority BETWEEN 1 AND 98 THEN s.effective_priority ELSE 99 END) || COALESCE(NULLIF(json_extract(s.evidence, '$.dateSubmitted'), ''), '9999-99-99'))"

var sortColumns = map[string]string{
	"id":            "s.ojs_submission_id",
	"title":         "s.title",
	statusFieldName: "s.status",
	"attention":     "s.effective_attention",
	"priority":      prioritySortExpr,
	// activity is Archived's own order: a closed submission carries no
	// actionable finding rank, so it sorts purely by OJS's own last-activity
	// timestamp instead. json_extract never returns SQL NULL here, since
	// dateLastActivity always serializes as a string (possibly empty), so a
	// submission OJS never reported activity for sorts last under dir=desc.
	"activity": "json_extract(s.evidence, '$.dateLastActivity')",
}

// requiresAttention builds the condition that matches every submission with at
// least one finding. It lists the finding kinds instead of excluding "normal",
// so a row written before the attention derivation never counts.
func requiresAttention(qualifier string) string {
	kinds := editorial.Kinds()
	quoted := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		quoted = append(quoted, "'"+string(kind)+"'")
	}
	return qualifier + ".effective_attention IN (" + strings.Join(quoted, ",") + ")"
}
