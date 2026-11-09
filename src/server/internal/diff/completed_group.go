package diff

import (
	"server/internal/normalizer"
	"sort"
	"strconv"
)

// reviewGroup is one submission's review assignments in one stage and round.
type reviewGroup struct {
	key         string
	assignments map[string]normalizer.ReviewAssignment
}

// groupReviews gathers the review assignments by submission, stage and round.
func groupReviews(reviews []normalizer.ReviewAssignment) map[string]*reviewGroup {
	groups := map[string]*reviewGroup{}
	for _, review := range reviews {
		key := review.SubmissionID + "|" + review.Stage + "|" + strconv.Itoa(review.Round)
		if groups[key] == nil {
			groups[key] = &reviewGroup{key: key, assignments: map[string]normalizer.ReviewAssignment{}}
		}
		groups[key].assignments[review.Key] = review
	}
	return groups
}

func completedGroupChanges(previous, current normalizer.NormalizedCycle) []Change {
	previousGroups := groupReviews(previous.Reviews)
	currentGroups := groupReviews(current.Reviews)
	keys := make([]string, 0, len(currentGroups))
	for key := range currentGroups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	changes := make([]Change, 0)
	for _, key := range keys {
		if change, ok := groupJustCompleted(previousGroups[key], currentGroups[key], key); ok {
			changes = append(changes, change)
		}
	}
	return changes
}
