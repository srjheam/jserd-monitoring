package editorial

import (
	"server/internal/normalizer"
	"testing"
	"time"
)

// TestCatalogOrder pins the relative order of the kinds whose ranking the work
// list used to assert for itself, in tests/work-list.test.tsx. The client no
// longer ranks anything: it renders the order the server ships.
func TestCatalogOrder(t *testing.T) {
	for _, pair := range []struct{ worse, milder FindingKind }{
		{FindingMissingEditor, FindingMissingCopyeditor},
		{FindingMissingCopyeditor, FindingUnsentReviewCompletion},
		{FindingUnsentReviewCompletion, FindingReviewsCompleteNoDecision},
		{FindingReviewsCompleteNoDecision, FindingRevisionPendingDecision},
		{FindingRevisionPendingDecision, FindingMissingRevisionIndication},
		{FindingMissingRevisionIndication, FindingOverdueReview},
		{FindingOverdueReview, FindingMissingReviewer},
		{FindingMissingReviewer, FindingOverdueReviewInvitation},
		{FindingOverdueReviewInvitation, FindingAwaitingDeskDecision},
	} {
		if rankOf(pair.worse) >= rankOf(pair.milder) {
			t.Fatalf("%s must rank worse than %s", pair.worse, pair.milder)
		}
	}
}

// An unknown kind sorts last and never displaces a declared one.
func TestUnknownKindRanksLast(t *testing.T) {
	if rankOf("not_a_kind") != len(catalog) {
		t.Fatalf("rankOf(unknown) = %d, want %d", rankOf("not_a_kind"), len(catalog))
	}
	if got := Rank([]Finding{{Kind: "not_a_kind"}}); got != string(FindingNormal) {
		t.Fatalf("Rank(unknown alone) = %q, want %q", got, FindingNormal)
	}
}

// TestEvaluateReturnsFindingsWorstFirst is the contract the client depends on:
// the dashboard renders the badges in the order they arrive, and elapsed() reads
// findings[0] for the deadline it shows.
func TestEvaluateReturnsFindingsWorstFirst(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// No editor assigned (missing_editor, the worst kind) and no reviewer
	// (missing_reviewer, a mild one). They are produced in the opposite order:
	// reviewStageFindings runs before editorialFindings.
	cycle := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{{
			OJSSubmissionID: "1",
			DateSubmitted:   testDate20260101,
			Status:          statusReview,
		}},
	}
	got := Evaluator{}.Evaluate(cycle, noDeliveries{}, now)
	if len(got) != 1 {
		t.Fatalf("Evaluate returned %d articles, want 1", len(got))
	}
	findings := got[0].Findings
	if len(findings) < 2 {
		t.Fatalf("got %#v, want at least two findings", findings)
	}
	for i := 1; i < len(findings); i++ {
		if rankOf(findings[i-1].Kind) > rankOf(findings[i].Kind) {
			t.Fatalf("findings are not worst first: %#v", findings)
		}
	}
	if findings[0].Kind != FindingMissingEditor {
		t.Fatalf("findings[0] = %s, want %s", findings[0].Kind, FindingMissingEditor)
	}
}

type noDeliveries struct{}

func (noDeliveries) HasUnsentRN4(string) (bool, bool) { return false, false }
