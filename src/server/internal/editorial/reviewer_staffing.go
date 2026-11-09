package editorial

import "server/internal/normalizer"

// This file counts a review round's staffing: which reviewers are still
// working it, and whether that count meets the floor review_status.go's
// findings check against.

func missingReviewer(reviews []normalizer.ReviewAssignment, working, requiredReviewers int) bool {
	if len(reviews) == 0 {
		return true
	}
	return !allReviewsCompleted(reviews) && working < requiredReviewers
}

// workingReviewers is the latest round's reviewer count that RN9 and RN19
// compare with the floor. OJS's live count (Submission.ActiveReviewers) wins
// when it is set: the Reviews export fills DateConfirmed on a decline too,
// so a declined row would count as working (defect B2). Without the live
// count, the export rows are the fallback.
//
// The export count no longer treats any single overdue invitation as
// disqualifying on its own: a round can carry a surplus reviewer who never
// answers while the reviewers who did answer already meet the floor.
// reviewerFindings also uses this
// result to silence overdue_review_invitation once the floor is met: a
// surplus no-show above the floor is not a chief editor's problem to chase
// by the staffing rule.
func workingReviewers(sub normalizer.Submission, reviews []normalizer.ReviewAssignment) int {
	if sub.ActiveReviewers != nil {
		return *sub.ActiveReviewers
	}
	return countWorkingReviewers(latestRound(reviews))
}

// countWorkingReviewers counts a round's reviewers who answered their
// invitation, already finished, or still have time to answer. A row OJS
// never sent an invitation for (no Key), or one OJS's live state no longer
// treats as open (ReviewDueWindowSuperseded), does not count.
func countWorkingReviewers(reviews []normalizer.ReviewAssignment) int {
	count := 0
	for _, review := range reviews {
		if isWorkingReviewer(review) {
			count++
		}
	}
	return count
}

// isWorkingReviewer counts a reviewer who answered (confirmed) or finished
// their invitation. It also counts a reviewer who has neither yet, as long
// as OJS has not flagged the round's response overdue. An invited reviewer
// may have a response deadline of today that they have not missed. Firing
// missing_reviewer the moment a round is
// short of confirmed reviewers, even while that seat's own invitation still
// has time to be answered, reads as a vacancy that is not there yet. Once
// RoundHasOverdueResponse flips true — OJS's own statusId says the response
// deadline passed — an unanswered seat stops counting, the same as before.
func isWorkingReviewer(review normalizer.ReviewAssignment) bool {
	if !isOpenAssignment(review) {
		return false
	}
	if review.DateCompleted != "" || review.DateConfirmed != "" {
		return true
	}
	return !review.RoundHasOverdueResponse
}

// isOpenAssignment counts a completed review regardless of
// ReviewDueWindowSuperseded: that flag matches a CSV row's round and review
// due date against OJS's live reviewAssignments, and two reviewers who share
// the same due date in the same round collide on that match. A second,
// already-replaced reviewer can share its stale due date with a
// completed one. A completed review already happened; whether OJS
// still lists a live counterpart for its due-date window cannot undo that.
func isOpenAssignment(review normalizer.ReviewAssignment) bool {
	if review.Key == "" {
		return false
	}
	return review.DateCompleted != "" || !review.ReviewDueWindowSuperseded
}

func allReviewsCompleted(reviews []normalizer.ReviewAssignment) bool {
	if len(reviews) == 0 {
		return false
	}
	for _, review := range reviews {
		if review.DateCompleted == "" {
			return false
		}
	}
	return true
}

func latestRound(reviews []normalizer.ReviewAssignment) []normalizer.ReviewAssignment {
	return roundReviews(reviews, maxRound(reviews))
}

func maxRound(reviews []normalizer.ReviewAssignment) int {
	round := 0
	for _, review := range reviews {
		if review.Round > round {
			round = review.Round
		}
	}
	return round
}

func roundReviews(reviews []normalizer.ReviewAssignment, round int) []normalizer.ReviewAssignment {
	current := make([]normalizer.ReviewAssignment, 0, len(reviews))
	for _, review := range reviews {
		if review.Round == round {
			current = append(current, review)
		}
	}
	return current
}
