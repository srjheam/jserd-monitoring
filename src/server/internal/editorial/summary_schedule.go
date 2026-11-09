package editorial

import (
	"context"
	"time"
)

// This file holds the forward-only summary-e-mail schedule Coordinator checks
// at the end of every cycle, and the choice of which path a cycle takes.
//
// summary_schedule_forced.go holds the other path: a forced send, and what it
// does to the schedule. summary_schedule_send.go holds the mechanics both
// paths share: send the e-mail, then move the stored due date forward.

// summaryStep is what a cycle does about the summary e-mail once its findings
// are stored: check the schedule, or force a send now.
type summaryStep func(ctx context.Context, cycleID string, now time.Time) error

// summaryStepFor reads RunCycle's forceDigest once, here, so no function below
// this line carries the flag. It reads it from a table rather than a branch,
// the way scopeAliases and rn4Decisions read theirs.
func (c *Coordinator) summaryStepFor(forceDigest bool) summaryStep {
	return map[bool]summaryStep{
		false: c.applySchedule,
		true:  c.forceSummary,
	}[forceDigest]
}

// applySchedule checks the forward-only summary schedule at the end of a cycle.
// A schedule with no stored due date starts at the next occurrence and sends
// nothing now. A due date still in the future sends nothing. A due date at or
// before now hands off to sendIfNotStale, which tells a natural catch-up from a
// stale timestamp left behind by downtime.
func (c *Coordinator) applySchedule(ctx context.Context, cycleID string, now time.Time) error {
	if !c.scheduleConfigured() {
		return nil
	}
	due, known, err := c.NextSummaryDueAt()
	if err != nil {
		return err
	}
	if !known || due.IsZero() {
		return c.scheduleForward(now)
	}
	if due.After(now) {
		return nil
	}
	return c.sendIfNotStale(ctx, cycleID, now, due)
}

// scheduleConfigured reports whether the summary schedule is wired.
func (c *Coordinator) scheduleConfigured() bool {
	return c.Schedule != nil && c.NextSummaryDueAt != nil && c.SetNextSummaryDueAt != nil
}
