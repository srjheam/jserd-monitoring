package editorial

import (
	"context"
	"errors"
	"time"

	"github.com/pocketbase/pocketbase/tools/cron"
)

// This file holds the forced-send path: send now regardless of the schedule,
// then decide whether that send dismisses this week's occurrence.
// summary_schedule.go holds the forward-only path and the choice between them.

// forceSummary sends the summary now, regardless of the schedule, then applies
// the dismissal rule: a send strictly before this week's scheduled instant
// dismisses that occurrence and skips the schedule to next week's; a send at or
// after it leaves the schedule untouched, so next week's occurrence is
// unaffected.
func (c *Coordinator) forceSummary(ctx context.Context, cycleID string, now time.Time) error {
	if !c.scheduleConfigured() {
		return nil
	}
	instant, err := scheduledInstantForWeek(c.Schedule, c.location(), now)
	if err != nil {
		return err
	}
	// Read before the send, because the send takes time, and now must not
	// cross the scheduled instant in the middle of one decision.
	dismissesThisWeek := now.Before(instant)
	if err := c.deliver(ctx, cycleID); err != nil {
		return err
	}
	if !dismissesThisWeek {
		return nil
	}
	base, err := c.dismissalBase(instant)
	if err != nil {
		return err
	}
	return c.scheduleForward(base)
}

// dismissalBase is the instant to skip forward from: the stored due date when
// there is one, and this week's scheduled instant otherwise.
func (c *Coordinator) dismissalBase(instant time.Time) (time.Time, error) {
	due, ok, err := c.NextSummaryDueAt()
	if err != nil {
		return time.Time{}, err
	}
	if !ok || due.IsZero() {
		return instant, nil
	}
	return due, nil
}

// scheduledInstantForWeek returns the one occurrence of schedule that falls
// in the calendar week (Sunday 00:00 to the following Sunday 00:00, in loc)
// containing now. A schedule with more than one occurrence per week is out
// of scope; the first match found is returned.
func scheduledInstantForWeek(schedule *cron.Schedule, loc *time.Location, now time.Time) (time.Time, error) {
	nowInLoc := now.In(loc)
	// The Sunday 00:00 that starts the calendar week containing now, in loc.
	midnight := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 0, 0, 0, 0, loc)
	const daysPerWeek = 7
	candidate := midnight.AddDate(0, 0, -int(nowInLoc.Weekday()))
	end := candidate.AddDate(0, 0, daysPerWeek)
	for candidate.Before(end) {
		if schedule.IsDue(cron.NewMoment(candidate)) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, errors.New("no summary schedule occurrence found in the current week")
}
