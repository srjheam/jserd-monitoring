package monitoring

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// aggregateStep fills one group of the side-screen counts.
type aggregateStep func(core.App, *Aggregates) error

var aggregateSteps = []aggregateStep{addTriageCounts, addWorkCounts, addStatuses, addEditors}

func loadAggregates(app core.App) (int, Aggregates, error) {
	aggregates := Aggregates{Metrics: map[string]int{}, Triage: emptyTriage(), Statuses: []string{}, Editors: []EditorFrequency{}}
	total, err := scalarCount(app, "SELECT COUNT(*) FROM ojs_submissions", dbx.Params{}, "count submissions")
	if err != nil {
		return 0, aggregates, err
	}
	for _, add := range aggregateSteps {
		if err := add(app, &aggregates); err != nil {
			return 0, aggregates, err
		}
	}
	aggregates.Total = total
	return total, aggregates, nil
}

// bucket is one grouped count: the value grouped on, and how many rows carry it.
type bucket struct {
	Key   string `db:"key"`
	Total int    `db:"total"`
}

// scalarCount runs a count query and labels its failure.
func scalarCount(app core.App, query string, params dbx.Params, label string) (int, error) {
	var count int
	if err := app.DB().NewQuery(query).Bind(params).Row(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}
	return count, nil
}

// buckets runs a grouped count query and labels its failure.
func buckets(app core.App, query string, params dbx.Params, label string) ([]bucket, error) {
	var rows []bucket
	if err := app.DB().NewQuery(query).Bind(params).All(&rows); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return rows, nil
}

// addTriageCounts counts the submissions in each queue. A queue the display
// does not name is left out.
func addTriageCounts(app core.App, aggregates *Aggregates) error {
	queues, err := buckets(app, "SELECT effective_queue AS key, COUNT(*) AS total FROM ojs_submissions GROUP BY effective_queue", dbx.Params{}, "aggregate queues")
	if err != nil {
		return err
	}
	for _, entry := range queues {
		if _, known := aggregates.Triage[entry.Key]; known {
			aggregates.Triage[entry.Key] = entry.Total
		}
	}
	return nil
}

// addWorkCounts counts the work list: how many submissions carry pending work,
// and how many sit in each attention bucket.
func addWorkCounts(app core.App, aggregates *Aggregates) error {
	// The work-list queue, named once here and resolved by the queue module.
	workScope, workParams := mustScope("work").predicate()
	requiring, err := scalarCount(app, "SELECT COUNT(*) FROM ojs_submissions s WHERE "+workScope+" AND "+requiresAttention("s"), workParams, "aggregate daily requiring attention")
	if err != nil {
		return err
	}
	aggregates.DailyRequiringAttention = requiring
	metrics, err := buckets(app, "SELECT s.effective_attention AS key, COUNT(*) AS total FROM ojs_submissions s WHERE "+workScope+" GROUP BY s.effective_attention", workParams, "aggregate metrics")
	if err != nil {
		return err
	}
	for _, entry := range metrics {
		aggregates.Metrics[entry.Key] = entry.Total
	}
	return nil
}

// addStatuses lists every OJS status the projection holds, in alphabetical
// order.
func addStatuses(app core.App, aggregates *Aggregates) error {
	statuses, err := buckets(app, "SELECT DISTINCT status AS key FROM ojs_submissions WHERE status <> '' ORDER BY status COLLATE NOCASE", dbx.Params{}, "aggregate statuses")
	if err != nil {
		return err
	}
	for _, entry := range statuses {
		aggregates.Statuses = append(aggregates.Statuses, entry.Key)
	}
	return nil
}

// addEditors ranks every assigned editor across the whole projection.
func addEditors(app core.App, aggregates *Aggregates) error {
	editors, err := loadEditorFrequencies(app, editorFrequencyScope{})
	if err != nil {
		return err
	}
	aggregates.Editors = editors
	return nil
}
