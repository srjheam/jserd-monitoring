package events

import (
	"server/internal/normalizer"
	"testing"
	"time"
)

const (
	reviewRoundDueDate        = "2026-06-10"
	reviewCompletionDueDate   = "2026-08-30"
	singleReviewAssignmentKey = "assignment-1"
	reviewerUsername          = "reviewer"
	reviewSubmissionID        = "600433"
)

func TestTemporalRuleBoundaries(t *testing.T) {
	due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: "42", ReviewerUsername: reviewerUsername, ResponseDueDate: reviewRoundDueDate, ReviewDueDate: reviewRoundDueDate, RoundHasOverdueResponse: true}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}
	atReminder := Temporal(cycle, due.AddDate(0, 0, -2))
	if countEvent(atReminder, EventReviewResponseReminder) != 1 || countEvent(atReminder, EventReviewDueReminder) != 1 {
		t.Fatalf("reminders=%#v", atReminder)
	}
	afterDue := Temporal(cycle, due.Add(time.Second))
	if countEvent(afterDue, EventReviewResponseDue) != 1 || countEvent(afterDue, EventReviewDue) != 1 {
		t.Fatalf("overdue=%#v", afterDue)
	}
	for _, event := range append(atReminder, afterDue...) {
		if event.SubmissionID != "42" || event.OccurrenceKey == "" {
			t.Fatalf("unstable temporal event: %#v", event)
		}
		if event.Type == EventReviewResponseDue && event.TargetKind != "editor" {
			t.Fatalf("RN2 target=%#v", event)
		}
		if event.Type != EventReviewResponseDue && (event.TargetKind != reviewerUsername || event.TargetKey != reviewerUsername) {
			t.Fatalf("reviewer target=%#v", event)
		}
		if matches := MatchRules(event); len(matches) != 1 {
			t.Fatalf("temporal event without rule match: %#v", event)
		}
	}
	replay := Temporal(cycle, due.Add(time.Hour))
	if eventKey(afterDue, EventReviewDue) != eventKey(replay, EventReviewDue) {
		t.Fatal("scan time changed occurrence identity")
	}
	cycle.Reviews[0].DateConfirmed = "2026-06-09"
	cycle.Reviews[0].DateCompleted = "2026-06-09"
	if got := Temporal(cycle, due.Add(time.Hour)); len(got) != 0 {
		t.Fatalf("completed assignment emitted events: %#v", got)
	}
}

// TestTemporalParsesOJSReviewReportTimestamp covers the timestamp format
// OJS's ReviewReportPlugin CSV export writes due dates as
// "2006-01-02 15:04:05" (space-separated, no zone). parseDue did not
// recognize that layout, so RN2, RN6, RN7 and RN8 never fired against real
// collected data.
func TestTemporalParsesOJSReviewReportTimestamp(t *testing.T) {
	due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: "42", ReviewerUsername: reviewerUsername, ResponseDueDate: "2026-06-10 00:00:00", ReviewDueDate: "2026-06-10 00:00:00", RoundHasOverdueResponse: true}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}
	afterDue := Temporal(cycle, due.Add(time.Second))
	if countEvent(afterDue, EventReviewResponseDue) != 1 || countEvent(afterDue, EventReviewDue) != 1 {
		t.Fatalf("overdue=%#v, want one RN2 and one RN8 event for OJS's own timestamp format", afterDue)
	}
}

// TestTemporalRequiresRoundHasOverdueResponseForRN2 covers the case where OJS's CSV export always reports Declined as true, so
// RN2 cannot rely on it to tell "still deciding" apart from "already
// declined." Without a pre-Process enrichment stage setting
// RoundHasOverdueResponse from OJS's own reviewAssignment statusId, RN2
// must not fire even when the response due date has passed.
func TestTemporalRequiresRoundHasOverdueResponseForRN2(t *testing.T) {
	due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: "42", ReviewerUsername: reviewerUsername, ResponseDueDate: reviewRoundDueDate}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}
	if got := countEvent(Temporal(cycle, due.Add(time.Hour)), EventReviewResponseDue); got != 0 {
		t.Fatalf("RN2 fired without RoundHasOverdueResponse: count=%d", got)
	}
	cycle.Reviews[0].RoundHasOverdueResponse = true
	if got := countEvent(Temporal(cycle, due.Add(time.Hour)), EventReviewResponseDue); got != 1 {
		t.Fatalf("RN2 did not fire once RoundHasOverdueResponse was set: count=%d", got)
	}
}

// TestTemporalFiresRN2ReminderDespiteAlwaysTrueDeclined reproduces a
// The export behavior matters here: OJS's CSV export always reports Declined as
// true (see the RN2 comment above), so RN6 (review_response_reminder) must
// not gate on it either. Before this fix, RN6 stayed permanently disabled:
// live evidence found six accepted-invitation gaps this way, but the deeper
// gap was RN6 never firing at all, on any submission, in the whole
// collection history.
func TestTemporalFiresRN2ReminderDespiteAlwaysTrueDeclined(t *testing.T) {
	due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: "42", ReviewerUsername: reviewerUsername, ResponseDueDate: reviewRoundDueDate, Declined: true}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}
	if got := countEvent(Temporal(cycle, due.AddDate(0, 0, -2)), EventReviewResponseReminder); got != 1 {
		t.Fatalf("RN6 did not fire for an unconfirmed review despite OJS's always-true Declined flag: count=%d", got)
	}
}

func TestCandidatesNeedingOverdueReviewCheckMatchesRN2Eligibility(t *testing.T) {
	due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	now := due.Add(time.Hour)
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "42", ReviewerUsername: "r1", ResponseDueDate: reviewRoundDueDate},
		{Key: "a2", SubmissionID: "42", ReviewerUsername: "r2", ResponseDueDate: reviewRoundDueDate},
		{Key: "a3", SubmissionID: "43", ReviewerUsername: "r3", ResponseDueDate: reviewRoundDueDate, DateConfirmed: "2026-06-01"},
		{Key: "a4", SubmissionID: "44", ReviewerUsername: "r4", ResponseDueDate: "2026-07-10"},
	}}
	got := CandidatesNeedingOverdueReviewCheck(cycle, now)
	if len(got) != 1 || got[0] != "42" {
		t.Fatalf("candidates=%#v, want only [42] (deduplicated, excluding confirmed and not-yet-due reviews)", got)
	}
}

// TestTemporalSkipsSupersededReviewDueWindowForRN7AndRN8 reproduces the
// The export can carry a stale assignment: OJS's ReviewReportPlugin CSV export can carry
// a review-assignment row that is stale — cancelled, reassigned, or
// otherwise no longer pending in OJS's own live reviewAssignment state.
// Without a pre-Process enrichment stage setting ReviewDueWindowSuperseded
// from that live state, both RN7 (reminder) and RN8 (overdue) fire forever
// against a row OJS itself no longer considers open.
func TestTemporalSkipsSupersededReviewDueWindowForRN7AndRN8(t *testing.T) {
	due := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: reviewSubmissionID, ReviewerUsername: reviewerUsername, ReviewDueDate: reviewCompletionDueDate}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}

	if got := countEvent(Temporal(cycle, due.Add(-24*time.Hour)), EventReviewDueReminder); got != 1 {
		t.Fatalf("RN7 did not fire for a live, non-superseded window: count=%d", got)
	}
	if got := countEvent(Temporal(cycle, due.Add(time.Hour)), EventReviewDue); got != 1 {
		t.Fatalf("RN8 did not fire for a live, non-superseded window: count=%d", got)
	}

	cycle.Reviews[0].ReviewDueWindowSuperseded = true
	if got := countEvent(Temporal(cycle, due.Add(-24*time.Hour)), EventReviewDueReminder); got != 0 {
		t.Fatalf("RN7 fired for a superseded window: count=%d", got)
	}
	if got := countEvent(Temporal(cycle, due.Add(time.Hour)), EventReviewDue); got != 0 {
		t.Fatalf("RN8 fired for a superseded window: count=%d", got)
	}
}

// TestTemporalFiresRN8FromStartOfDueDay covers the case where the
// review report CSV export writes a due date as "23:59:59", but OJS's own
// reviewAssignment status already reports the review overdue from the first
// moment of that calendar day. RN8 must fire as soon as the due date's day
// begins, not only after it ends.
func TestTemporalFiresRN8FromStartOfDueDay(t *testing.T) {
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: "600452", ReviewerUsername: reviewerUsername, ReviewDueDate: "2026-09-09 23:59:59"}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}
	morningOfDueDay := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	if got := countEvent(Temporal(cycle, morningOfDueDay), EventReviewDue); got != 1 {
		t.Fatalf("RN8 did not fire in the morning of the due date: count=%d", got)
	}
	dayBefore := time.Date(2026, 9, 8, 23, 0, 0, 0, time.UTC)
	if got := countEvent(Temporal(cycle, dayBefore), EventReviewDue); got != 0 {
		t.Fatalf("RN8 fired the day before the due date: count=%d", got)
	}
}

func TestCandidatesNeedingReviewDueCheckMatchesRN7AndRN8Eligibility(t *testing.T) {
	due := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	now := due.Add(-24 * time.Hour)
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: reviewSubmissionID, ReviewerUsername: "r1", ReviewDueDate: reviewCompletionDueDate},
		{Key: "a2", SubmissionID: reviewSubmissionID, ReviewerUsername: "r2", ReviewDueDate: reviewCompletionDueDate},
		{Key: "a3", SubmissionID: "600434", ReviewerUsername: "r3", ReviewDueDate: reviewCompletionDueDate, DateCompleted: "2026-08-20"},
		{Key: "a4", SubmissionID: "600440", ReviewerUsername: "r4", ReviewDueDate: "2026-09-30"},
	}}
	got := CandidatesNeedingReviewDueCheck(cycle, now)
	if len(got) != 1 || got[0] != reviewSubmissionID {
		t.Fatalf("candidates=%#v, want only [600433] (deduplicated, excluding completed and not-yet-in-window reviews)", got)
	}
}

func countEvent(events []Event, kind string) int {
	count := 0
	for _, event := range events {
		if event.Type == kind {
			count++
		}
	}
	return count
}

func eventKey(events []Event, kind string) string {
	for _, event := range events {
		if event.Type == kind {
			return event.IdempotencyKey
		}
	}
	return ""
}

func TestTemporalReviewDueWindowDoesNotOverlapOverdueDay(t *testing.T) {
	review := normalizer.ReviewAssignment{Key: singleReviewAssignmentKey, SubmissionID: reviewSubmissionID, ReviewerUsername: reviewerUsername, ReviewDueDate: "2026-09-09 23:59:59"}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{review}}
	cases := []struct {
		name    string
		now     time.Time
		wantRN7 int
		wantRN8 int
	}{
		{name: "two days before due day before window", now: time.Date(2026, 9, 6, 23, 59, 59, 0, time.UTC)},
		{name: "start of reminder window", now: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), wantRN7: 1},
		{name: "day before due day", now: time.Date(2026, 9, 8, 23, 0, 0, 0, time.UTC), wantRN7: 1},
		{name: "start of due day", now: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), wantRN8: 1},
		{name: "morning of due day", now: time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC), wantRN8: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			events := Temporal(cycle, test.now)
			if got := countEvent(events, EventReviewDueReminder); got != test.wantRN7 {
				t.Fatalf("RN7 count=%d, want %d", got, test.wantRN7)
			}
			if got := countEvent(events, EventReviewDue); got != test.wantRN8 {
				t.Fatalf("RN8 count=%d, want %d", got, test.wantRN8)
			}
		})
	}
	if reviewDueWindowIsOpen(review, time.Date(2026, 9, 6, 23, 59, 59, 0, time.UTC)) {
		t.Fatal("candidate window opened before the RN7 window")
	}
	if !reviewDueWindowIsOpen(review, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("candidate window did not open at the RN7 window")
	}
}
