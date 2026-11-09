package editorial

import (
	"context"
	"encoding/json"
	"errors"
	"server/internal/normalizer"
	"server/internal/ojs"
	"testing"
	"time"
)

type fakeSubmissionDetailProvider struct {
	details  map[string]ojs.SubmissionDetail
	fetchErr error
	fetched  []string
}

func (f *fakeSubmissionDetailProvider) Get(_ context.Context, submissionID string) (ojs.SubmissionDetail, error) {
	f.fetched = append(f.fetched, submissionID)
	if f.fetchErr != nil {
		return ojs.SubmissionDetail{}, f.fetchErr
	}
	return f.details[submissionID], nil
}

func mustSubmissionDetail(t *testing.T, jsonBody string) ojs.SubmissionDetail {
	t.Helper()
	var detail ojs.SubmissionDetail
	if err := json.Unmarshal([]byte(jsonBody), &detail); err != nil {
		t.Fatalf("decode submission detail fixture: %v", err)
	}
	return detail
}

func TestEnricherFetchesOnlyRN5CandidatesAndMerges(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		"600348": mustSubmissionDetail(t, `{"reviewRounds":[{"round":2,"statusId":11}]}`),
	}}
	enricher := OJSRevisionEvidenceEnricher{
		Evaluator: Evaluator{},
		Now:       func() time.Time { return time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) },
	}
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: "600348", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}}}},
		{OJSSubmissionID: "600457", Status: statusReview, DateSubmitted: testDate20260101},
	}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 1 || provider.fetched[0] != "600348" {
		t.Fatalf("fetched=%#v, want only [600348]", provider.fetched)
	}
	if cycle.Submissions[0].RevisionPending == nil || *cycle.Submissions[0].RevisionPending {
		t.Fatalf("submission 600348 RevisionPending=%v, want observed and false (statusId 11)", cycle.Submissions[0].RevisionPending)
	}
	if cycle.Submissions[1].RevisionPending != nil {
		t.Fatalf("submission 600457 RevisionPending=%v, want untouched nil", cycle.Submissions[1].RevisionPending)
	}
}

func TestEnricherSkipsLoginWhenThereAreNoCandidates(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{}
	enricher := OJSRevisionEvidenceEnricher{Evaluator: Evaluator{}, Now: func() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) }}
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "600457", Status: statusReview, DateSubmitted: testDate20260101}}}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 0 {
		t.Fatalf("fetched=%v, want none when there is nothing to fetch", provider.fetched)
	}
}

func TestEnricherPropagatesLoginAndFetchFailures(t *testing.T) {
	cycle := func() normalizer.NormalizedCycle {
		return normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
			{OJSSubmissionID: "600348", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}}}},
		}}
	}
	now := func() time.Time { return time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC) }
	provider := &fakeSubmissionDetailProvider{fetchErr: errors.New("REST GET failed")}
	enricher := OJSRevisionEvidenceEnricher{Evaluator: Evaluator{}, Now: now}
	c := cycle()
	if err := enricher.Enrich(context.Background(), &c, provider); err == nil {
		t.Fatal("expected fetch failure to surface")
	}
}
