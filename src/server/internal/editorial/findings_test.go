package editorial

import (
	"testing"

	_ "server/migrations"
)

func TestRankReturnsTheWorstFindingKind(t *testing.T) {
	order := []FindingKind{FindingMissingEditor, FindingUnsentReviewCompletion, FindingMissingRevisionIndication, FindingOverdueReview, FindingMissingReviewer}
	for index, worst := range order {
		findings := make([]Finding, 0, len(order))
		for _, kind := range order[index:] {
			findings = append(findings, Finding{Kind: kind})
		}
		if got := Rank(findings); got != string(worst) {
			t.Fatalf("rank of %v = %q, want %q", findings, got, worst)
		}
		reversed := make([]Finding, 0, len(findings))
		for i := len(findings) - 1; i >= 0; i-- {
			reversed = append(reversed, findings[i])
		}
		if got := Rank(reversed); got != string(worst) {
			t.Fatalf("rank is not order independent: %q, want %q", got, worst)
		}
	}
	if got := Rank(nil); got != "normal" {
		t.Fatalf("empty rank=%q", got)
	}
	if got := Rank([]Finding{{Kind: FindingKind("something_else")}}); got != "normal" {
		t.Fatalf("unknown kind rank=%q", got)
	}
}

// TestRankKnowsEveryFindingKind reproduces a defect an outside review
// reported: overdue_review_invitation was added as a finding kind
// (evaluator.go) and to the client's badge order (work-list.ts) but never
// added to rankOrder here. Rank fell through to "normal" for a submission
// whose only finding was overdue_review_invitation, which also corrupted
// PriorityFor (sorted the row as if it carried no finding at all) and
// requiresAttention's SQL filter (excluded the row from any "needs
// attention" count). Every declared FindingKind must rank as itself, never
// as "normal".
func TestRankKnowsEveryFindingKind(t *testing.T) {
	declared := []FindingKind{
		FindingMissingReviewer,
		FindingOverdueReview,
		FindingMissingRevisionIndication,
		FindingUnsentReviewCompletion,
		FindingMissingEditor,
		FindingReviewsCompleteNoDecision,
		FindingOverdueReviewInvitation,
		FindingRevisionPendingDecision,
	}
	for _, kind := range declared {
		if got := Rank([]Finding{{Kind: kind}}); got != string(kind) {
			t.Fatalf("Rank(%q) = %q, want %q, not the FindingNormal fallback", kind, got, kind)
		}
	}
	if got := PriorityFor([]Finding{{Kind: FindingOverdueReviewInvitation}}); got == len(Kinds())+1 {
		t.Fatalf("PriorityFor(overdue_review_invitation alone) = %d, want a real rank, not the no-finding fallback", got)
	}
}

func TestLatestFindingsReadsEditorialFindingsDirectly(t *testing.T) {
	app := editorialTestApp(t)

	empty, err := LatestFindings(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("without any findings=%#v", empty)
	}

	store := NewStore(app)
	findings := []ArticleFinding{{SubmissionID: "42", Title: testSubmissionTitle, DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingOverdueReview, Contact: "reviewer@example.test"}, {Kind: FindingMissingEditor}}}}
	if err := store.ReplaceFindings("cycle-1", findings); err != nil {
		t.Fatal(err)
	}

	got, err := LatestFindings(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got["42"]) != 2 {
		t.Fatalf("findings=%#v", got)
	}
	if got["42"][0].Kind != FindingOverdueReview || got["42"][0].Contact != "reviewer@example.test" {
		t.Fatalf("first finding=%#v", got["42"][0])
	}
	if rank := Rank(got["42"]); rank != string(FindingMissingEditor) {
		t.Fatalf("rank=%q", rank)
	}

	byID, err := LatestFindingsFor(app, []string{"42"})
	if err != nil || len(byID) != 1 {
		t.Fatalf("byID=%#v err=%v", byID, err)
	}
	if none, err := LatestFindingsFor(app, nil); err != nil || len(none) != 0 {
		t.Fatalf("none=%#v err=%v", none, err)
	}
}

func TestLatestArticleFindingsIncludesTitleAndURL(t *testing.T) {
	app := editorialTestApp(t)
	store := NewStore(app)
	findings := []ArticleFinding{{SubmissionID: "42", Title: testSubmissionTitle, URL: testSubmissionURL, DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingMissingReviewer}}}}
	if err := store.ReplaceFindings("cycle-1", findings); err != nil {
		t.Fatal(err)
	}
	articles, err := LatestArticleFindings(app)
	if err != nil || len(articles) != 1 {
		t.Fatalf("articles=%#v err=%v", articles, err)
	}
	if articles[0].Title != testSubmissionTitle || articles[0].URL != testSubmissionURL || articles[0].DateSubmitted != testDate20260101 {
		t.Fatalf("article=%#v", articles[0])
	}
}
