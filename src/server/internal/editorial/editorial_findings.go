package editorial

import (
	"server/internal/normalizer"
	"time"
)

// This file holds the findings that belong to the chief editor's own desk
// once a round of reviews is settled: an unsent RN4 delivery, a submission
// with no assigned editor, and a completed round with no decision.

func editorialFindings(sub normalizer.Submission, reviews []normalizer.ReviewAssignment, deliveries DeliveryLookup) []Finding {
	// The three findings this function can append below.
	const capacityHint = 3
	findings := make([]Finding, 0, capacityHint)
	findings = append(findings, unsentReviewCompletionFinding(sub, deliveries)...)
	if len(sub.Editors) == 0 {
		findings = append(findings, Finding{Kind: FindingMissingEditor})
	}
	if reviewsCompleteNoDecision(sub, reviews) {
		findings = append(findings, Finding{Kind: FindingReviewsCompleteNoDecision, Responsible: responsibleEditor})
	}
	return findings
}

func unsentReviewCompletionFinding(sub normalizer.Submission, deliveries DeliveryLookup) []Finding {
	if unsent, known := deliveries.HasUnsentRN4(sub.OJSSubmissionID); known && unsent {
		return []Finding{{Kind: FindingUnsentReviewCompletion, Responsible: responsibleEditor}}
	}
	return nil
}

// reviewsCompleteNoDecision reports whether every review of the latest round
// is complete and no editorial decision followed. It carries no deadline: the
// wait starts the moment the last review lands, and nothing in OJS sets a date
// by which the chief editor must decide.
//
// It reads the latest round alone, not every round. An unanswered assignment
// in an earlier round must not keep allReviewsCompleted false forever.
func reviewsCompleteNoDecision(sub normalizer.Submission, reviews []normalizer.ReviewAssignment) bool {
	completedAt, ok := latestRoundCompletedAt(reviews)
	if !ok {
		return false
	}
	return !hasDecisionAfter(sub, completedAt)
}

// latestRoundCompletedAt returns when the latest round finished, and whether
// every review of that round carries a completion date at all.
func latestRoundCompletedAt(reviews []normalizer.ReviewAssignment) (time.Time, bool) {
	var latest time.Time
	for _, review := range latestRound(reviews) {
		when, ok := normalizer.ParseDate(review.DateCompleted)
		if !ok {
			return time.Time{}, false
		}
		if when.After(latest) {
			latest = when
		}
	}
	return latest, !latest.IsZero()
}

func hasDecisionAfter(sub normalizer.Submission, moment time.Time) bool {
	for _, editor := range sub.Editors {
		if editorDecidedAfter(editor, moment) {
			return true
		}
	}
	return false
}

func editorDecidedAfter(editor normalizer.Editor, moment time.Time) bool {
	for _, decision := range editor.Decisions {
		if when, ok := normalizer.ParseDate(decision.DateDecided); ok && when.After(moment) {
			return true
		}
	}
	return false
}
