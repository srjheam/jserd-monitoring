package editorial

import (
	"context"
	"errors"
	"server/internal/normalizer"
	"server/internal/ojs"
	"testing"
	"time"
)

func TestResponseOverdueEnricherFetchesOnlyRN2CandidatesAndMerges(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		"42": mustSubmissionDetail(t, `{"reviewAssignments":[{"statusId":4,"round":1}]}`),
	}}
	enricher := OJSResponseOverdueEnricher{
		Now: func() time.Time { return time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC) },
	}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "42", ReviewerUsername: "r1", ResponseDueDate: testDate20260610, Round: 1},
		{Key: "a2", SubmissionID: "42", ReviewerUsername: "r2", ResponseDueDate: testDate20260610, Round: 2},
		{Key: "a3", SubmissionID: "99", ReviewerUsername: "r3", ResponseDueDate: testDate20260710, Round: 1},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 1 || provider.fetched[0] != "42" {
		t.Fatalf("fetched=%#v, want only [42]", provider.fetched)
	}
	if !cycle.Reviews[0].RoundHasOverdueResponse {
		t.Fatal("submission 42 round 1 should be marked overdue")
	}
	if cycle.Reviews[1].RoundHasOverdueResponse {
		t.Fatal("submission 42 round 2 was not reported overdue and must stay false")
	}
	if cycle.Reviews[2].RoundHasOverdueResponse {
		t.Fatal("submission 99 is not a candidate and must be untouched")
	}
}

func TestResponseOverdueEnricherSkipsLoginWhenThereAreNoCandidates(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{}
	enricher := OJSResponseOverdueEnricher{Now: func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }}
	cycle := normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
		{Key: "a1", SubmissionID: "42", ReviewerUsername: "r1", ResponseDueDate: testDate20260710},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 0 {
		t.Fatalf("fetched=%v, want none when there is nothing to fetch", provider.fetched)
	}
}

func TestResponseOverdueEnricherPropagatesLoginAndFetchFailures(t *testing.T) {
	cycle := func() normalizer.NormalizedCycle {
		return normalizer.NormalizedCycle{Reviews: []normalizer.ReviewAssignment{
			{Key: "a1", SubmissionID: "42", ReviewerUsername: "r1", ResponseDueDate: testDate20260610},
		}}
	}
	now := func() time.Time { return time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC) }
	provider := &fakeSubmissionDetailProvider{fetchErr: errors.New("REST GET failed")}
	enricher := OJSResponseOverdueEnricher{Now: now}
	c := cycle()
	if err := enricher.Enrich(context.Background(), &c, provider); err == nil {
		t.Fatal("expected fetch failure to surface")
	}
}
