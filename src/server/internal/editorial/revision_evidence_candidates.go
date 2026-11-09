package editorial

import (
	"server/internal/normalizer"
	"time"
)

// This file names the submissions the OJSRevisionEvidenceEnricher stage
// (enrichment.go) must fetch live review-round status for: those a stale
// revision decision, with no later review round yet, could turn into a
// missing_revision_indication or revision_pending_decision finding once
// revisionFindings (revision_findings.go) runs.

// CandidatesNeedingRevisionEvidence reports which submission IDs
// missing_revision_indication (RN11), revision_pending_decision (RN16), and
// revised_version_hint (RN5, see diff.rn5HintChanges) need a live
// review-round status for: a revision decision with no later review round.
// The fetch starts on the decision date, not after the RN11 wait: RN5 needs
// the pending state in an earlier snapshot, so a revision that lands inside
// the wait still produces RN5. RN11 and RN16 apply the wait themselves
// (revisionFindings).
func (e Evaluator) CandidatesNeedingRevisionEvidence(cycle normalizer.NormalizedCycle, now time.Time) []string {
	bySubmission := groupReviewsBySubmission(cycle.Reviews)
	candidates := make([]string, 0)
	for _, submission := range cycle.Submissions {
		if !isEvaluableSubmission(submission) {
			continue
		}
		if needsRevisionEvidence(submission, bySubmission[submission.OJSSubmissionID], now.UTC()) {
			candidates = append(candidates, submission.OJSSubmissionID)
		}
	}
	return candidates
}

func needsRevisionEvidence(sub normalizer.Submission, reviews []normalizer.ReviewAssignment, now time.Time) bool {
	_, decidedAt, ok := latestRevisionDecision(sub)
	return ok && !hasLaterReviewRound(reviews, decidedAt) && !now.Before(decidedAt)
}
