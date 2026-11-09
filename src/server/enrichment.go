package main

import (
	"server/internal/config"
	"server/internal/editorial"
	"server/internal/ojs"
)

// configureLiveEnrichment wires the enrichment stages that need live OJS
// access. Fixtures mode has no live OJS credentials to authenticate the
// detail-endpoint fetch with, so RN5's, RN2's, RN7/RN8's and RN18's
// enrichment stages only run live.
func configureLiveEnrichment(coordinator *editorial.Coordinator, cfg config.Config, ojsClient *ojs.Client, evaluator editorial.Evaluator) {
	if cfg.SourceMode != config.SourceModeLive {
		return
	}

	coordinator.OJSClient = ojsClient
	coordinator.OJSUsername = cfg.Username
	coordinator.OJSPassword = cfg.Password
	revisionEvidenceEnricher := editorial.OJSRevisionEvidenceEnricher{Evaluator: evaluator}
	reviewDueEnricher := editorial.OJSReviewDueEnricher{}
	responseOverdueEnricher := editorial.OJSResponseOverdueEnricher{}
	copyeditingEnricher := editorial.OJSCopyeditingEnricher{}
	// reviewDueEnricher and responseOverdueEnricher also run here, not
	// only inside PreProcessEnrichers below: overdue_review and
	// missing_reviewer/overdue_review_invitation (evaluator.go) need
	// ReviewDueWindowSuperseded and RoundHasOverdueResponse on the cycle
	// Evaluate reads, which is a separate, later-reloaded cycle than the
	// one PreProcessEnrichers enriches for event generation.
	// It shares the one per-cycle SubmissionDetailLoader built below, so a
	// submission already fetched by PreProcessEnrichers is not fetched
	// from OJS again here.
	coordinator.Enrichers = []editorial.EnrichmentStage{
		reviewDueEnricher.Enrich,
		responseOverdueEnricher.Enrich,
		// RN9 and RN19 count active reviewers from OJS's live list, which
		// omits declined assignments. Only Evaluate reads the count.
		editorial.OJSActiveReviewersEnricher{}.Enrich,
	}
	// RN2 and RN7/RN8 are decided inside Process (events.Temporal), while
	// RN18 compares snapshots there, so their evidence must land on
	// collected.Normalized before Process runs. revisionEvidenceEnricher also
	// runs here, not in Enrichers above: RN5 (revised_version_hint, diff.go) and
	// missing_revision_indication (evaluator.go) both read
	// RevisionPending off the *saved* snapshot, and only what runs before
	// Process ends up saved. No dashboard code reads
	// RevisionPending, so persisting it here only affects RN5 and RN11. All four stages
	// share the one per-cycle SubmissionDetailLoader Coordinator.RunCycle
	// builds from OJSClient.
	coordinator.PreProcessEnrichers = []editorial.EnrichmentStage{
		responseOverdueEnricher.Enrich,
		reviewDueEnricher.Enrich,
		revisionEvidenceEnricher.Enrich,
		copyeditingEnricher.Enrich,
	}
}
