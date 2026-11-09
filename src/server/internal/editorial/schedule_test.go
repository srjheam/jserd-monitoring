package editorial

import (
	"context"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tools/cron"
)

// weeklyMonday9am matches SUMMARY_CRON's default (0 9 * * 1).
func weeklyMonday9am(t *testing.T) *cron.Schedule {
	t.Helper()
	schedule, err := cron.NewSchedule("0 9 * * 1")
	if err != nil {
		t.Fatal(err)
	}
	return schedule
}

func TestNextOccurrenceWalksForward(t *testing.T) {
	schedule := weeklyMonday9am(t)
	cases := []struct {
		name string
		from time.Time
		want time.Time
	}{
		{"one minute before due", time.Date(2026, 6, 29, 8, 59, 0, 0, time.UTC), time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)},
		{"at the due minute excludes itself", time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC), time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)},
		{"past-due of any age", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextOccurrence(schedule, tc.from)
			if err != nil || !got.Equal(tc.want) {
				t.Fatalf("got=%v err=%v want=%v", got, err, tc.want)
			}
		})
	}
}

func TestScheduledInstantForWeek(t *testing.T) {
	schedule := weeklyMonday9am(t)
	cases := []struct {
		name string
		now  time.Time
	}{
		{"Sunday before the instant", time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)},
		{"Tuesday after the instant", time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)},
	}
	want := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scheduledInstantForWeek(schedule, time.UTC, tc.now)
			if err != nil || !got.Equal(want) {
				t.Fatalf("got=%v err=%v want=%v", got, err, want)
			}
		})
	}
}

// scheduleCoordinator builds a Coordinator with only the fields applySchedule
// needs, with no App or Store: the schedule logic is tested independent of
// any database.
func scheduleCoordinator(t *testing.T, now time.Time, due time.Time, ok bool) (*Coordinator, *spyDelivery, *fakeSchedule) {
	t.Helper()
	delivery := &spyDelivery{}
	sched := &fakeSchedule{due: due, ok: ok}
	coordinator := &Coordinator{
		Delivery:            delivery,
		Schedule:            weeklyMonday9am(t),
		Now:                 func() time.Time { return now },
		NextSummaryDueAt:    sched.Next,
		SetNextSummaryDueAt: sched.Set,
	}
	return coordinator, delivery, sched
}

func TestApplyScheduleUnsetDueTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	coordinator, delivery, sched := scheduleCoordinator(t, now, time.Time{}, false)
	if err := coordinator.applySchedule(context.Background(), fieldCycle, now); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 0 {
		t.Fatalf("delivery calls=%d, want no send for an unset schedule", delivery.calls.Load())
	}
	want := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	if len(sched.sets) != 1 || !sched.sets[0].Equal(want) {
		t.Fatalf("sets=%v want=[%v]", sched.sets, want)
	}
}

func TestApplyScheduleNaturallyDueSends(t *testing.T) {
	now := time.Date(2026, 6, 29, 9, 5, 0, 0, time.UTC)
	due := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	coordinator, delivery, sched := scheduleCoordinator(t, now, due, true)
	if err := coordinator.applySchedule(context.Background(), fieldCycle, now); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 1 {
		t.Fatalf("delivery calls=%d, want exactly one send", delivery.calls.Load())
	}
	want := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	if len(sched.sets) != 1 || !sched.sets[0].Equal(want) {
		t.Fatalf("sets=%v want=[%v]", sched.sets, want)
	}
}

func TestApplyScheduleStaleDueTimestampOfAnyAgeSkipsSilently(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	for _, age := range []string{"one week stale", "months stale"} {
		t.Run(age, func(t *testing.T) {
			due := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
			if age == "months stale" {
				due = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
			}
			coordinator, delivery, sched := scheduleCoordinator(t, now, due, true)
			if err := coordinator.applySchedule(context.Background(), fieldCycle, now); err != nil {
				t.Fatal(err)
			}
			if delivery.calls.Load() != 0 {
				t.Fatalf("delivery calls=%d, a stale due timestamp must never trigger a backdated catch-up send", delivery.calls.Load())
			}
			want := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
			if len(sched.sets) != 1 || !sched.sets[0].Equal(want) {
				t.Fatalf("sets=%v want=[%v]", sched.sets, want)
			}
		})
	}
}

func TestApplyScheduleNotYetDueLeavesScheduleUntouched(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	coordinator, delivery, sched := scheduleCoordinator(t, now, due, true)
	if err := coordinator.applySchedule(context.Background(), fieldCycle, now); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 0 || len(sched.sets) != 0 {
		t.Fatalf("delivery calls=%d sets=%v, want no action while not yet due", delivery.calls.Load(), sched.sets)
	}
}

func TestApplyScheduleForcedBeforeInstantDismissesAndSkipsForward(t *testing.T) {
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC) // Sunday, before Monday 09:00
	due := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	coordinator, delivery, sched := scheduleCoordinator(t, now, due, true)
	if err := coordinator.forceSummary(context.Background(), fieldCycle, now); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 1 {
		t.Fatalf("delivery calls=%d, want exactly one send", delivery.calls.Load())
	}
	want := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	if len(sched.sets) != 1 || !sched.sets[0].Equal(want) {
		t.Fatalf("sets=%v want=[%v], the dismissed occurrence must be skipped forward", sched.sets, want)
	}
}

func TestApplyScheduleForcedAfterInstantLeavesNextWeekUntouched(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC) // Tuesday, after Monday 09:00
	due := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	coordinator, delivery, sched := scheduleCoordinator(t, now, due, true)
	if err := coordinator.forceSummary(context.Background(), fieldCycle, now); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 1 {
		t.Fatalf("delivery calls=%d, want exactly one send", delivery.calls.Load())
	}
	if len(sched.sets) != 0 {
		t.Fatalf("sets=%v, an ad hoc send after the instant must not pull next week's occurrence earlier", sched.sets)
	}
}

func TestApplyScheduleForcedExactlyAtInstantDoesNotDismiss(t *testing.T) {
	// "Strictly before" excludes the exact scheduled minute itself: a forced
	// send that lands precisely on the instant behaves like an after-instant
	// send, leaving the schedule untouched.
	now := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	due := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)
	coordinator, delivery, sched := scheduleCoordinator(t, now, due, true)
	if err := coordinator.forceSummary(context.Background(), fieldCycle, now); err != nil {
		t.Fatal(err)
	}
	if delivery.calls.Load() != 1 {
		t.Fatalf("delivery calls=%d, want exactly one send", delivery.calls.Load())
	}
	if len(sched.sets) != 0 {
		t.Fatalf("sets=%v, a forced send exactly at the scheduled minute must not dismiss it", sched.sets)
	}
}
