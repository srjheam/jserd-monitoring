package events

import (
	"server/internal/normalizer"
	"time"
)

const statusCopyediting = "Copyediting"

// Copyediting reports a newly observed missing copyeditor, including the first
// observed cycle. Unchanged absence does not notify again; resolved absence can.
func Copyediting(previous, current normalizer.NormalizedCycle, observedAt time.Time) []Event {
	missingBefore := make(map[string]bool, len(previous.Submissions))
	for _, submission := range previous.Submissions {
		missingBefore[submission.OJSSubmissionID] = missingCopyeditor(submission)
	}
	result := []Event{}
	for _, submission := range current.Submissions {
		if !missingCopyeditor(submission) || missingBefore[submission.OJSSubmissionID] {
			continue
		}
		event := Event{
			Type: EventMissingCopyeditor, EntityType: "submission", EntityKey: submission.OJSSubmissionID,
			SubmissionID: submission.OJSSubmissionID, TargetKind: "editor", Transition: "copyeditor_missing",
			OccurrenceKey: observedAt.UTC().Format(time.RFC3339Nano),
		}
		event.IdempotencyKey = IdempotencyKey(event)
		result = append(result, event)
	}
	return result
}

func missingCopyeditor(submission normalizer.Submission) bool {
	return submission.OJSSubmissionID != "" && submission.DateSubmitted != "" &&
		submission.SubmissionProgress == 0 && submission.Status == statusCopyediting &&
		submission.CopyeditorAssigned != nil && !*submission.CopyeditorAssigned
}
