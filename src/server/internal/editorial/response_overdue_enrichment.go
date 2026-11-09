package editorial

import (
	"context"
	"server/internal/events"
	"server/internal/normalizer"
	"server/internal/ojs"
	"time"
)

// OJSResponseOverdueEnricher is Coordinator's PreProcessEnrichers stage for
// RN2. Unlike Coordinator.Enrich, it must run before
// Process, because Process is where events.Temporal decides RN2 — running
// after Process would be too late for the data to reach it. It calls the
// SubmissionDetailProvider only for the submissions
// events.CandidatesNeedingOverdueReviewCheck names, and merges the result
// onto the cycle before Process runs.
type OJSResponseOverdueEnricher struct {
	Now func() time.Time
}

func (e OJSResponseOverdueEnricher) Enrich(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	candidates := events.CandidatesNeedingOverdueReviewCheck(*cycle, nowOr(e.Now))
	if len(candidates) == 0 {
		return nil
	}
	overdueRounds, err := fetchDetails(ctx, details, candidates, always(ojs.SubmissionDetail.OverdueReviewRounds))
	if err != nil {
		return err
	}
	applyOverdueReviewRoundEnrichment(cycle, overdueRounds)
	return nil
}

func applyOverdueReviewRoundEnrichment(cycle *normalizer.NormalizedCycle, overdueRounds map[string]map[int]bool) {
	for i := range cycle.Reviews {
		rounds, ok := overdueRounds[cycle.Reviews[i].SubmissionID]
		if !ok {
			continue
		}
		cycle.Reviews[i].RoundHasOverdueResponse = rounds[cycle.Reviews[i].Round]
	}
}
