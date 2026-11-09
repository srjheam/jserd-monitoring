package events

import (
	"crypto/sha256"
	"encoding/hex"
	"server/internal/diff"
	"strings"
)

const (
	EventReviewInvitationAccepted = "review_invitation_accepted"
	EventReviewInvitationRejected = "review_invitation_rejected"
	EventReviewSubmitted          = "review_submitted"
	EventAllReviewsCompleted      = "all_reviews_completed"
	EventRN5Hint                  = "revised_version_hint"
	EventReviewResponseDue        = "review_response_due"
	EventReviewResponseReminder   = "review_response_reminder"
	EventReviewDueReminder        = "review_due_reminder"
	EventReviewDue                = "review_due"
	EventMissingCopyeditor        = "missing_copyeditor"

	RuleRN1  = "RN1"
	RuleRN3  = "RN3"
	RuleRN4  = "RN4"
	RuleRN5  = "RN5"
	RuleRN2  = "RN2"
	RuleRN6  = "RN6"
	RuleRN7  = "RN7"
	RuleRN8  = "RN8"
	RuleRN18 = "RN18"

	// RuleWelcome is not an editorial rule: no event produces it, and it
	// carries no submission. The welcome e-mail borrows the rule_code column to
	// name its template, so the code lives here with the rest of them.
	RuleWelcome = "WELCOME"

	MatchStatusPending = "pending"
)

type Event struct {
	Type             string   `json:"type"`
	EntityType       string   `json:"entityType"`
	EntityKey        string   `json:"entityKey"`
	Transition       string   `json:"transition"`
	OccurrenceKey    string   `json:"occurrenceKey"`
	IdempotencyKey   string   `json:"idempotencyKey"`
	SubmissionID     string   `json:"submissionId,omitempty"`
	TargetKind       string   `json:"targetKind,omitempty"`
	TargetKey        string   `json:"targetKey,omitempty"`
	Title            string   `json:"title,omitempty"`
	OJSURL           string   `json:"ojs_url,omitempty"`
	Deadline         string   `json:"deadline,omitempty"`
	ReviewerName     string   `json:"reviewerName,omitempty"`
	EditorTargetKeys []string `json:"editorTargetKeys,omitempty"`
}

type RuleMatch struct {
	RuleCode string `json:"ruleCode"`
	Status   string `json:"status"`
}

func Detect(changes []diff.Change) []Event {
	events := make([]Event, 0, len(changes))
	for _, change := range changes {
		switch change.Type {
		case diff.ChangeReviewInvitationAccepted, diff.ChangeReviewInvitationRejected, diff.ChangeReviewSubmitted, diff.ChangeAllReviewsCompleted, diff.ChangeRN5Hint:
			event := Event{Type: change.Type, EntityType: change.EntityType, EntityKey: change.EntityKey, Transition: change.Transition, OccurrenceKey: change.OccurrenceKey}
			event.IdempotencyKey = IdempotencyKey(event)
			events = append(events, event)
		}
	}
	return events
}

func IdempotencyKey(event Event) string {
	preimage := strings.Join([]string{"event:v1", event.Type, event.EntityType, event.EntityKey, event.Transition, event.OccurrenceKey}, "\x00")
	sum := sha256.Sum256([]byte(preimage))
	return hex.EncodeToString(sum[:])
}

// eventRules maps each event type to the notification rule it triggers. An
// event type with no entry triggers no notification.
var eventRules = map[string]string{
	EventReviewInvitationAccepted: RuleRN1,
	EventReviewInvitationRejected: RuleRN1,
	EventReviewSubmitted:          RuleRN3,
	EventAllReviewsCompleted:      RuleRN4,
	EventRN5Hint:                  RuleRN5,
	EventReviewResponseDue:        RuleRN2,
	EventReviewResponseReminder:   RuleRN6,
	EventReviewDueReminder:        RuleRN7,
	EventReviewDue:                RuleRN8,
	EventMissingCopyeditor:        RuleRN18,
}

func MatchRules(event Event) []RuleMatch {
	rule, ok := eventRules[event.Type]
	if !ok {
		return nil
	}
	return []RuleMatch{{RuleCode: rule, Status: MatchStatusPending}}
}
