package editorial

import (
	"context"
	"server/internal/normalizer"
	"server/internal/ojs"
	"time"
)

// OJSRevisionEvidenceEnricher is Coordinator's Enrich stage for
// missing_revision_indication (RN11) and revised_version_hint (RN5).
// It calls the SubmissionDetailProvider only for the
// submissions Evaluator.CandidatesNeedingRevisionEvidence names, and merges
// the result onto the cycle before Evaluate runs, so Evaluate itself never
// makes a live call.
type OJSRevisionEvidenceEnricher struct {
	Evaluator Evaluator
	Now       func() time.Time
}

func (e OJSRevisionEvidenceEnricher) Enrich(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	candidates := e.Evaluator.CandidatesNeedingRevisionEvidence(*cycle, nowOr(e.Now))
	if len(candidates) == 0 {
		return nil
	}
	pending, err := fetchDetails(ctx, details, candidates, ojs.SubmissionDetail.LatestRoundRevisionPending)
	if err != nil {
		return err
	}
	applyRevisionPendingEnrichment(cycle, pending)
	return nil
}

func applyRevisionPendingEnrichment(cycle *normalizer.NormalizedCycle, pending map[string]bool) {
	for i := range cycle.Submissions {
		if value, ok := pending[cycle.Submissions[i].OJSSubmissionID]; ok {
			v := value
			cycle.Submissions[i].RevisionPending = &v
		}
	}
}
