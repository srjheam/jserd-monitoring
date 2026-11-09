package events

import (
	"server/internal/normalizer"
	"sort"
	"time"
)

func Temporal(cycle normalizer.NormalizedCycle, now time.Time) []Event {
	now = now.UTC()
	result := []Event{}
	for _, review := range cycle.Reviews {
		if review.Key == "" || review.ReviewerUsername == "" {
			continue
		}
		result = append(result, responseEvents(review, now)...)
		result = append(result, reviewDueEvents(review, now)...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].IdempotencyKey < result[j].IdempotencyKey })
	return result
}

func temporalEvent(eventType string, review normalizer.ReviewAssignment, boundary string, due time.Time) Event {
	dueText := due.UTC().Format(time.RFC3339)
	targetKind, targetKey := "reviewer", review.ReviewerUsername
	if eventType == EventReviewResponseDue {
		targetKind, targetKey = "editor", ""
	}
	event := Event{Type: eventType, EntityType: "review", EntityKey: review.Key, Transition: boundary, OccurrenceKey: review.Key + "|" + boundary + "|" + dueText, SubmissionID: review.SubmissionID, TargetKind: targetKind, TargetKey: targetKey}
	event.IdempotencyKey = IdempotencyKey(event)
	return event
}
