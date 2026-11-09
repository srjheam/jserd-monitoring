package editorial

import (
	"context"
	"fmt"
	"server/internal/diff"
	"server/internal/normalizer"
	"server/internal/ojs"
	"strings"
	"testing"
	"time"
)

// ruleCode reads the rule a matrix case belongs to out of its name. It cuts on
// the first space: slicing a fixed 4 characters instead silently missed every
// 3-character code, so no "RN9 ..." case asserted anything at all.
func ruleCode(name string) string {
	code, _, _ := strings.Cut(name, " ")
	return code
}

type deliveryEvidence struct {
	unsent map[string]bool
	known  map[string]bool
}

func (d deliveryEvidence) HasUnsentRN4(id string) (bool, bool) { return d.unsent[id], d.known[id] }

type evaluatorMatrixCase struct {
	name     string
	sub      normalizer.Submission
	reviews  []normalizer.ReviewAssignment
	delivery deliveryEvidence
	want     FindingKind
}

func evaluatorMatrixBase() normalizer.Submission {
	return normalizer.Submission{OJSSubmissionID: "42", Title: "Article", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Sequence: 1, GivenName: testEditorGivenName, Email: testEditorEmail}}}
}

// evaluatorMatrixWithOwnEditor returns a submission with its own editor
// slice. The RN11 cases set Decisions on evaluatorMatrixBase().Editors[0],
// and a struct copy of that submission shares the same editor array, so an
// RN15/RN16 case must use this instead or it inherits a decision it never
// declared.
func evaluatorMatrixWithOwnEditor() normalizer.Submission {
	s := evaluatorMatrixBase()
	s.Editors = []normalizer.Editor{{Sequence: 1, GivenName: testEditorGivenName, Email: testEditorEmail}}
	return s
}

func evaluatorMatrixCases() []evaluatorMatrixCase {
	return append(evaluatorMatrixCasesRN9ToRN16(), evaluatorMatrixCasesRN12ToRN17()...)
}

func evaluatorMatrixCasesRN9ToRN16() []evaluatorMatrixCase {
	return append(evaluatorMatrixCasesRN9AndRN10(), evaluatorMatrixCasesRN11AndRN16()...)
}

func evaluatorMatrixCasesRN9AndRN10() []evaluatorMatrixCase {
	base := evaluatorMatrixBase()
	review := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "42", ReviewerUsername: targetKindReviewer, DateConfirmed: testDate20260601, ReviewDueDate: testDate20260630}
	return []evaluatorMatrixCase{
		{"RN9 positive", base, nil, deliveryEvidence{}, FindingMissingReviewer},
		// A round must carry the required reviewer count (2 by default, see
		// requiredReviewers) before it counts as fully staffed: an editor
		// must never let a round settle for just one reviewer, even a
		// pending invitation or an accepted, not-yet-due reviewer.
		{"RN9 positive below required reviewer count, pending invitation", base, []normalizer.ReviewAssignment{{Key: "r", SubmissionID: "42", DateNotified: "2026-06-01"}}, deliveryEvidence{}, FindingMissingReviewer},
		// OJS's live "Reviews" export marks Declined and Unconsidered
		// "Yes" on every row unconditionally,
		// so an accepted, not-yet-due reviewer must still count as
		// active even though both flags read true — but one reviewer
		// alone is still short of the required count.
		{"RN9 positive below required reviewer count despite always-true Declined export quirk", base, []normalizer.ReviewAssignment{{Key: "r", SubmissionID: "42", DateConfirmed: "2026-06-01", ReviewDueDate: "2026-09-25", Declined: true, Unconsidered: true}}, deliveryEvidence{}, FindingMissingReviewer},
		{"RN9 insufficient id", func() normalizer.Submission { s := base; s.OJSSubmissionID = ""; return s }(), nil, deliveryEvidence{}, ""},
		{"RN10 positive", base, []normalizer.ReviewAssignment{review}, deliveryEvidence{}, FindingOverdueReview},
		{"RN10 negative completed", base, []normalizer.ReviewAssignment{func() normalizer.ReviewAssignment { r := review; r.DateCompleted = testDate20260629; return r }()}, deliveryEvidence{}, ""},
		{"RN10 insufficient due", base, []normalizer.ReviewAssignment{func() normalizer.ReviewAssignment { r := review; r.ReviewDueDate = ""; return r }()}, deliveryEvidence{}, ""},
		// A superseded window is a
		// CSV row OJS's own live state no longer treats as open: the
		// reviewer was replaced or the round moved on. It must not
		// raise overdue_review.
		{"RN10 negative superseded window", base, []normalizer.ReviewAssignment{func() normalizer.ReviewAssignment { r := review; r.ReviewDueWindowSuperseded = true; return r }()}, deliveryEvidence{}, ""},
	}
}

func evaluatorMatrixCasesRN11AndRN16() []evaluatorMatrixCase {
	base := evaluatorMatrixBase()
	withOwnEditor := evaluatorMatrixWithOwnEditor
	return []evaluatorMatrixCase{
		{"RN11 positive", func() normalizer.Submission {
			s := base
			s.RevisionPending = boolPtr(true)
			s.Editors[0].Decisions = []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260620}}
			return s
		}(), nil, deliveryEvidence{}, FindingMissingRevisionIndication},
		// OJS's own
		// review-round state says the revision already landed, so the
		// author-facing nudge must not fire — but the editor-facing one
		// (RN16) must, since nothing else in evaluator.go watches a
		// delivered revision the editor never decided on.
		{"RN11 negative revision submitted", func() normalizer.Submission {
			s := base
			s.RevisionPending = boolPtr(false)
			s.Editors[0].Decisions = []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260620}}
			return s
		}(), nil, deliveryEvidence{}, ""},
		{"RN16 positive", func() normalizer.Submission {
			s := withOwnEditor()
			s.RevisionPending = boolPtr(false)
			s.Editors[0].Decisions = []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260620}}
			return s
		}(), nil, deliveryEvidence{}, FindingRevisionPendingDecision},
		// A later review round means
		// the editor already acted by opening a new round, so RN16 must stay
		// silent even with a delivered revision and a stale deadline.
		{"RN16 negative later round already opened", func() normalizer.Submission {
			s := withOwnEditor()
			s.RevisionPending = boolPtr(false)
			s.Editors[0].Decisions = []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}
			return s
		}(), []normalizer.ReviewAssignment{{SubmissionID: "42", Round: 2, DateAssigned: testDate20260610}}, deliveryEvidence{}, ""},
		{"RN11 insufficient not observed", func() normalizer.Submission {
			s := base
			s.Editors[0].Decisions = []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260620}}
			return s
		}(), nil, deliveryEvidence{}, ""},
	}
}

func evaluatorMatrixCasesRN12ToRN17() []evaluatorMatrixCase {
	base := evaluatorMatrixBase()
	done1 := normalizer.ReviewAssignment{Key: "c1", SubmissionID: "42", Round: 1, DateConfirmed: testDate20260601, ReviewDueDate: testDate20260630, DateCompleted: testDate20260620}
	done2 := normalizer.ReviewAssignment{Key: "c2", SubmissionID: "42", Round: 1, DateConfirmed: testDate20260601, ReviewDueDate: testDate20260630, DateCompleted: "2026-06-22"}
	withOwnEditor := evaluatorMatrixWithOwnEditor
	return []evaluatorMatrixCase{
		{"RN12 positive", base, nil, deliveryEvidence{known: map[string]bool{"42": true}, unsent: map[string]bool{"42": true}}, FindingUnsentReviewCompletion},
		{"RN12 negative sent", base, nil, deliveryEvidence{known: map[string]bool{"42": true}}, ""},
		{"RN12 insufficient lookup", base, nil, deliveryEvidence{}, ""},
		{"RN13 positive", func() normalizer.Submission { s := base; s.Editors = nil; return s }(), nil, deliveryEvidence{}, FindingMissingEditor},
		{"RN13 negative", base, nil, deliveryEvidence{}, ""},
		{"RN13 insufficient editor", func() normalizer.Submission { s := base; s.Editors = []normalizer.Editor{{Sequence: 1}}; return s }(), nil, deliveryEvidence{}, ""},
		{"RN14 insufficient unsubmitted draft", func() normalizer.Submission { s := base; s.DateSubmitted = ""; s.Editors = nil; return s }(), nil, deliveryEvidence{}, ""},
		{"RN15 positive", withOwnEditor(), []normalizer.ReviewAssignment{done1, done2}, deliveryEvidence{}, FindingReviewsCompleteNoDecision},
		// The chief
		// editor already acted after the last review completed.
		{"RN15 negative decision followed", func() normalizer.Submission {
			s := withOwnEditor()
			s.Editors[0].Decisions = []normalizer.EditorDecision{{Decision: "Resubmit for Review", DateDecided: "2026-06-25"}}
			return s
		}(), []normalizer.ReviewAssignment{done1, done2}, deliveryEvidence{}, ""},
		// A case where round 1 holds an assignment the
		// reviewer never answered, and round 2 is complete. The finding
		// must still fire, so the rule reads the latest round alone.
		{"RN15 positive latest round complete despite open earlier round", withOwnEditor(), []normalizer.ReviewAssignment{
			{Key: testOldSubmissionID, SubmissionID: "42", Round: 1, DateAssigned: testDate20260201},
			func() normalizer.ReviewAssignment { r := done1; r.Round = 2; return r }(),
		}, deliveryEvidence{}, FindingReviewsCompleteNoDecision},
		{"RN15 negative open review in latest round", withOwnEditor(), []normalizer.ReviewAssignment{done1, func() normalizer.ReviewAssignment { r := done2; r.DateCompleted = ""; return r }()}, deliveryEvidence{}, ""},
		{"RN15 insufficient no review", withOwnEditor(), nil, deliveryEvidence{}, ""},
		// OJS still holds
		// these in the Submission stage, with no review round at all, so
		// missing_reviewer must not fire there. The chief editor owes a
		// send-to-review decision instead.
		{"RN17 positive submission stage reports desk decision not missing reviewer", func() normalizer.Submission {
			s := base
			s.Status = "Submission"
			return s
		}(), nil, deliveryEvidence{}, FindingAwaitingDeskDecision},
	}
}

func TestEvaluatorMatrix(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	for _, testCase := range evaluatorMatrixCases() {
		t.Run(testCase.name, func(t *testing.T) {
			assertEvaluatorMatrixCase(t, testCase, now)
		})
	}
}

func TestEvaluatorCopyeditingFindingRequiresKnownMissingAssignment(t *testing.T) {
	base := normalizer.Submission{OJSSubmissionID: "600438", Status: statusCopyediting, DateSubmitted: testDate20260101}
	assignedFalse := base
	assignedFalse.CopyeditorAssigned = boolPtr(false)
	assignedFalse.Editors = []normalizer.Editor{{Decisions: []normalizer.EditorDecision{{Decision: testDecisionAcceptSubmission, DateDecided: testDate20260601}}}}
	cases := []struct {
		name string
		sub  normalizer.Submission
		want FindingKind
	}{
		{"known missing copyeditor despite accepted decision", assignedFalse, FindingMissingCopyeditor},
		{"unknown assignment", base, ""},
		{"copyeditor assigned", func() normalizer.Submission { sub := base; sub.CopyeditorAssigned = boolPtr(true); return sub }(), ""},
		{"incomplete submission", func() normalizer.Submission { sub := assignedFalse; sub.SubmissionProgress = 1; return sub }(), ""},
		{"missing submission id", func() normalizer.Submission { sub := assignedFalse; sub.OJSSubmissionID = ""; return sub }(), ""},
		{"missing submission date", func() normalizer.Submission { sub := assignedFalse; sub.DateSubmitted = ""; return sub }(), ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{testCase.sub}}, noDeliveries{}, time.Now())
			if testCase.want == "" {
				if len(got) != 0 {
					t.Fatalf("findings=%#v, want none", got)
				}
				return
			}
			if len(got) != 1 || len(got[0].Findings) != 1 || got[0].Findings[0].Kind != testCase.want {
				t.Fatalf("findings=%#v, want only %q", got, testCase.want)
			}
		})
	}
}

func assertEvaluatorMatrixCase(t *testing.T, testCase evaluatorMatrixCase, now time.Time) {
	t.Helper()
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{testCase.sub}, Reviews: testCase.reviews}, testCase.delivery, now)
	if testCase.want == "" {
		checked := map[string]FindingKind{"RN9": FindingMissingReviewer, "RN10": FindingOverdueReview, "RN11": FindingMissingRevisionIndication, "RN12": FindingUnsentReviewCompletion, "RN13": FindingMissingEditor, "RN15": FindingReviewsCompleteNoDecision, "RN16": FindingRevisionPendingDecision}[ruleCode(testCase.name)]
		if len(got) > 0 && containsKind(got[0].Findings, checked) {
			t.Fatalf("unexpected %s in %#v", checked, got)
		}
		return
	}
	if len(got) != 1 || !containsKind(got[0].Findings, testCase.want) {
		t.Fatalf("got %#v, want %s", got, testCase.want)
	}
}

func TestEvaluatorGroupsAndSorts(t *testing.T) {
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{
		{OJSSubmissionID: "new", Status: statusReview, DateSubmitted: testDate20260201},
		{OJSSubmissionID: testOldSubmissionID, Status: statusReview, DateSubmitted: testDate20260101},
	}}
	got := (Evaluator{}).Evaluate(cycle, deliveryEvidence{}, now)
	if len(got) != 2 || got[0].SubmissionID != testOldSubmissionID {
		t.Fatalf("got %#v", got)
	}
	if len(got[0].Findings) != 2 {
		t.Fatalf("expected grouped RN9+RN13, got %#v", got[0])
	}
}

func TestEvaluatorRN11EndsAfterLaterDecisionOrRound(t *testing.T) {
	base := normalizer.Submission{OJSSubmissionID: "42", Status: statusReview, RevisionPending: boolPtr(true), Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}}}}
	cases := []struct {
		name    string
		sub     normalizer.Submission
		reviews []normalizer.ReviewAssignment
	}{
		{"later decision", func() normalizer.Submission {
			s := base
			s.Editors[0].Decisions = append(s.Editors[0].Decisions, normalizer.EditorDecision{Decision: testDecisionAcceptSubmission, DateDecided: testDate20260610})
			return s
		}(), nil},
		{"later round", base, []normalizer.ReviewAssignment{{SubmissionID: "42", Round: 2, DateAssigned: testDate20260610}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{testCase.sub}, Reviews: testCase.reviews}, deliveryEvidence{}, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
			if len(got) > 0 && containsKind(got[0].Findings, FindingMissingRevisionIndication) {
				t.Fatalf("unexpected RN11: %#v", got)
			}
		})
	}
}

// TestEvaluatorParsesOJSReviewReportTimestamp covers the export timestamp format:
// OJS's ReviewReportPlugin CSV export writes due dates as
// "2006-01-02 15:04:05" (space-separated, no zone), a format parseDate does
// not recognize. Every real overdue_review finding is silently dropped
// because of this, and the corresponding events/temporal.go parseDue has the
// same two-layout list and drops every RN2/RN6/RN7/RN8 event too.
func TestEvaluatorParsesOJSReviewReportTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "42", Title: "Article", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Sequence: 1, GivenName: testEditorGivenName, Email: testEditorEmail}}}
	review := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "42", ReviewerUsername: targetKindReviewer, DateConfirmed: "2026-06-01 09:00:00", ReviewDueDate: "2026-06-30 23:59:59"}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{review}}, deliveryEvidence{}, now)
	if len(got) != 1 || !containsKind(got[0].Findings, FindingOverdueReview) {
		t.Fatalf("got %#v, want overdue_review for a review due date in OJS's own export format", got)
	}
}

// TestCandidatesNeedingRevisionEvidenceMatchesEligibility reproduces the
// that missing_revision_indication and RN5 never fire
// because the bulk submissions list endpoint's schema has no review-round
// data. This candidate set must name exactly the submissions RN5/RN11 need
// live evidence for, and only those.
func TestCandidatesNeedingRevisionEvidenceMatchesEligibility(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	stale := normalizer.Submission{OJSSubmissionID: "600348", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}}}}
	notYetDue := normalizer.Submission{OJSSubmissionID: "600415", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260629}}}}}
	noDecision := normalizer.Submission{OJSSubmissionID: "600457", Status: statusReview, DateSubmitted: testDate20260101}
	laterRound := normalizer.Submission{OJSSubmissionID: "600429", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260601}}}}}
	cycle := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{stale, notYetDue, noDecision, laterRound},
		Reviews:     []normalizer.ReviewAssignment{{SubmissionID: "600429", Round: 2, DateAssigned: testDate20260610}},
	}
	got := (Evaluator{}).CandidatesNeedingRevisionEvidence(cycle, now)
	if len(got) != 2 || got[0] != "600348" || got[1] != "600415" {
		t.Fatalf("candidates=%#v, want [600348 600415]", got)
	}
}

// TestRevisionInsideWaitProducesRN5 reproduces defect B1: a revision that
// lands two days after the revision-request decision, inside the RN11 wait,
// must still produce RN5. The detail fetch starts on the decision date, so
// the previous snapshot already shows the revision as pending.
func TestRevisionInsideWaitProducesRN5(t *testing.T) {
	decision := []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260629}}}}
	sub := normalizer.Submission{OJSSubmissionID: "600415", Status: statusReview, DateSubmitted: testDate20260101, Editors: decision}
	day1 := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	previous := enrichRevision(t, sub, day1, 1)
	current := enrichRevision(t, sub, day2, 11)
	changes := diff.Compare(previous, current)
	if !hasChange(changes, diff.ChangeRN5Hint) {
		t.Fatalf("changes=%#v, want revised_version_hint for a revision inside the wait", changes)
	}
}

// TestRevisionPendingDecisionKeepsSevenDayWait guards the user's decision
// to keep the seven-day wait for RN16: a revision that arrived on day 2 does
// not yet produce revision_pending_decision.
func TestRevisionPendingDecisionKeepsSevenDayWait(t *testing.T) {
	pending := false
	sub := normalizer.Submission{OJSSubmissionID: "600415", Status: statusReview, DateSubmitted: testDate20260101, RevisionPending: &pending, Editors: []normalizer.Editor{{Email: testChiefEmail, Decisions: []normalizer.EditorDecision{{Decision: statusRevisionsRequired, DateDecided: testDate20260629}}}}}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}}, deliveryEvidence{}, now)
	if len(got) != 0 && containsKind(got[0].Findings, FindingRevisionPendingDecision) {
		t.Fatalf("got %#v, want no revision_pending_decision inside the wait", got)
	}
}

func enrichRevision(t *testing.T, sub normalizer.Submission, now time.Time, roundStatus int) normalizer.NormalizedCycle {
	t.Helper()
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		sub.OJSSubmissionID: mustSubmissionDetail(t, fmt.Sprintf(`{"reviewRounds":[{"round":1,"statusId":%d}]}`, roundStatus)),
	}}
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}}
	enricher := OJSRevisionEvidenceEnricher{Now: func() time.Time { return now }}
	if err := enricher.Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return cycle
}

func hasChange(changes []diff.Change, kind string) bool {
	for _, change := range changes {
		if change.Type == kind {
			return true
		}
	}
	return false
}

// TestMissingReviewerFiresDespiteOneActiveReviewer reproduces submission
// A round with two assigned reviewers, one accepted
// and active, one who never answered the invitation and is now past the
// response deadline. missing_reviewer must still fire, because OJS no
// longer counts the round as fully staffed, and overdue_review_invitation
// must also fire so a chief editor knows which reviewer to chase.
func TestMissingReviewerFiresDespiteOneActiveReviewer(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600418", Status: statusReview, DateSubmitted: testDate20260101}
	active := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "600418", Round: 3, ReviewerUsername: "r1", DateConfirmed: testDate20260901, ReviewDueDate: testDate20260925}
	overdueInvite := normalizer.ReviewAssignment{Key: "r2", SubmissionID: "600418", Round: 3, ReviewerUsername: "r2", RoundHasOverdueResponse: true}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{active, overdueInvite}}, deliveryEvidence{}, now)
	if len(got) != 1 || !containsKind(got[0].Findings, FindingMissingReviewer) {
		t.Fatalf("got %#v, want missing_reviewer despite an active reviewer", got)
	}
	if !containsKind(got[0].Findings, FindingOverdueReviewInvitation) {
		t.Fatalf("got %#v, want overdue_review_invitation alongside missing_reviewer", got)
	}
}

// TestMissingReviewerStaysSilentWhenRoundIsFullyActive is the regression
// guard for the fix above: a round where every assigned reviewer is active
// (none overdue on their invitation) must not gain missing_reviewer or
// overdue_review_invitation just because it has more than one reviewer.
func TestMissingReviewerStaysSilentWhenRoundIsFullyActive(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "42", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail}}}
	first := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "42", Round: 1, ReviewerUsername: "one", DateConfirmed: testDate20260901, ReviewDueDate: testDate20260925}
	second := normalizer.ReviewAssignment{Key: "r2", SubmissionID: "42", Round: 1, ReviewerUsername: "two", DateConfirmed: "2026-09-02", ReviewDueDate: testDate20260925}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{first, second}}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no findings for a fully active round", got)
	}
}

// TestMissingReviewerFiresBelowRequiredCount covers a round with
// round 4: one reviewer accepted, on time, with no overdue-response signal
// at all, but the journal never accepts a round with fewer than two
// reviewers. missing_reviewer must fire on the reviewer-count floor alone.
func TestMissingReviewerFiresBelowRequiredCount(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600362", Status: statusReview, DateSubmitted: testDate20260101}
	solo := normalizer.ReviewAssignment{Key: "sole-reviewer", SubmissionID: "600362", Round: 4, ReviewerUsername: "sole-reviewer", DateConfirmed: "2026-08-17", ReviewDueDate: "2026-09-12"}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{solo}}, deliveryEvidence{}, now)
	if len(got) != 1 || !containsKind(got[0].Findings, FindingMissingReviewer) {
		t.Fatalf("got %#v, want missing_reviewer with only one of two required reviewers assigned", got)
	}
}

// TestMissingReviewerIgnoresEarlierCompletedRound is the regression guard
// for the floor above: a round already fully completed must stay silent
// regardless of how many reviewers it carried, and must not let an earlier
// round's reviewer count leak into a later round's floor check.
func TestMissingReviewerIgnoresEarlierCompletedRound(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600362", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail, Decisions: []normalizer.EditorDecision{{Decision: testDecisionAcceptSubmission, DateDecided: "2026-02-05"}}}}}
	round1a := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "600362", Round: 1, DateConfirmed: testDate20260101, DateCompleted: testDate20260201}
	round1b := normalizer.ReviewAssignment{Key: "r2", SubmissionID: "600362", Round: 1, DateConfirmed: testDate20260101, DateCompleted: testDate20260201}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{round1a, round1b}}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no findings once every reviewer of the latest round is done", got)
	}
}

// TestMissingReviewerHonorsConfiguredRequiredCount confirms an operator can
// change the reviewer quorum by setting Evaluator.RequiredReviewers, without
// a code change.
func TestMissingReviewerHonorsConfiguredRequiredCount(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "99", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail}}}
	solo := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "99", Round: 1, ReviewerUsername: "one", DateConfirmed: testDate20260901, ReviewDueDate: testDate20260925}
	got := (Evaluator{RequiredReviewers: 1}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{solo}}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no missing_reviewer once the required count is configured down to 1", got)
	}
}

// TestOverdueReviewFiresFromStartOfDueDay covers the case where OJS's
// own reviewAssignment status already reports a review overdue from the
// first moment of its due date, not from the moment the day ends. The
// evaluator's boundary must match, so a review due earlier today counts as
// overdue even before 23:59:59.
func TestOverdueReviewFiresFromStartOfDueDay(t *testing.T) {
	sub := normalizer.Submission{OJSSubmissionID: "600452", Status: statusReview, DateSubmitted: testDate20260101, Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail}}}
	review := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "600452", ReviewerUsername: targetKindReviewer, DateConfirmed: "2026-08-05", ReviewDueDate: "2026-09-09"}
	second := normalizer.ReviewAssignment{Key: "r2", SubmissionID: "600452", ReviewerUsername: "other", DateConfirmed: "2026-08-05", ReviewDueDate: testDate20260925}
	morningOfDueDay := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{review, second}}, deliveryEvidence{}, morningOfDueDay)
	if len(got) != 1 || !containsKind(got[0].Findings, FindingOverdueReview) {
		t.Fatalf("got %#v, want overdue_review already true in the morning of the due date", got)
	}
	dayBefore := time.Date(2026, 9, 8, 23, 0, 0, 0, time.UTC)
	got = (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{review, second}}, deliveryEvidence{}, dayBefore)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no overdue_review the day before the due date", got)
	}
}

// TestHasOverdueResponseIgnoresClosedEarlierRound covers the case where
// round 1's reviewer never answered its invitation back in 2025 and OJS
// still reports that round's response overdue today, but round 2 opened
// since, with two reviewers who both finished on time. missing_reviewer and
// overdue_review_invitation must not fire on a closed round's stale signal.
func TestHasOverdueResponseIgnoresClosedEarlierRound(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600348", Status: statusReview, DateSubmitted: "2025-04-16", Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail, Decisions: []normalizer.EditorDecision{
		{Decision: "Request Revisions", DateDecided: "2026-05-07"},
	}}}}
	staleInvite := normalizer.ReviewAssignment{Key: "reviewer-a", SubmissionID: "600348", Round: 1, ReviewerUsername: "reviewer-a", RoundHasOverdueResponse: true}
	roundOneDone := normalizer.ReviewAssignment{Key: "r1a", SubmissionID: "600348", Round: 1, DateConfirmed: "2025-06-01", DateCompleted: "2025-06-19"}
	currentA := normalizer.ReviewAssignment{Key: "r2a", SubmissionID: "600348", Round: 2, DateConfirmed: "2026-01-29", DateCompleted: "2026-02-19"}
	currentB := normalizer.ReviewAssignment{Key: "r2b", SubmissionID: "600348", Round: 2, DateConfirmed: "2026-01-07", DateCompleted: "2026-03-19"}
	reviews := []normalizer.ReviewAssignment{staleInvite, roundOneDone, currentA, currentB}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: reviews}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no findings once the current round is fully staffed and on time", got)
	}
}

// TestCountWorkingReviewersCountsACompletedReviewDespiteSupersededWindow
// covers two submissions where two reviewers in the same round
// share a review due date, one already replaced with no live counterpart,
// the other genuinely completed. The due-date match cannot tell them apart,
// so both get marked ReviewDueWindowSuperseded — but a completed review
// must still count as a working reviewer regardless.
func TestCountWorkingReviewersCountsACompletedReviewDespiteSupersededWindow(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600440", Status: statusReview, DateSubmitted: "2026-05-31", Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail}}}
	replaced := normalizer.ReviewAssignment{Key: "reviewer-b", SubmissionID: "600440", Round: 1, DateConfirmed: "2026-07-02", ReviewDueDate: "2026-08-31", ReviewDueWindowSuperseded: true}
	completed := normalizer.ReviewAssignment{Key: "reviewer-c", SubmissionID: "600440", Round: 1, DateConfirmed: "2026-07-03", DateCompleted: "2026-08-29", ReviewDueDate: "2026-08-31", ReviewDueWindowSuperseded: true}
	active := normalizer.ReviewAssignment{Key: "reviewer-d", SubmissionID: "600440", Round: 1, DateConfirmed: testDate20260629, ReviewDueDate: "2026-09-30"}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{replaced, completed, active}}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no missing_reviewer once the completed reviewer counts despite the superseded window", got)
	}
}

// TestNoFindingsWithSurplusNoShowAboveFloor covers submissions where the round already has enough reviewers who answered or finished to
// meet the floor, plus one unrelated reviewer who never answered and is now
// overdue on the invitation. Neither missing_reviewer nor
// overdue_review_invitation must fire — the floor is met without the
// no-show, so the surplus invitation is not a chief editor's problem to
// chase (confirmed product decision).
func TestNoFindingsWithSurplusNoShowAboveFloor(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600456", Status: statusReview, DateSubmitted: "2026-08-07", Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail}}}
	active := normalizer.ReviewAssignment{Key: "r1", SubmissionID: "600456", Round: 1, DateConfirmed: "2026-08-18", ReviewDueDate: "2026-09-22"}
	done := normalizer.ReviewAssignment{Key: "completed-reviewer", SubmissionID: "600456", Round: 1, DateConfirmed: "2026-08-19", DateCompleted: "2026-08-28"}
	noShow := normalizer.ReviewAssignment{Key: "r2", SubmissionID: "600456", Round: 1, RoundHasOverdueResponse: true}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{active, done, noShow}}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no findings once the floor is met without the no-show", got)
	}
}

// TestMissingReviewerStaysSilentWhileInviteResponseDeadlineNotYetPassed
// covers a round 2 with one reviewer already accepted and
// working, one reviewer invited the same day whose response deadline is
// today, not yet missed. OJS has not flagged the round's response overdue
// (RoundHasOverdueResponse false). missing_reviewer must not fire: the
// second seat is still pending an answer within its own deadline, not
// genuinely missing (confirmed product decision).
func TestMissingReviewerStaysSilentWhileInviteResponseDeadlineNotYetPassed(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{OJSSubmissionID: "600415", Status: statusReview, DateSubmitted: "2026-03-06", Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail}}}
	accepted := normalizer.ReviewAssignment{Key: "reviewerA", SubmissionID: "600415", Round: 2, DateConfirmed: "2026-09-08", ReviewDueDate: "2026-10-08"}
	pending := normalizer.ReviewAssignment{Key: "reviewerB", SubmissionID: "600415", Round: 2, DateAssigned: "2026-09-03", RoundHasOverdueResponse: false}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{accepted, pending}}, deliveryEvidence{}, now)
	if len(got) != 0 {
		t.Fatalf("got %#v, want no findings while the second invitation's response deadline has not passed", got)
	}
}

// TestRevisionPendingDecisionCarriesNoDeadline covers the case where
// the fallback deadline is the date the author's reply to the original
// decision was due, which reads as nonsense once the revision already
// landed months later. revision_pending_decision must carry no deadline, the
// same as its sibling reviews_complete_no_decision.
func TestRevisionPendingDecisionCarriesNoDeadline(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	sub := normalizer.Submission{
		OJSSubmissionID: "600348", Status: statusReview, DateSubmitted: "2025-04-16",
		RevisionPending: boolPtr(false),
		Editors: []normalizer.Editor{{Sequence: 1, Email: testEditorEmail, Decisions: []normalizer.EditorDecision{
			{Decision: "Request Revisions", DateDecided: "2026-05-07"},
		}}},
	}
	got := (Evaluator{}).Evaluate(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}}, deliveryEvidence{}, now)
	if len(got) != 1 || !containsKind(got[0].Findings, FindingRevisionPendingDecision) {
		t.Fatalf("got %#v, want revision_pending_decision", got)
	}
	for _, finding := range got[0].Findings {
		if finding.Kind == FindingRevisionPendingDecision && finding.Deadline != "" {
			t.Fatalf("got deadline %q, want revision_pending_decision to carry no deadline", finding.Deadline)
		}
	}
}

func boolPtr(v bool) *bool { return &v }

func containsKind(items []Finding, kind FindingKind) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

// TestDeclinedReviewerDoesNotCountForRN9AndRN19 reproduces defect B2. OJS
// fills DateConfirmed on a decline too, so the Reviews export shows two
// answered rows. The OJS detail query omits the declined assignment and
// reports one active reviewer. RN9 (missing_reviewer) and RN19
// (overdue_review_invitation) must read that live count.
func TestDeclinedReviewerDoesNotCountForRN9AndRN19(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	active := 1
	sub := normalizer.Submission{OJSSubmissionID: "600418", Status: statusReview, DateSubmitted: testDate20260101, ActiveReviewers: &active}
	accepted := normalizer.ReviewAssignment{Key: "accepted", SubmissionID: "600418", Round: 1, DateConfirmed: testDate20260901, ReviewDueDate: testDate20260925}
	declined := normalizer.ReviewAssignment{Key: "declined", SubmissionID: "600418", Round: 1, DateConfirmed: testDate20260901, ReviewDueDate: testDate20260925}
	noShow := normalizer.ReviewAssignment{Key: "no-show", SubmissionID: "600418", Round: 1, RoundHasOverdueResponse: true}
	cycle := normalizer.NormalizedCycle{Submissions: []normalizer.Submission{sub}, Reviews: []normalizer.ReviewAssignment{accepted, declined, noShow}}
	got := (Evaluator{}).Evaluate(cycle, deliveryEvidence{}, now)
	if len(got) != 1 || !containsKind(got[0].Findings, FindingMissingReviewer) || !containsKind(got[0].Findings, FindingOverdueReviewInvitation) {
		t.Fatalf("got %#v, want missing_reviewer and overdue_review_invitation from the live count of 1", got)
	}
}

// TestActiveReviewersEnricherFetchesOpenReviewRounds names only submissions
// in Review with an incomplete review, and merges the live count onto them.
func TestActiveReviewersEnricherFetchesOpenReviewRounds(t *testing.T) {
	provider := &fakeSubmissionDetailProvider{details: map[string]ojs.SubmissionDetail{
		"600418": mustSubmissionDetail(t, `{"reviewRounds":[{"round":1}],"reviewAssignments":[{"round":1,"statusId":5}]}`),
	}}
	cycle := normalizer.NormalizedCycle{
		Submissions: []normalizer.Submission{
			{OJSSubmissionID: "600418", Status: statusReview, DateSubmitted: testDate20260101},
			{OJSSubmissionID: "8000", Status: statusReview, DateSubmitted: testDate20260101},
			{OJSSubmissionID: "8100", Status: "Submission", DateSubmitted: testDate20260101},
		},
		Reviews: []normalizer.ReviewAssignment{
			{Key: "a", SubmissionID: "600418", Round: 1},
			{Key: "b", SubmissionID: "8000", Round: 1, DateCompleted: testDate20260901},
			{Key: "c", SubmissionID: "8100", Round: 1},
		},
	}
	if err := (OJSActiveReviewersEnricher{}).Enrich(context.Background(), &cycle, provider); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if len(provider.fetched) != 1 || provider.fetched[0] != "600418" {
		t.Fatalf("fetched=%#v, want only [600418]", provider.fetched)
	}
	if got := cycle.Submissions[0].ActiveReviewers; got == nil || *got != 1 {
		t.Fatalf("600418 ActiveReviewers=%v, want 1", got)
	}
	if cycle.Submissions[1].ActiveReviewers != nil || cycle.Submissions[2].ActiveReviewers != nil {
		t.Fatalf("non-candidates were enriched: %#v", cycle.Submissions)
	}
}
