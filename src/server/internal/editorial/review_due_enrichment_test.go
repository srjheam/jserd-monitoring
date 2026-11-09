package editorial

import (
	"context"
	"errors"
	"server/internal/normalizer"
	"server/internal/ojs"
	"testing"
	"time"
)

func TestReviewDueEnricherFetchesOnlyRN7RN8CandidatesAndMerges(t *testing.T) {
	// The live OJS detail endpoint's "due" field carries no time of day
	// ("2026-08-30"), unlike the CSV export's "Review Due Date"
	// ("2026-08-30 23:59:59") that ReviewDueDate below matches in
	// production: using a bare
	// date here for both sides would hide a key-format mismatch bug this
	// suite must catch.
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		"600433": mustSubmissionDetail(t, `{"reviewAssignments":[{"statusId":8,"round":1,"due":"2026-08-30"}]}`),
	}}
	enricher := OJSReviewDueEnricher{
		Now: func() time.Time { return time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC) },
	}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "600433", ReviewerUsername: "r1", ReviewDueDate: "2026-08-30 23:59:59", Round: 1},
		{Key: "a2", SubmissionID: "600433", ReviewerUsername: "r2", ReviewDueDate: "2026-08-21 23:59:59", Round: 1},
		{Key: "a3", SubmissionID: "600440", ReviewerUsername: "r3", ReviewDueDate: "2026-09-30 23:59:59", Round: 1},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 1 || provider.fetched[0] != "600433" {
		t.Fatalf("fetched=%#v, want only [600433]", provider.fetched)
	}
	if !cycle.Reviews[0].ReviewDueWindowSuperseded {
		t.Fatal("submission 600433 round 1 due 2026-08-30 has a live, resolved (statusId 8) match once dates are compared by calendar day, so it should be marked superseded")
	}
	if !cycle.Reviews[1].ReviewDueWindowSuperseded {
		t.Fatal("submission 600433 round 1 due 2026-08-21 has no matching live window at all, so it must be treated as superseded too, even though this specific row was not the candidate that triggered the fetch")
	}
	if cycle.Reviews[2].ReviewDueWindowSuperseded {
		t.Fatal("submission 600440 is not a candidate and must be untouched")
	}
}

// TestReviewDueEnricherKeepsAGenuinelyOpenWindowNotSuperseded reproduces the
// the format-mismatch case directly: one round-1 window is genuinely overdue and unresolved
// (statusId 6), which must stay not-superseded so overdue_review keeps
// firing for it, and a second round-1 window is accepted and not yet due
// (statusId 5), which must also stay not-superseded. Neither the CSV's
// ReviewDueDate (with a time of day) nor the live "due" field (without one)
// should stop them matching.
func TestReviewDueEnricherKeepsAGenuinelyOpenWindowNotSuperseded(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		"600447": mustSubmissionDetail(t, `{"reviewAssignments":[{"statusId":6,"round":1,"due":"2026-08-13"},{"statusId":5,"round":1,"due":"2026-10-05"}]}`),
	}}
	enricher := OJSReviewDueEnricher{
		Now: func() time.Time { return time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC) },
	}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "600447", ReviewerUsername: "r1", ReviewDueDate: "2026-08-13 23:59:59", Round: 1},
		{Key: "a2", SubmissionID: "600447", ReviewerUsername: "r2", ReviewDueDate: "2026-10-05 23:59:59", Round: 1},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if cycle.Reviews[0].ReviewDueWindowSuperseded {
		t.Fatal("a live, still-overdue-and-unresolved (statusId 6) reviewAssignment must not be marked superseded")
	}
	if cycle.Reviews[1].ReviewDueWindowSuperseded {
		t.Fatal("a live, still-pending (statusId 5) reviewAssignment sharing a different calendar due date must not be marked superseded")
	}
}

func TestReviewDueEnricherTreatsAMissingLiveWindowAsSuperseded(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{"600452": {}}}
	enricher := OJSReviewDueEnricher{
		Now: func() time.Time { return time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC) },
	}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "600452", ReviewerUsername: "r1", ReviewDueDate: "2026-08-27", Round: 1},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if !cycle.Reviews[0].ReviewDueWindowSuperseded {
		t.Fatal("a review whose (round, due) has no live reviewAssignment at all should be marked superseded")
	}
}

func TestReviewDueEnricherSkipsLoginWhenThereAreNoCandidates(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{}
	enricher := OJSReviewDueEnricher{Now: func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "42", ReviewerUsername: "r1", ReviewDueDate: testDate20260710},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 0 {
		t.Fatalf("fetched=%v, want none when there is nothing to fetch", provider.fetched)
	}
}

func TestReviewDueEnricherPropagatesLoginAndFetchFailures(t *testing.T) {
	cycle := func() normalizer.NormalizedCycle {
		return normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
			{Key: "a1", SubmissionID: "600433", ReviewerUsername: "r1", ReviewDueDate: "2026-08-30"},
		}}
	}
	now := func() time.Time { return time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC) }
	provider := &fakeSubmissionDetailProvider{fetchErr: errors.New("REST GET failed")}
	enricher := OJSReviewDueEnricher{Now: now}
	c := cycle()
	if err := enricher.Enrich(context.Background(), &c, provider); err == nil {
		t.Fatal("expected fetch failure to surface")
	}
}
