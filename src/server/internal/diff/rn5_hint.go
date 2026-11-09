package diff

import "server/internal/normalizer"

// rn5HintChanges fires once per submission whose OJS review-round state
// moves from "revision requested" to "revision submitted" — RevisionPending
// going from true to false. This is OJS's own computed state (see
// server/internal/ojs, SubmissionDetail.LatestRoundRevisionPending), not an
// inferred file-count delta, so no corroborating review-completion marker
// is needed: OJS already only reaches "requested" after a decision follows
// a completed review.
func rn5HintChanges(previous, current normalizer.NormalizedCycle) []Change {
	previousPending := previousRevisionPending(previous.Submissions)
	changes := []Change{}
	for _, submission := range current.Submissions {
		if !revisionJustSubmitted(submission, previousPending) {
			continue
		}
		changes = append(changes, Change{Type: ChangeRN5Hint, EntityType: "submission", EntityKey: submission.OJSSubmissionID, Transition: "revision_requested->revision_submitted", OccurrenceKey: submission.OJSSubmissionID + "|pending->submitted"})
	}
	return changes
}

func previousRevisionPending(submissions []normalizer.Submission) map[string]bool {
	pending := make(map[string]bool, len(submissions))
	for _, submission := range submissions {
		if submission.RevisionPending != nil {
			pending[submission.OJSSubmissionID] = *submission.RevisionPending
		}
	}
	return pending
}

func revisionJustSubmitted(submission normalizer.Submission, previousPending map[string]bool) bool {
	if submission.RevisionPending == nil || *submission.RevisionPending {
		return false
	}
	before, observed := previousPending[submission.OJSSubmissionID]
	return observed && before
}
