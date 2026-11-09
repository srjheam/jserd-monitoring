package editorial

import (
	"context"
	"errors"
	"server/internal/normalizer"
	"time"
)

// This file holds Coordinator's publish half of a cycle: read the latest
// snapshot, enrich and evaluate it, store the findings, and hand off to
// afterFindings, which refreshes the dashboard and defers to
// summary_schedule.go for the summary e-mail.

// storeConfigured reports whether the editorial pipeline can store findings.
func (c *Coordinator) storeConfigured() bool {
	return c.Store != nil && c.App != nil
}

// publishFindings evaluates the findings of the latest snapshot, stores them,
// refreshes the dashboard, and checks the summary e-mail schedule.
func (c *Coordinator) publishFindings(ctx context.Context, details SubmissionDetailProvider, forceDigest bool) (Result, error) {
	if !c.storeConfigured() {
		return Result{}, errors.New("editorial pipeline is not configured")
	}
	c.pruneRetention()
	cycleID, findings, now, err := c.evaluateLatest(ctx, details)
	if err != nil {
		return Result{}, err
	}
	if err := c.Store.ReplaceFindings(cycleID, findings); err != nil {
		return Result{}, err
	}
	result := Result{CycleID: cycleID}
	c.setActive(result)
	return result, c.afterFindings(ctx, cycleID, now, c.summaryStepFor(forceDigest))
}

// pruneRetention deletes the artifacts retention no longer keeps. Retention is
// disk hygiene, not correctness: a failure here must not block or fail an
// otherwise-successful cycle.
func (c *Coordinator) pruneRetention() {
	if c.Prune == nil {
		return
	}
	if err := c.Prune(); err != nil {
		c.App.Logger().Error("retention prune failed", "error", err)
	}
}

// evaluateLatest computes the findings of the latest snapshot, and reports the
// moment they were computed at.
func (c *Coordinator) evaluateLatest(ctx context.Context, details SubmissionDetailProvider) (string, []ArticleFinding, time.Time, error) {
	cycle, cycleID, _, err := c.latestSnapshot()
	if err != nil {
		return "", nil, time.Time{}, err
	}
	if err := c.enrichCycle(ctx, &cycle, details); err != nil {
		return "", nil, time.Time{}, err
	}
	now := nowOr(c.Now)
	return cycleID, c.Evaluator.Evaluate(cycle, PocketBaseDeliveryLookup{App: c.App}, now), now, nil
}

// enrichCycle runs every Coordinator.Enrichers stage, in order, on the cycle
// Evaluate will read. Before this enrichment was added, only the RN5
// stage-file-count enricher ran here, so ReviewDueWindowSuperseded never reached the cycle Evaluate
// reads: that enricher only ran inside PreProcessEnrichers, against the
// separate, discarded collected.Normalized used for event generation.
func (c *Coordinator) enrichCycle(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	for _, stage := range c.Enrichers {
		if err := stage(ctx, cycle, details); err != nil {
			return err
		}
	}
	return nil
}

// afterFindings refreshes the dashboard, then runs the cycle's summary step.
func (c *Coordinator) afterFindings(ctx context.Context, cycleID string, now time.Time, summary summaryStep) error {
	if c.OnFindingsReady != nil {
		if err := c.OnFindingsReady(c.App, cycleID); err != nil {
			return err
		}
	}
	return summary(ctx, cycleID, now)
}
