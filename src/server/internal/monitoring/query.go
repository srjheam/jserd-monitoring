package monitoring

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// SubmissionPage is one server-paginated batch of the submissions table.
type SubmissionPage struct {
	Items      []Submission `json:"items"`
	NextCursor string       `json:"nextCursor"`
	Total      int          `json:"total"`
}

// clientError marks request mistakes that must surface as HTTP 400.
type clientError struct{ message string }

func (e clientError) Error() string { return e.message }

type pageRow struct {
	ID                  string `db:"id"`
	OjsSubmissionID     string `db:"ojs_submission_id"`
	Title               string `db:"title"`
	Status              string `db:"status"`
	EffectiveFinalState string `db:"effective_final_state"`
	EffectiveQueue      string `db:"effective_queue"`
	EffectiveAttention  string `db:"effective_attention"`
	Evidence            string `db:"evidence"`
	FindingDigest       string `db:"finding_digest"`
	// SortKey is the value of the requested sort expression for this row.
	// The priority sort is computed in SQL, so it has no column to read back.
	SortKey string `db:"sort_key"`
}

// ListSubmissions returns one keyset-paginated batch of materialized
// submissions matching the requested filters and sort order.
func ListSubmissions(app core.App, query listQuery) (*SubmissionPage, error) {
	filters, err := buildFilters(app, query)
	if err != nil {
		return nil, err
	}
	// The status a submission is in always outranks whichever column the
	// chief editor picked to sort by, per the Daily/Up to date/Muted/Archived
	// display order. The rank is folded ahead of the column into one text
	// key, the same fold prioritySortExpr already uses, so the existing
	// single-column keyset cursor still applies unchanged.
	sortExpr := "(" + orderRank(query.desc) + " || " + sortColumns[query.sortKey] + ")"
	rows, err := pageRows(app, query, filters, sortExpr)
	if err != nil {
		return nil, err
	}
	total, err := countSubmissions(app, filters)
	if err != nil {
		return nil, err
	}
	return pageResponse(app, query, rows, total)
}

// pageRows reads one batch of stored rows. It asks for one row more than the
// page size, so the caller can tell whether another page follows.
func pageRows(app core.App, query listQuery, filters filterSet, sortExpr string) ([]pageRow, error) {
	conditions := append([]string{}, filters.conditions...)
	params := dbx.Params{}
	mergeParams(params, filters.params)
	if query.hasCursor {
		conditions = append(conditions, cursorCondition(query, sortExpr))
		params["cursor_value"] = query.cursor.V
		params["cursor_id"] = query.cursor.I
	}
	params["limit"] = query.limit + 1
	listSQL := "SELECT s.id, s.ojs_submission_id, s.title, s.status, s.effective_final_state, s.effective_queue, s.effective_attention, s.evidence, s.finding_digest, " + sortExpr + " AS sort_key FROM ojs_submissions s" +
		whereClause(conditions) +
		" ORDER BY " + orderByClause(query, sortExpr) + " LIMIT {:limit}"
	var rows []pageRow
	if err := app.DB().NewQuery(listSQL).Bind(params).All(&rows); err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}
	return rows, nil
}

// countSubmissions counts every submission the filters match, across all pages.
func countSubmissions(app core.App, filters filterSet) (int, error) {
	totalSQL := "SELECT COUNT(*) FROM ojs_submissions s" + whereClause(filters.conditions)
	var total int
	if err := app.DB().NewQuery(totalSQL).Bind(filters.params).Row(&total); err != nil {
		return 0, fmt.Errorf("count submissions: %w", err)
	}
	return total, nil
}

// pageResponse turns one over-long batch of stored rows into the page the
// route answers with.
func pageResponse(app core.App, query listQuery, rows []pageRow, total int) (*SubmissionPage, error) {
	hasMore := len(rows) > query.limit
	if hasMore {
		rows = rows[:query.limit]
	}
	nextCursor := ""
	if hasMore {
		cursor, err := nextPageCursor(query, rows)
		if err != nil {
			return nil, err
		}
		nextCursor = cursor
	}
	items, err := submissionViews(app, rows)
	if err != nil {
		return nil, err
	}
	return &SubmissionPage{Items: items, NextCursor: nextCursor, Total: total}, nil
}
