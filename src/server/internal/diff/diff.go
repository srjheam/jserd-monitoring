package diff

import (
	"server/internal/normalizer"
)

const (
	ChangeReviewInvitationAccepted = "review_invitation_accepted"
	ChangeReviewInvitationRejected = "review_invitation_rejected"
	ChangeReviewSubmitted          = "review_submitted"
	ChangeAllReviewsCompleted      = "all_reviews_completed"
	ChangeRN5Hint                  = "revised_version_hint"
)

type Change struct {
	Type          string `json:"type"`
	EntityType    string `json:"entityType"`
	EntityKey     string `json:"entityKey"`
	Transition    string `json:"transition"`
	OccurrenceKey string `json:"occurrenceKey"`
}

func Compare(previous, current normalizer.NormalizedCycle) []Change {
	previousByKey := make(map[string]normalizer.ReviewAssignment, len(previous.Reviews))
	for _, review := range previous.Reviews {
		previousByKey[review.Key] = review
	}
	changes := make([]Change, 0)
	for _, review := range current.Reviews {
		before, exists := previousByKey[review.Key]
		if !exists {
			continue
		}
		changes = append(changes, reviewChanges(before, review)...)
	}
	changes = append(changes, completedGroupChanges(previous, current)...)
	changes = append(changes, rn5HintChanges(previous, current)...)
	return changes
}
