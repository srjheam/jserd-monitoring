package editorial

import (
	"context"
	"fmt"
	"server/internal/events"
	"server/internal/normalizer"
	"server/internal/ojs"
	"strings"
	"time"
)

// OJSReviewDueEnricher is a second Coordinator.PreProcessEnrichers stage,
// for RN7/RN8. Like
// RN2's enricher, it must run before Process, since Process is where
// events.Temporal decides RN7/RN8. It calls the SubmissionDetailProvider
// only for the submissions events.CandidatesNeedingReviewDueCheck names,
// and merges the result onto the cycle before Process runs.
type OJSReviewDueEnricher struct {
	Now func() time.Time
}

func (e OJSReviewDueEnricher) Enrich(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	candidates := events.CandidatesNeedingReviewDueCheck(*cycle, nowOr(e.Now))
	if len(candidates) == 0 {
		return nil
	}
	windows, err := fetchDetails(ctx, details, candidates, always(ojs.SubmissionDetail.ResolvedReviewWindows))
	if err != nil {
		return err
	}
	applyResolvedReviewWindowEnrichment(cycle, windows)
	return nil
}

func applyResolvedReviewWindowEnrichment(cycle *normalizer.NormalizedCycle, windows map[string]map[string]bool) {
	for i := range cycle.Reviews {
		review := &cycle.Reviews[i]
		resolved, ok := windows[review.SubmissionID]
		if !ok {
			continue
		}
		review.ReviewDueWindowSuperseded = reviewWindowSuperseded(resolved, review.Round, review.ReviewDueDate)
	}
}

// reviewWindowSuperseded treats a window OJS's live state marks resolved,
// or a window with no live reviewAssignment at all, as superseded: the
// reviewer was replaced or the round moved on since the CSV export's row
// was written.
func reviewWindowSuperseded(resolved map[string]bool, round int, due string) bool {
	value, ok := resolved[fmt.Sprintf("%d|%s", round, dueDateOnly(due))]
	return !ok || value
}

// dueDateOnly trims the CSV export's "Review Due Date" ("2026-08-24
// 23:59:59") down to its calendar date, matching OJS's live
// reviewAssignment "due" field, which carries no time of day
// ("2026-08-24"). Without this, the two never share a lookup key, and
// every window reads as having no live match at all — superseded, even a
// genuinely still-open one.
func dueDateOnly(due string) string {
	if idx := strings.IndexByte(due, ' '); idx >= 0 {
		return due[:idx]
	}
	return due
}
