package monitoring

import "strings"

func whereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

// sqlDirections are the SQL keywords for the two requested directions.
var sqlDirections = map[bool]string{false: "ASC", true: "DESC"}

// orderByClause orders by the folded sort key, with the OJS submission id as
// the tiebreaker.
func orderByClause(q listQuery, sortExpr string) string {
	direction := sqlDirections[q.desc]
	orderBy := sortExpr + " COLLATE NOCASE " + direction
	if q.sortKey == "id" {
		// The id is already the tiebreaker for every other sort key; repeating
		// it when it's also the primary sort key duplicates the ORDER BY term,
		// which stops SQLite from resolving the sort from the index alone.
		return orderBy
	}
	return orderBy + ", s.ojs_submission_id COLLATE NOCASE " + direction
}
