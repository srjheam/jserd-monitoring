package ojs

import (
	"net/url"
	"strconv"
)

// validPagingQuery reports whether a query names one page of an allowed size,
// at an allowed offset, and names at most one other param.
func validPagingQuery(query url.Values) bool {
	if len(query) < 1 || len(query) > 2 {
		return false
	}
	return validCount(query.Get("count")) && validOffset(query.Get("offset"))
}

// validCount reports whether a page size is inside the allowed range.
func validCount(raw string) bool {
	count, err := strconv.Atoi(raw)
	return err == nil && count >= 1 && count <= maxSubmissionPageSize
}

// validOffset reports whether an offset is zero or more. An absent offset means
// the first page.
func validOffset(raw string) bool {
	if raw == "" {
		return true
	}
	offset, err := strconv.Atoi(raw)
	return err == nil && offset >= 0
}
