package editorial

import (
	"context"
	"server/internal/normalizer"
	"server/internal/ojs"
)

// OJSCopyeditingEnricher loads Copyediting-stage assignment state before
// Process persists the cycle, so the evaluator can read the same evidence.
type OJSCopyeditingEnricher struct{}

func (OJSCopyeditingEnricher) Enrich(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	candidates := copyeditingCandidates(*cycle)
	if len(candidates) == 0 {
		return nil
	}
	assignments, err := fetchDetails(ctx, details, candidates, func(detail ojs.SubmissionDetail) (bool, bool) {
		if detail.CopyeditorAssigned == nil {
			return false, false
		}
		return *detail.CopyeditorAssigned, true
	})
	if err != nil {
		return err
	}
	for i := range cycle.Submissions {
		if assigned, ok := assignments[cycle.Submissions[i].OJSSubmissionID]; ok {
			value := assigned
			cycle.Submissions[i].CopyeditorAssigned = &value
		}
	}
	return nil
}

func copyeditingCandidates(cycle normalizer.NormalizedCycle) []string {
	ids := make([]string, 0)
	for _, submission := range cycle.Submissions {
		if submission.Status == statusCopyediting && submission.SubmissionProgress == 0 && hasSubmissionIdentity(submission) {
			ids = append(ids, submission.OJSSubmissionID)
		}
	}
	return ids
}

func copyeditingFindings(submission normalizer.Submission) []Finding {
	if submission.CopyeditorAssigned == nil || *submission.CopyeditorAssigned {
		return nil
	}
	return []Finding{{Kind: FindingMissingCopyeditor, Responsible: responsibleEditor}}
}
