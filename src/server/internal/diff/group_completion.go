package diff

import (
	"server/internal/normalizer"
	"sort"
	"strings"
)

// groupJustCompleted reports whether every review of one round just finished.
// A round whose reviewers changed since the previous cycle reports nothing,
// because the two rounds are not the same set of obligations.
func groupJustCompleted(previousGroup, currentGroup *reviewGroup, key string) (Change, bool) {
	if !comparableGroups(previousGroup, currentGroup) {
		return Change{}, false
	}
	signature, allCurrent, allPrevious := groupCompletion(previousGroup, currentGroup)
	if !justCompleted(signature, allCurrent, allPrevious) {
		return Change{}, false
	}
	sort.Strings(signature)
	return Change{Type: ChangeAllReviewsCompleted, EntityType: "review_round", EntityKey: key, Transition: "incomplete->completed", OccurrenceKey: strings.Join(signature, ",")}, true
}

// justCompleted reports whether a round holds the same reviewers as before, all
// complete now, and not all complete before.
func justCompleted(signature []string, allCurrent, allPrevious bool) bool {
	return signature != nil && allCurrent && !allPrevious
}

// comparableGroups reports whether two rounds hold the same number of review
// assignments, and at least one.
func comparableGroups(previousGroup, currentGroup *reviewGroup) bool {
	if previousGroup == nil || len(currentGroup.assignments) == 0 {
		return false
	}
	return len(currentGroup.assignments) == len(previousGroup.assignments)
}

// groupCompletion reports whether every review of a round is complete now, and
// whether every one was complete before. It reports a nil signature when the
// two rounds hold different reviewers.
func groupCompletion(previousGroup, currentGroup *reviewGroup) (signature []string, allCurrent, allPrevious bool) {
	completedNow, completedBefore := 0, 0
	signature = make([]string, 0, len(currentGroup.assignments))
	for assignmentKey, review := range currentGroup.assignments {
		before, existed := previousGroup.assignments[assignmentKey]
		if !existed {
			return nil, false, false
		}
		completedNow += countComplete(review)
		completedBefore += countComplete(before)
		signature = append(signature, review.Key+"@"+review.DateCompleted)
	}
	total := len(currentGroup.assignments)
	return signature, completedNow == total, completedBefore == total
}

// countComplete is 1 for a review that was submitted, and 0 for one that was
// not.
func countComplete(review normalizer.ReviewAssignment) int {
	if review.DateCompleted != "" {
		return 1
	}
	return 0
}
