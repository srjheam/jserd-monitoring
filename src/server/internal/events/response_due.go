package events

import (
	"server/internal/normalizer"
	"time"
)

// responseEvents handles RN6 (review_response_reminder) and RN2
// (review_response_due). Neither gates on Declined: OJS's CSV export
// always reports it as true, so it carries no signal.
// RN2 instead gates on RoundHasOverdueResponse, set by a pre-Process
// enrichment stage from OJS's own reviewAssignment statusId, which the CSV
// export cannot provide. RN6 needs no such gate: responseEvents already
// requires DateConfirmed == "" below, which is enough to know the review is
// still open ahead of its due date.
func responseEvents(review normalizer.ReviewAssignment, now time.Time) []Event {
	due, ok := normalizer.ParseDate(review.ResponseDueDate)
	if !ok || review.DateConfirmed != "" {
		return nil
	}
	return append(responseReminderEvents(review, now, due), responseDueEvents(review, now, due)...)
}

func responseReminderEvents(review normalizer.ReviewAssignment, now, due time.Time) []Event {
	if now.Before(due.AddDate(0, 0, -2)) || now.After(due) {
		return nil
	}
	return []Event{temporalEvent(EventReviewResponseReminder, review, "due-2d", due)}
}

func responseDueEvents(review normalizer.ReviewAssignment, now, due time.Time) []Event {
	if !now.After(due) || !review.RoundHasOverdueResponse {
		return nil
	}
	return []Event{temporalEvent(EventReviewResponseDue, review, "overdue", due)}
}

// responseIsOverdueAndUnconfirmed matches responseDueEvents' own firing
// condition, minus RoundHasOverdueResponse, so the pre-Process enrichment
// candidate set (CandidatesNeedingOverdueReviewCheck) and RN2's own firing
// condition cannot drift apart.
func responseIsOverdueAndUnconfirmed(review normalizer.ReviewAssignment, now time.Time) bool {
	due, ok := normalizer.ParseDate(review.ResponseDueDate)
	return ok && review.DateConfirmed == "" && now.After(due)
}

// CandidatesNeedingOverdueReviewCheck names the submissions RN2 could fire
// for this cycle: at least one unconfirmed review whose response due date
// has already passed. A pre-Process enrichment stage must fetch OJS's own
// per-round overdue status for exactly these submissions before Temporal
// runs, since the CSV export's Declined column cannot tell "still
// deciding" apart from "already declined".
func CandidatesNeedingOverdueReviewCheck(cycle normalizer.NormalizedCycle, now time.Time) []string {
	now = now.UTC()
	seen := map[string]bool{}
	ids := []string{}
	for _, review := range cycle.Reviews {
		if !isNewOverdueCandidate(review, now, seen) {
			continue
		}
		seen[review.SubmissionID] = true
		ids = append(ids, review.SubmissionID)
	}
	return ids
}

func isNewOverdueCandidate(review normalizer.ReviewAssignment, now time.Time, seen map[string]bool) bool {
	return review.SubmissionID != "" && !seen[review.SubmissionID] && responseIsOverdueAndUnconfirmed(review, now)
}
