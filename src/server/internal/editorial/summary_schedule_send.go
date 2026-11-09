package editorial

import (
	"context"
	"errors"
	"time"

	"github.com/pocketbase/pocketbase/tools/cron"
)

// This file holds the mechanics both the natural and the forced summary-send
// path share: send the e-mail, then move the stored due date forward to the
// schedule's next future occurrence.

// sendIfNotStale decides what a due date at or before now means. It is either
// the schedule catching up naturally (no later occurrence has arrived yet) or a
// stale timestamp left behind by downtime or a restore (at least one later
// occurrence has already arrived too). Only the former sends. The latter
// silently reschedules, so recovering from an outage never triggers a backdated
// catch-up send.
func (c *Coordinator) sendIfNotStale(ctx context.Context, cycleID string, now, due time.Time) error {
	subsequent, err := nextOccurrence(c.Schedule, due.In(c.location()))
	if err != nil {
		return err
	}
	if !subsequent.After(now) {
		return c.scheduleForward(now)
	}
	if err := c.deliver(ctx, cycleID); err != nil {
		return err
	}
	return c.scheduleForward(now)
}

// deliver sends the editorial summary e-mail for one cycle, if delivery is
// wired.
func (c *Coordinator) deliver(ctx context.Context, cycleID string) error {
	if c.Delivery == nil {
		return nil
	}
	return c.Delivery.Deliver(ctx, cycleID)
}

func (c *Coordinator) scheduleForward(from time.Time) error {
	next, err := nextOccurrence(c.Schedule, from.In(c.location()))
	if err != nil {
		return err
	}
	return c.SetNextSummaryDueAt(next)
}

// nextOccurrence walks forward minute by minute from just after from,
// returning the next future occurrence of schedule.
func nextOccurrence(schedule *cron.Schedule, from time.Time) (time.Time, error) {
	candidate := from.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 60*24*370; i++ {
		if schedule.IsDue(cron.NewMoment(candidate)) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, errors.New("no summary schedule occurrence found")
}
