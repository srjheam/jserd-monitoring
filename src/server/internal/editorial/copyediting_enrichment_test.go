package editorial

import (
	"context"
	"errors"
	"server/internal/normalizer"
	"server/internal/ojs"
	"testing"
)

func TestCopyeditingEnricherFetchesCompletedCopyeditingSubmissionsAndMergesKnownValues(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		"600434": {CopyeditorAssigned: boolPtr(false)},
		"600431": {CopyeditorAssigned: boolPtr(true)},
		"600418": {},
	}}
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: "600434", Status: statusCopyediting, DateSubmitted: testDate20260101},
		{OJSSubmissionID: "600431", Status: statusCopyediting, DateSubmitted: testDate20260101},
		{OJSSubmissionID: "600418", Status: statusCopyediting, DateSubmitted: testDate20260101},
		{OJSSubmissionID: "600362", Status: statusCopyediting, DateSubmitted: testDate20260101, SubmissionProgress: 1},
		{OJSSubmissionID: "review", Status: statusReview, DateSubmitted: testDate20260101},
		{OJSSubmissionID: "no-date", Status: statusCopyediting},
		{Status: statusCopyediting, DateSubmitted: testDate20260101},
	}}
	if err := (OJSCopyeditingEnricher{}).Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 3 || provider.fetched[0] != "600434" || provider.fetched[1] != "600431" || provider.fetched[2] != "600418" {
		t.Fatalf("fetched=%#v, want completed Copyediting IDs [600434 600431 600418]", provider.fetched)
	}
	if cycle.Submissions[0].CopyeditorAssigned == nil || *cycle.Submissions[0].CopyeditorAssigned {
		t.Fatalf("600434 CopyeditorAssigned=%v, want known false", cycle.Submissions[0].CopyeditorAssigned)
	}
	if cycle.Submissions[1].CopyeditorAssigned == nil || !*cycle.Submissions[1].CopyeditorAssigned {
		t.Fatalf("600431 CopyeditorAssigned=%v, want known true", cycle.Submissions[1].CopyeditorAssigned)
	}
	for _, index := range []int{2, 3, 4, 5, 6} {
		if cycle.Submissions[index].CopyeditorAssigned != nil {
			t.Fatalf("submission %q CopyeditorAssigned=%v, want unknown", cycle.Submissions[index].OJSSubmissionID, cycle.Submissions[index].CopyeditorAssigned)
		}
	}
}

func TestCopyeditingEnricherSkipsDetailReadsWithoutEligibleSubmissions(t *testing.T) {
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: "600362", Status: statusCopyediting, DateSubmitted: testDate20260101, SubmissionProgress: 1},
		{OJSSubmissionID: "review", Status: statusReview, DateSubmitted: testDate20260101},
	}}
	provider := &fakeSubmissionDetailProvider{}
	if err := (OJSCopyeditingEnricher{}).Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 0 {
		t.Fatalf("fetched=%v, want none", provider.fetched)
	}
}

func TestCopyeditingEnricherPropagatesDetailFailures(t *testing.T) {
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: "600434", Status: statusCopyediting, DateSubmitted: testDate20260101}}}
	provider := &fakeSubmissionDetailProvider{fetchErr: errors.New("REST GET failed")}
	if err := (OJSCopyeditingEnricher{}).Enrich(context.Background(), &cycle, provider); err == nil {
		t.Fatal("expected detail failure to surface")
	}
}
