package events

import (
	"server/internal/normalizer"
	"time"
)

// reviewDueWindowIsOpen matches reviewDueReminderEvents' and
// reviewOverdueEvents' own firing window, minus ReviewDueWindowSuperseded:
// an incomplete review whose 2-day RN7 reminder window has opened. This also
// covers every already-overdue review, since the window only ever grows
// once opened.
func reviewDueWindowIsOpen(review normalizer.ReviewAssignment, now time.Time) bool {
	due, ok := normalizer.ParseDate(review.ReviewDueDate)
	return ok && review.DateCompleted == "" && !now.Before(startOfDueDay(due).AddDate(0, 0, -2))
}

// CandidatesNeedingReviewDueCheck names the submissions RN7 or RN8 could
// fire for this cycle: at least one incomplete review whose 2-day reminder
// window has opened. A pre-Process enrichment stage must fetch OJS's own
// live reviewAssignment status for exactly these submissions before
// Temporal runs, since a cancelled or superseded assignment reads as
// permanently due otherwise.
func CandidatesNeedingReviewDueCheck(cycle normalizer.NormalizedCycle, now time.Time) []string {
	now = now.UTC()
	seen := map[string]bool{}
	ids := []string{}
	for _, review := range cycle.Reviews {
		if !isNewReviewDueCandidate(review, now, seen) {
			continue
		}
		seen[review.SubmissionID] = true
		ids = append(ids, review.SubmissionID)
	}
	return ids
}

func isNewReviewDueCandidate(review normalizer.ReviewAssignment, now time.Time, seen map[string]bool) bool {
	return review.SubmissionID != "" && !seen[review.SubmissionID] && reviewDueWindowIsOpen(review, now)
}
