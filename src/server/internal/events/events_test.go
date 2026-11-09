package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"server/internal/diff"
	"strings"
	"testing"
)

const rn5HintEventType = "revised_version_hint"

func TestRN5HintOccurrenceIsStable(t *testing.T) {
	change := diff.Change{Type: rn5HintEventType, EntityType: "submission", EntityKey: "42", Transition: "completed_review->later_file_increase", OccurrenceKey: "42|review-1@2026-06-01T10:00:00Z|files:2->3"}
	first := Detect([]diff.Change{change})
	second := Detect([]diff.Change{change})
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("RED: RN5 hint event missing: first=%#v second=%#v", first, second)
	}
	if first[0].OccurrenceKey != change.OccurrenceKey || first[0].IdempotencyKey != second[0].IdempotencyKey {
		t.Fatalf("unstable RN5 hint occurrence: first=%#v second=%#v", first, second)
	}
	matches := MatchRules(first[0])
	if len(matches) != 1 || matches[0].RuleCode != RuleRN5 {
		t.Fatalf("RN5 hint matches = %#v", matches)
	}
}

func TestRN5HintReplayIsIdempotent(t *testing.T) {
	base := diff.Change{Type: rn5HintEventType, EntityType: "submission", EntityKey: "42", Transition: "completed_review->later_file_increase", OccurrenceKey: "42|review-1@2026-06-01T10:00:00Z|files:2->3"}
	replay := Detect([]diff.Change{base, base})
	if len(replay) != 2 {
		t.Fatalf("RED: RN5 hint replay contract missing: events=%#v", replay)
	}
	if replay[0].IdempotencyKey != replay[1].IdempotencyKey {
		t.Fatalf("replay keys differ: %#v", replay)
	}
	next := base
	next.OccurrenceKey = "42|review-1@2026-06-01T10:00:00Z|files:3->4"
	nextEvents := Detect([]diff.Change{next})
	if len(nextEvents) != 1 || nextEvents[0].IdempotencyKey == replay[0].IdempotencyKey {
		t.Fatalf("new file increase did not create new occurrence: %#v", nextEvents)
	}
}

func TestDetectMapsDomainChangesToCatalog(t *testing.T) {
	tests := []struct{ changeType, eventType string }{
		{diff.ChangeReviewInvitationAccepted, EventReviewInvitationAccepted},
		{diff.ChangeReviewInvitationRejected, EventReviewInvitationRejected},
		{diff.ChangeReviewSubmitted, EventReviewSubmitted},
		{diff.ChangeAllReviewsCompleted, EventAllReviewsCompleted},
	}
	for _, tt := range tests {
		change := diff.Change{Type: tt.changeType, EntityType: "review", EntityKey: "42|Review|1|r1", Transition: "pending->completed", OccurrenceKey: "2026-06-01T10:00:00Z"}
		events := Detect([]diff.Change{change})
		if len(events) != 1 || events[0].Type != tt.eventType {
			t.Fatalf("Detect(%q) = %#v", tt.changeType, events)
		}
	}
}

func TestIdempotencyKeyIsVersionedStableAndCycleIndependent(t *testing.T) {
	event := Event{Type: EventReviewSubmitted, EntityType: "review", EntityKey: "42|Review|1|r1", Transition: "pending->submitted", OccurrenceKey: "2026-06-01T10:00:00Z"}
	wantPreimage := "event:v1\x00review_submitted\x00review\x0042|Review|1|r1\x00pending->submitted\x002026-06-01T10:00:00Z"
	sum := sha256.Sum256([]byte(wantPreimage))
	want := hex.EncodeToString(sum[:])
	if got := IdempotencyKey(event); got != want {
		t.Fatalf("IdempotencyKey() = %q, want %q", got, want)
	}
	if got := IdempotencyKey(event); got != want {
		t.Fatalf("replay key = %q, want %q", got, want)
	}
	event.OccurrenceKey = "2026-06-02T10:00:00Z"
	if got := IdempotencyKey(event); got == want {
		t.Fatal("new occurrence reused previous key")
	}
	event.OccurrenceKey = "round:2"
	if got := IdempotencyKey(event); got == want {
		t.Fatal("new round reused previous key")
	}
}

func TestMatchRulesReturnsSeparateUniqueMatches(t *testing.T) {
	tests := []struct{ eventType, rule string }{
		{EventReviewInvitationAccepted, RuleRN1},
		{EventReviewInvitationRejected, RuleRN1},
		{EventReviewSubmitted, RuleRN3},
		{EventAllReviewsCompleted, RuleRN4},
	}
	for _, tt := range tests {
		event := Event{Type: tt.eventType, EntityKey: "entity"}
		matches := MatchRules(event)
		if len(matches) != 1 || matches[0].RuleCode != tt.rule {
			t.Fatalf("MatchRules(%q) = %#v", tt.eventType, matches)
		}
		if matches[0].Status != MatchStatusPending {
			t.Fatalf("status = %q, want pending", matches[0].Status)
		}
	}
}

func TestEventDoesNotEmbedRuleCode(t *testing.T) {
	body, err := json.Marshal(Event{Type: EventReviewSubmitted, EntityKey: "entity"})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "rulecode") || strings.Contains(strings.ToLower(string(body)), "rule_code") {
		t.Fatalf("event embeds rule code: %s", body)
	}
}

func TestActivityTitlesUseSemanticKeys(t *testing.T) {
	want := map[string]string{
		RuleRN1: "review_invitation_answered", RuleRN2: "review_invitation_expired",
		RuleRN3: "review_submitted", RuleRN4: "all_reviews_completed",
		RuleRN5: "revised_file_hint", RuleRN6: "review_invitation_reminder",
		RuleRN7: "review_deadline_reminder", RuleRN8: "review_deadline_expired",
		RuleRN18: "missing_copyeditor", RuleWelcome: "welcome", "RN99": "editorial_notification",
	}
	for rule, key := range want {
		if got := ActivityTitleKey(rule); got != key {
			t.Errorf("ActivityTitleKey(%q) = %q, want %q", rule, got, key)
		}
	}
}

// The reviewer subset used to be respelled in three other files: the template
// switch in email/render.go, the reviewerRules map in email/renderer.go, and
// isReviewerRule in notifications/recipients.go. It is declared once now, so
// this test pins what that one declaration says.
func TestIsReviewerRuleNamesTheThreeReviewerRules(t *testing.T) {
	want := map[string]bool{
		RuleRN1: false, RuleRN2: false, RuleRN3: false, RuleRN4: false,
		RuleRN5: false, RuleRN6: true, RuleRN7: true, RuleRN8: true,
		RuleWelcome: false, "RN99": false,
	}
	for rule, isReviewer := range want {
		if got := IsReviewerRule(rule); got != isReviewer {
			t.Errorf("IsReviewerRule(%q) = %v, want %v", rule, got, isReviewer)
		}
	}
}

// A reviewer rule that AllRuleCodes does not list could never be delivered,
// because the notification service rejects a code IsKnownRule denies.
func TestEveryReviewerRuleIsAKnownRule(t *testing.T) {
	for _, rule := range AllRuleCodes() {
		if !IsKnownRule(rule) {
			t.Errorf("AllRuleCodes lists %q, but IsKnownRule denies it", rule)
		}
	}
	for rule := range reviewerRules {
		if !IsKnownRule(rule) {
			t.Errorf("reviewerRules lists %q, but IsKnownRule denies it", rule)
		}
	}
	if IsKnownRule("RN99") || IsKnownRule(RuleWelcome) {
		t.Error("IsKnownRule accepted a code no editorial rule produces")
	}
}

func TestRecipientRoleMapsRulesWithoutGuessingUnknownRoles(t *testing.T) {
	for _, rule := range []string{RuleRN1, RuleRN2, RuleRN3, RuleRN4, RuleRN5} {
		if got := RecipientRole(rule); got != RecipientRoleAssociateEditor {
			t.Errorf("RecipientRole(%q)=%q, want associate editor", rule, got)
		}
	}
	for _, rule := range []string{RuleRN6, RuleRN7, RuleRN8} {
		if got := RecipientRole(rule); got != RecipientRoleReviewer {
			t.Errorf("RecipientRole(%q)=%q, want reviewer", rule, got)
		}
	}
	if got := RecipientRole(RuleRN18); got != RecipientRoleJournalEditor {
		t.Errorf("RecipientRole(RN18)=%q, want journal editor", got)
	}
	for _, rule := range []string{RuleWelcome, "unknown-rule"} {
		if got := RecipientRole(rule); got != "" {
			t.Errorf("RecipientRole(%q)=%q, want no specific role", rule, got)
		}
	}
}
