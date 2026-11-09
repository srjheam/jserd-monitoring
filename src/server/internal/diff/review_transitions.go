package diff

import "server/internal/normalizer"

// reviewTransition is one state change a review assignment can go through in a
// cycle: the change to record, and the test that tells whether it just
// happened.
type reviewTransition struct {
	changeType    string
	transition    string
	happened      func(before, review normalizer.ReviewAssignment) bool
	occurrenceKey func(review normalizer.ReviewAssignment) string
}

var reviewTransitions = []reviewTransition{
	{ChangeReviewInvitationAccepted, "pending->accepted", invitationAccepted, func(r normalizer.ReviewAssignment) string { return r.DateConfirmed }},
	{ChangeReviewInvitationRejected, "pending->rejected", invitationRejected, func(r normalizer.ReviewAssignment) string { return r.Key }},
	{ChangeReviewSubmitted, "pending->submitted", reviewSubmitted, func(r normalizer.ReviewAssignment) string { return r.DateCompleted }},
}

// reviewChanges are the changes one review assignment went through since the
// previous cycle.
func reviewChanges(before, review normalizer.ReviewAssignment) []Change {
	changes := make([]Change, 0, len(reviewTransitions))
	for _, transition := range reviewTransitions {
		if !transition.happened(before, review) {
			continue
		}
		changes = append(changes, Change{
			Type:          transition.changeType,
			EntityType:    "review",
			EntityKey:     review.Key,
			Transition:    transition.transition,
			OccurrenceKey: transition.occurrenceKey(review),
		})
	}
	return changes
}

// invitationRejected reports whether a reviewer just declined the invitation.
func invitationRejected(before, review normalizer.ReviewAssignment) bool {
	return !before.Declined && review.Declined
}

// reviewSubmitted reports whether a reviewer just submitted the review.
func reviewSubmitted(before, review normalizer.ReviewAssignment) bool {
	return before.DateCompleted == "" && review.DateCompleted != ""
}

// invitationAccepted reports whether a reviewer just accepted the invitation.
//
// OJS's Reviews export marks Declined: Yes on every row unconditionally (see
// evaluator.go's comment on the same quirk), so a permanently-true Declined
// carries no signal here. Only a Declined that just turned true this cycle,
// alongside a fresh confirmation, means a real decline.
func invitationAccepted(before, review normalizer.ReviewAssignment) bool {
	justDeclined := review.Declined && !before.Declined
	return before.DateConfirmed == "" && review.DateConfirmed != "" && !justDeclined
}
