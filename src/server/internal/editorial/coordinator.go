package editorial

import (
	"context"
	"errors"
	"server/internal/collector"
	"server/internal/normalizer"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
)

type RunSource string

const (
	RunSourceScheduled RunSource = "scheduled"
	RunSourceManual    RunSource = "manual"
)

var ErrRunConflict = errors.New("editorial pipeline already running")

type Result struct {
	CycleID  string
	Conflict bool
}

// EnrichmentStage is a Coordinator.Enrich or PreProcessEnrichers step: it
// fetches live OJS data a finding needs beyond the bulk-collected cycle,
// through the shared per-cycle SubmissionDetailProvider, and merges it onto
// cycle in place.
type EnrichmentStage func(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error

// Coordinator runs one cycle: collect from OJS, process the changes, compute
// findings, refresh the dashboard, and check the summary e-mail schedule.
// There is no separate weekly pass; every cycle does the same steps.
// findings_publish.go holds the publish half of that pipeline, and
// summary_schedule.go holds the forward-only summary schedule it checks at
// the end of every cycle.
type Coordinator struct {
	App      core.App
	Store    *Store
	Delivery interface {
		Deliver(context.Context, string) error
	}
	Evaluator           Evaluator
	Collect             func(context.Context) (collector.CycleResult, error)
	Process             func(collector.CycleResult) error
	Enrichers           []EnrichmentStage
	PreProcessEnrichers []EnrichmentStage
	OJSClient           SubmissionDetailSource
	OJSUsername         string
	OJSPassword         string
	CreatePending       func() error
	Prune               func() error
	OnFindingsReady     func(app core.App, cycleID string) error
	Schedule            *cron.Schedule
	Location            *time.Location
	Now                 func() time.Time
	NextSummaryDueAt    func() (time.Time, bool, error)
	SetNextSummaryDueAt func(time.Time) error
	mu                  sync.Mutex
	activeMu            sync.Mutex
	active              Result
}

// RunCycle collects, processes, evaluates findings, refreshes the dashboard,
// and checks whether the summary e-mail is due, unconditionally: none of
// that is conditional on source or forceDigest. forceDigest sends the
// summary e-mail now regardless of the schedule state; source and
// requestID record how the cycle was triggered, for audit and logging only.
func (c *Coordinator) RunCycle(ctx context.Context, source RunSource, requestID string, forceDigest bool) (Result, error) {
	if !c.mu.TryLock() {
		c.activeMu.Lock()
		conflict := c.active
		c.activeMu.Unlock()
		conflict.Conflict = true
		return conflict, ErrRunConflict
	}
	defer c.mu.Unlock()
	c.setActive(Result{})
	// One loader is built per cycle and shared by both enrichment stages
	// below, so a submission that both a PreProcessEnrichers stage and
	// Enrich want detail for is fetched from OJS at most once.
	details := c.newSubmissionDetailLoader()
	pendingErr, err := c.collectAndProcess(ctx, details)
	if err != nil {
		return Result{}, err
	}
	result, err := c.publishFindings(ctx, details, forceDigest)
	if err != nil {
		return result, err
	}
	return result, pendingErr
}

// collectionConfigured reports whether the collection pipeline is wired.
func (c *Coordinator) collectionConfigured() bool {
	return c.Collect != nil && c.Process != nil
}

// collectAndProcess collects one cycle from OJS and processes the changes it
// carries.
//
// It reports the pending-notification failure separately. A match whose
// recipient no longer resolves is independent of findings evaluation and must
// not block it, so one stuck match cannot leave editorial_findings stale
// forever.
func (c *Coordinator) collectAndProcess(ctx context.Context, details SubmissionDetailProvider) (pendingErr error, err error) {
	if !c.collectionConfigured() {
		return nil, errors.New("collection pipeline is not configured")
	}
	collected, err := c.Collect(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.enrichAndProcess(ctx, collected, details); err != nil {
		return nil, err
	}
	return c.createPending(), nil
}

// enrichAndProcess enriches the collected cycle, then processes it.
//
// PreProcessEnrichers run before Process, unlike Enrich: RN2 and RN7/RN8 are
// decided inside Process (events.Temporal), so their enrichment must land on
// collected.Normalized before Process reads it.
func (c *Coordinator) enrichAndProcess(ctx context.Context, collected collector.CycleResult, details SubmissionDetailProvider) error {
	if err := c.preProcessEnrichCycle(ctx, &collected.Normalized, details); err != nil {
		return err
	}
	return c.Process(collected)
}

// createPending records the notifications the new events call for.
func (c *Coordinator) createPending() error {
	if c.CreatePending == nil {
		return nil
	}
	return c.CreatePending()
}

func (c *Coordinator) preProcessEnrichCycle(ctx context.Context, cycle *normalizer.NormalizedCycle, details SubmissionDetailProvider) error {
	for _, stage := range c.PreProcessEnrichers {
		if err := stage(ctx, cycle, details); err != nil {
			return err
		}
	}
	return nil
}

// newSubmissionDetailLoader builds one per-cycle SubmissionDetailProvider,
// shared by PreProcessEnrichers and Enrich in this RunCycle call. It is nil
// when no OJSClient is configured (fixtures mode has no live OJS
// credentials), matching how Enrich/PreProcessEnrichers are already no-ops
// without live wiring.
func (c *Coordinator) newSubmissionDetailLoader() SubmissionDetailProvider {
	if c.OJSClient == nil {
		return nil
	}
	return NewSubmissionDetailLoader(c.OJSClient, c.OJSUsername, c.OJSPassword)
}

func (c *Coordinator) location() *time.Location {
	if c.Location != nil {
		return c.Location
	}
	return time.UTC
}

func (c *Coordinator) setActive(result Result) {
	c.activeMu.Lock()
	c.active = result
	c.activeMu.Unlock()
}
