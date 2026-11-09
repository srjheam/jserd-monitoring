package ojs

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func validateRESTPath(parsed *url.URL) error {
	if id, ok := strings.CutPrefix(parsed.Path, "submissions/"); ok {
		return validateSubmissionDetailPath(id, parsed.Query())
	}
	return validateSubmissionOrUserListPath(parsed)
}

// validateSubmissionDetailPath allows only "submissions/<positive integer>"
// with no query string, the shape SubmissionDetail needs.
func validateSubmissionDetailPath(submissionID string, query url.Values) error {
	id, err := strconv.Atoi(submissionID)
	if err != nil || id <= 0 || len(query) != 0 {
		return fmt.Errorf("unsupported REST path %q", "submissions/"+submissionID)
	}
	return nil
}

// listPaths are the two REST collections this client pages through.
var listPaths = map[string]bool{"submissions": true, "users": true}

func validateSubmissionOrUserListPath(parsed *url.URL) error {
	if !listPaths[parsed.Path] || !validPagingQuery(parsed.Query()) {
		return fmt.Errorf("unsupported REST path %q", parsed.Path)
	}
	return nil
}
