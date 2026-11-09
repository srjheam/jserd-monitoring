package editorial

import (
	"context"
	"server/internal/normalizer"
	"server/internal/ojs"
)

// OJSActiveReviewersEnricher is a Coordinator.Enrichers stage for RN9
// (missing_reviewer) and RN19 (overdue_review_invitation). OJS fills
// DateConfirmed on a decline as well as on an acceptance, so the Reviews
// export counts a declined reviewer as active. The detail query omits
// declined assignments, so its count is the one these rules read.
type OJSActiveReviewersEnricher struct{}

func (OJSActiveReviewersEnricher) Enrich(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	candidates := activeReviewerCandidates(*cycle)
	if len(candidates) == 0 {
		return nil
	}
	counts, err := fetchDetails(ctx, details, candidates, always(ojs.SubmissionDetail.ActiveReviewers))
	if err != nil {
		return err
	}
	for i := range cycle.Submissions {
		if count, ok := counts[cycle.Submissions[i].OJSSubmissionID]; ok {
			value := count
			cycle.Submissions[i].ActiveReviewers = &value
		}
	}
	return nil
}

// activeReviewerCandidates names the submissions RN9 or RN19 can fire for:
// evaluable, in the Review stage, with at least one incomplete review.
func activeReviewerCandidates(cycle normalizer.NormalizedCycle) []string {
	bySubmission := groupReviewsBySubmission(cycle.Reviews)
	ids := make([]string, 0)
	for _, submission := range cycle.Submissions {
		reviews := bySubmission[submission.OJSSubmissionID]
		if submission.Status == "Review" && isEvaluableSubmission(submission) && len(reviews) > 0 && !allReviewsCompleted(reviews) {
			ids = append(ids, submission.OJSSubmissionID)
		}
	}
	return ids
}
