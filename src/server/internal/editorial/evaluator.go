package editorial

import (
	"server/internal/normalizer"
	"sort"
	"time"
)

type FindingKind string

const (
	FindingMissingReviewer           FindingKind = "missing_reviewer"
	FindingOverdueReview             FindingKind = "overdue_review"
	FindingMissingRevisionIndication FindingKind = "missing_revision_indication"
	FindingUnsentReviewCompletion    FindingKind = "unsent_review_completion"
	FindingMissingEditor             FindingKind = "missing_editor"
	FindingMissingCopyeditor         FindingKind = "missing_copyeditor"
	FindingReviewsCompleteNoDecision FindingKind = "reviews_complete_no_decision"
	FindingOverdueReviewInvitation   FindingKind = "overdue_review_invitation"
	FindingRevisionPendingDecision   FindingKind = "revision_pending_decision"
	FindingAwaitingDeskDecision      FindingKind = "awaiting_desk_decision"
)

const statusCopyediting = "Copyediting"

type Finding struct {
	Kind        FindingKind `json:"kind"`
	Responsible string      `json:"responsible,omitempty"`
	Deadline    string      `json:"deadline,omitempty"`
	Contact     string      `json:"contact,omitempty"`
}

// responsibleEditor is the Finding.Responsible value for every finding a
// chief editor, not a reviewer or an author, owes an action for.
const responsibleEditor = "editor"

// findingsCapacityHint is a preallocation hint for a submission's finding
// list, not a cap: a submission can carry more findings than this.
const findingsCapacityHint = 6

type ArticleFinding struct {
	SubmissionID  string    `json:"submission_id"`
	Title         string    `json:"title,omitempty"`
	URL           string    `json:"url,omitempty"`
	DateSubmitted string    `json:"date_submitted"`
	Findings      []Finding `json:"findings"`
}

type DeliveryLookup interface {
	HasUnsentRN4(submissionID string) (unsent bool, evidenceKnown bool)
}

type Evaluator struct {
	RN11FallbackDays  int
	RequiredReviewers int
}

func (e Evaluator) Evaluate(cycle normalizer.NormalizedCycle, deliveries DeliveryLookup, now time.Time) []ArticleFinding {
	bySubmission := groupReviewsBySubmission(cycle.Reviews)
	result := make([]ArticleFinding, 0)
	for _, submission := range cycle.Submissions {
		if !isEvaluableSubmission(submission) {
			continue
		}
		reviews := bySubmission[submission.OJSSubmissionID]
		findings := e.submissionFindings(submission, reviews, deliveries, now.UTC())
		if len(findings) == 0 {
			continue
		}
		result = append(result, ArticleFinding{SubmissionID: submission.OJSSubmissionID, Title: submission.Title, URL: submission.URL, DateSubmitted: submission.DateSubmitted, Findings: findings})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].DateSubmitted < result[j].DateSubmitted })
	return result
}

func groupReviewsBySubmission(reviews []normalizer.ReviewAssignment) map[string][]normalizer.ReviewAssignment {
	bySubmission := make(map[string][]normalizer.ReviewAssignment)
	for _, review := range reviews {
		if review.SubmissionID != "" {
			bySubmission[review.SubmissionID] = append(bySubmission[review.SubmissionID], review)
		}
	}
	return bySubmission
}

// defaultRN11FallbackDays is the wait, after a revision decision with no
// later review round, before RN11 and RN16 fire, when the operator sets no
// override. RN5 does not wait.
const defaultRN11FallbackDays = 7

func (e Evaluator) fallbackDays() int {
	if e.RN11FallbackDays > 0 {
		return e.RN11FallbackDays
	}
	return defaultRN11FallbackDays
}

// defaultRequiredReviewers is requiredReviewers' floor when the operator
// sets no override.
const defaultRequiredReviewers = 2

// requiredReviewers is the floor an editor must never accept fewer than:
// the round's newest reviewer count that counts as working or done. It is a
// single default for now, not read from config, kept in this one place so a
// future operator setting only has to plug a value in here.
func (e Evaluator) requiredReviewers() int {
	if e.RequiredReviewers > 0 {
		return e.RequiredReviewers
	}
	return defaultRequiredReviewers
}
