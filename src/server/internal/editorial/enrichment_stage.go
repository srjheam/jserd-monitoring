package editorial

import (
	"context"
	"server/internal/ojs"
	"time"
)

// This file holds what every enrichment stage shares. A stage names its own
// candidates and merges its own result onto the cycle, but the call to OJS in
// between is the same call, and so is the meaning of an unset clock.

// nowOr returns the time an optional clock gives, or the real current time.
// Four stages take a clock so a test can fix the day; this is the one place
// that decides what leaving it unset means.
func nowOr(clock func() time.Time) time.Time {
	if clock != nil {
		return clock()
	}
	return time.Now()
}

// fetchDetails asks the detail provider for each submission in turn and
// collects what read finds in the answer. A submission read says nothing about
// is left out of the result rather than recorded as a zero value.
//
// This is the one place an enrichment stage calls OJS. Changing how a failed
// read is treated is one edit here, and it reaches RN5, RN2, RN7 and RN8
// together, instead of reaching whichever stage the maintainer remembered.
func fetchDetails[T any](ctx context.Context, details SubmissionDetailProvider, submissionIDs []string, read func(ojs.SubmissionDetail) (T, bool)) (map[string]T, error) {
	values := make(map[string]T, len(submissionIDs))
	for _, submissionID := range submissionIDs {
		detail, err := details.Get(ctx, submissionID)
		if err != nil {
			return nil, err
		}
		if value, found := read(detail); found {
			values[submissionID] = value
		}
	}
	return values, nil
}

// always wraps a reader that answers for every submission, so it fits the
// shape fetchDetails takes.
func always[T any](read func(ojs.SubmissionDetail) T) func(ojs.SubmissionDetail) (T, bool) {
	return func(detail ojs.SubmissionDetail) (T, bool) {
		return read(detail), true
	}
}
