package state

import (
	"server/internal/events"
	"server/internal/normalizer"
	"sort"
	"strings"
)

func enrichEvent(event events.Event, cycle normalizer.NormalizedCycle) events.Event {
	event, matchedReview := withSubmissionID(event, cycle)
	if matchedReview == nil {
		matchedReview = reviewByKey(cycle.Reviews, event.EntityKey)
	}
	if matchedReview != nil {
		event = withReviewDetail(event, *matchedReview)
	}
	return withSubmissionDetail(event, cycle)
}

// withSubmissionID fills the submission an event belongs to, when the event
// names none already. It also reports the review assignment the event key
// names, if there is one.
func withSubmissionID(event events.Event, cycle normalizer.NormalizedCycle) (events.Event, *normalizer.ReviewAssignment) {
	if event.SubmissionID != "" {
		return event, nil
	}
	event.SubmissionID = submissionIDFromEntity(event)
	review := reviewByKey(cycle.Reviews, event.EntityKey)
	if review != nil {
		event.SubmissionID = review.SubmissionID
	}
	return event, review
}

// submissionIDFromEntity reads the submission an event's entity key names.
func submissionIDFromEntity(event events.Event) string {
	switch event.EntityType {
	case "submission":
		return event.EntityKey
	case "review_round":
		// EntityKey is "<submissionID>|<Stage>|<Round>" (diff.completedGroupChanges).
		if id, _, ok := strings.Cut(event.EntityKey, "|"); ok {
			return id
		}
	}
	return event.SubmissionID
}

// reviewByKey is the review assignment one event key names, or nil when no
// review carries that key.
func reviewByKey(reviews []normalizer.ReviewAssignment, key string) *normalizer.ReviewAssignment {
	for i := range reviews {
		if reviews[i].Key == key {
			return &reviews[i]
		}
	}
	return nil
}

// reviewDeadlines maps each review event type to the deadline it reports.
var reviewDeadlines = map[string]func(normalizer.ReviewAssignment) string{
	events.EventReviewResponseDue:      func(r normalizer.ReviewAssignment) string { return r.ResponseDueDate },
	events.EventReviewResponseReminder: func(r normalizer.ReviewAssignment) string { return r.ResponseDueDate },
	events.EventReviewDueReminder:      func(r normalizer.ReviewAssignment) string { return r.ReviewDueDate },
	events.EventReviewDue:              func(r normalizer.ReviewAssignment) string { return r.ReviewDueDate },
}

// withReviewDetail fills the reviewer name, the title and the deadline that the
// review assignment carries.
func withReviewDetail(event events.Event, review normalizer.ReviewAssignment) events.Event {
	if requiresReviewerName(event.Type) {
		event.ReviewerName = strings.Join(strings.Fields(review.ReviewerGivenName+" "+review.ReviewerFamilyName), " ")
	}
	if event.Title == "" {
		event.Title = review.SubmissionTitle
	}
	if deadline, ok := reviewDeadlines[event.Type]; ok {
		event.Deadline = deadline(review)
	}
	return event
}

// withSubmissionDetail fills the title, the OJS link and the assigned editors
// of the submission an event belongs to.
func withSubmissionDetail(event events.Event, cycle normalizer.NormalizedCycle) events.Event {
	for _, submission := range cycle.Submissions {
		if submission.OJSSubmissionID != event.SubmissionID {
			continue
		}
		event.Title = submission.Title
		event.OJSURL = submission.URL
		event.EditorTargetKeys = editorTargetKeys(submission.Editors)
		return event
	}
	return event
}

// editorTargetKeys are the assigned-editor slugs (ADR-0006) of one submission,
// without repeats, in increasing order.
func editorTargetKeys(editors []normalizer.Editor) []string {
	keys := make([]string, 0, len(editors))
	seen := map[string]struct{}{}
	for _, editor := range editors {
		key := normalizer.NameSlug(editor.GivenName + " " + editor.FamilyName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func requiresReviewerName(eventType string) bool {
	switch eventType {
	case events.EventReviewInvitationAccepted, events.EventReviewInvitationRejected, events.EventReviewResponseDue, events.EventReviewSubmitted:
		return true
	default:
		return false
	}
}
