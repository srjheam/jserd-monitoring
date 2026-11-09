package editorial

import (
	"server/internal/normalizer"
	"time"
)

// This file composes the review-stage findings and holds the overdue-date
// predicate they need. reviewer_staffing.go holds the round-staffing
// counting reviewerFindings itself builds on.

// reviewStageFindings reports missing_reviewer, overdue_review_invitation,
// and overdue_review, none of which a chief editor can act on before OJS's
// own "Send to Review" action has run. A submission still sitting in the
// Submission stage carries no review round at all, so it reports
// awaiting_desk_decision instead: the decision a chief editor actually owes
// there.
func reviewStageFindings(sub normalizer.Submission, reviews []normalizer.ReviewAssignment, requiredReviewers int, now time.Time) []Finding {
	if sub.Status == "Submission" {
		return []Finding{{Kind: FindingAwaitingDeskDecision, Responsible: responsibleEditor}}
	}
	// The two findings reviewerFindings can report, plus overdueReviewFinding's one.
	const capacityHint = 3
	findings := make([]Finding, 0, capacityHint)
	findings = append(findings, reviewerFindings(sub, reviews, requiredReviewers)...)
	findings = append(findings, overdueReviewFinding(reviews, now)...)
	return findings
}

// overdueReviewFinding reports overdue_review: the earliest confirmed,
// still-open review whose due date has passed.
func overdueReviewFinding(reviews []normalizer.ReviewAssignment, now time.Time) []Finding {
	for _, review := range reviews {
		if isOverdueCandidate(review, now) {
			return []Finding{{Kind: FindingOverdueReview, Responsible: "reviewer", Deadline: review.ReviewDueDate, Contact: review.ReviewerEmail}}
		}
	}
	return nil
}

// isOverdueCandidate compares the due date against the start of its
// calendar day, not the end: OJS's own reviewAssignment status already
// reports a review overdue from the first moment of its due date, not from
// the moment the day ends.
func isOverdueCandidate(review normalizer.ReviewAssignment, now time.Time) bool {
	if !isOpenForOverdueCheck(review) {
		return false
	}
	due, ok := normalizer.ParseDate(review.ReviewDueDate)
	startOfDue := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, due.Location())
	return ok && !now.Before(startOfDue)
}

// isOpenForOverdueCheck excludes a review OJS itself no longer treats as
// open. OJS's "Reviews" export marks Declined and Unconsidered on
// assignment rows that are still open once a submission has more than one
// reviewer invitation in the round, so those two flags cannot rule out an
// overdue review here: a past-due date always wins.
// ReviewDueWindowSuperseded (set by OJSReviewDueEnricher) skips a review row
// OJS's own live state no longer treats as open: the CSV export keeps a superseded or reassigned row with
// an empty Date Completed and a past-due date forever, which would
// otherwise read as permanently overdue.
func isOpenForOverdueCheck(review normalizer.ReviewAssignment) bool {
	return review.DateConfirmed != "" && review.DateCompleted == "" && !review.ReviewDueWindowSuperseded
}

// reviewerFindings reports staffing below the required minimum and overdue
// invitations that leave the round below that minimum. A surplus unanswered
// invitation does not reopen either finding in an otherwise staffed round.
func reviewerFindings(sub normalizer.Submission, reviews []normalizer.ReviewAssignment, requiredReviewers int) []Finding {
	// missing_reviewer and overdue_review_invitation, the two findings below.
	const capacityHint = 2
	findings := make([]Finding, 0, capacityHint)
	working := workingReviewers(sub, reviews)
	if missingReviewer(reviews, working, requiredReviewers) {
		findings = append(findings, Finding{Kind: FindingMissingReviewer, Responsible: responsibleEditor})
	}
	if hasOverdueResponse(reviews) && working < requiredReviewers {
		findings = append(findings, Finding{Kind: FindingOverdueReviewInvitation, Responsible: responsibleEditor})
	}
	return findings
}

// hasOverdueResponse reports whether the latest round's open reviews have a
// reviewer invitation OJS itself considers overdue (RoundHasOverdueResponse,
// set by OJSResponseOverdueEnricher from OJS's own reviewAssignment statusId).
// It is round-level, not reviewer-level, matching the
// field it reads: OJS's REST API has no way to join an overdue round status
// back to a specific reviewer identity.
//
// It reads the latest round alone, not every round ever opened: OJS keeps a
// closed round's own unanswered invitation at statusId 4 forever, even after
// a later round supersedes it.
// Scanning every round let that permanently-stuck invitation block
// missing_reviewer and overdue_review_invitation from ever clearing, even
// once the current round was fully staffed and on time.
func hasOverdueResponse(reviews []normalizer.ReviewAssignment) bool {
	for _, review := range latestRound(reviews) {
		if review.DateCompleted == "" && review.RoundHasOverdueResponse {
			return true
		}
	}
	return false
}
