package events

import (
	"server/internal/normalizer"
	"time"
)

func reviewDueEvents(review normalizer.ReviewAssignment, now time.Time) []Event {
	due, ok := normalizer.ParseDate(review.ReviewDueDate)
	if !ok || review.DateCompleted != "" {
		return nil
	}
	return append(reviewDueReminderEvents(review, now, due), reviewOverdueEvents(review, now, due)...)
}

// reviewDueReminderEvents covers the two calendar days before the due date
// and ends where reviewOverdueEvents begins, so one deadline never yields
// both an approaching and an overdue notice on the same day.
func reviewDueReminderEvents(review normalizer.ReviewAssignment, now, due time.Time) []Event {
	dueDay := startOfDueDay(due)
	if review.ReviewDueWindowSuperseded || now.Before(dueDay.AddDate(0, 0, -2)) || !now.Before(dueDay) {
		return nil
	}
	return []Event{temporalEvent(EventReviewDueReminder, review, "due-2d", due)}
}

func startOfDueDay(due time.Time) time.Time {
	return time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, due.Location())
}

// reviewOverdueEvents compares against the start of the due date's calendar
// day, not its end: OJS's own reviewAssignment status already reports a
// review overdue from the first moment of its due date.
func reviewOverdueEvents(review normalizer.ReviewAssignment, now, due time.Time) []Event {
	if review.ReviewDueWindowSuperseded || now.Before(startOfDueDay(due)) {
		return nil
	}
	return []Event{temporalEvent(EventReviewDue, review, "overdue", due)}
}
