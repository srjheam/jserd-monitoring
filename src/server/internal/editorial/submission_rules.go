package editorial

import (
	"server/internal/normalizer"
	"sort"
	"time"
)

// submissionFindings selects the rules for a workflow stage and ranks the result.
func (e Evaluator) submissionFindings(submission normalizer.Submission, reviews []normalizer.ReviewAssignment, deliveries DeliveryLookup, now time.Time) []Finding {
	if submission.Status == statusCopyediting {
		return copyeditingFindings(submission)
	}
	findings := make([]Finding, 0, findingsCapacityHint)
	findings = append(findings, reviewStageFindings(submission, reviews, e.requiredReviewers(), now)...)
	findings = append(findings, revisionFindings(submission, reviews, now, e.fallbackDays())...)
	findings = append(findings, editorialFindings(submission, reviews, deliveries)...)
	sort.SliceStable(findings, func(i, j int) bool { return rankOf(findings[i].Kind) < rankOf(findings[j].Kind) })
	return findings
}

func isEvaluableSubmission(sub normalizer.Submission) bool {
	return hasSubmissionIdentity(sub) && (sub.Status == "Submission" || sub.Status == "Review" ||
		(sub.Status == statusCopyediting && sub.SubmissionProgress == 0))
}

func hasSubmissionIdentity(sub normalizer.Submission) bool {
	return sub.OJSSubmissionID != "" && sub.DateSubmitted != ""
}
