package monitoring

import (
	"net/url"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 100
	// defaultQueue is the work-list scope a request gets when it names no
	// queue at all, the same "daily" spelling queue.go stores.
	defaultQueue = "daily"
)

type listQuery struct {
	limit             int
	sortKey           string
	desc              bool
	queue             string
	requiresAttention bool
	conference        string
	status            string
	attention         string
	editor            string
	queryText         string
	hasCursor         bool
	cursor            cursorPayload
}

// listQueryPart reads one group of related query params into the parsed query.
// The order of the parts matters: applyCursor reads the sort key and the
// direction that applyOrder resolved.
type listQueryPart func(url.Values, *listQuery) error

var listQueryParts = []listQueryPart{applyLimit, applyOrder, applyQueue, applyCursor}

func parseListQuery(e *core.RequestEvent) (listQuery, error) {
	query := e.Request.URL.Query()
	parsed := listQuery{
		sortKey:    "id",
		queue:      defaultQueue,
		conference: query.Get("conference"),
		status:     query.Get("status"),
		attention:  query.Get("attention"),
		editor:     query.Get("editor"),
		queryText:  query.Get("query"),
	}
	for _, apply := range listQueryParts {
		if err := apply(query, &parsed); err != nil {
			return parsed, err
		}
	}
	return parsed, nil
}

// applyLimit reads the page size, clamped to the allowed range. A request that
// asks for no size gets the default page size.
func applyLimit(query url.Values, parsed *listQuery) error {
	raw := query.Get("limit")
	if raw == "" {
		parsed.limit = defaultPageLimit
		return nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return clientError{"limit must be an integer"}
	}
	parsed.limit = min(max(limit, 1), maxPageLimit)
	return nil
}

// sortDirections maps every accepted `dir` value to the descending flag it
// stands for. An absent `dir` sorts ascending.
var sortDirections = map[string]bool{"": false, "asc": false, "desc": true}

// applyOrder reads the sort column and the sort direction.
func applyOrder(query url.Values, parsed *listQuery) error {
	if raw := query.Get("sort"); raw != "" {
		if _, ok := sortColumns[raw]; !ok {
			return clientError{"unknown sort key"}
		}
		parsed.sortKey = raw
	}
	desc, ok := sortDirections[query.Get("dir")]
	if !ok {
		return clientError{"dir must be asc or desc"}
	}
	parsed.desc = desc
	return nil
}

// applyQueue reads the queue scope and the pending-work flag.
func applyQueue(query url.Values, parsed *listQuery) error {
	parsed.requiresAttention = query.Get("requiresAttention") == "true"
	if !query.Has("queue") {
		return nil
	}
	raw := query.Get("queue")
	if _, ok := parseScope(raw); !ok {
		return clientError{"unknown queue"}
	}
	parsed.queue = raw
	return nil
}
