package email

import (
	"context"
	"encoding/json"
	"fmt"
	"server/internal/events"
	"slices"

	"github.com/pocketbase/pocketbase/core"
)

// Check queued targets too: older events may still name every assigned editor.
func (w *Worker) editorRoleAllowed(ctx context.Context, request *core.Record) (deliveryRecords, bool, error) {
	records, err := w.deliveryRecords(request)
	if err != nil {
		return records, false, err
	}
	if w.EditorTargets == nil {
		return records, true, nil
	}
	rule := records.notification.GetString("rule_code")
	if events.IsReviewerRule(rule) || records.target.GetString("target_kind") == "reviewer" {
		return records, true, nil
	}
	rule, err = editorNotificationRule(rule, records.event.GetString("type"))
	if err != nil {
		return records, false, err
	}
	var payload deliveryPayload
	if err := json.Unmarshal([]byte(records.event.GetString("payload")), &payload); err != nil {
		return records, false, err
	}
	keys, err := w.EditorTargets(ctx, rule, payload.SubmissionID)
	if err != nil {
		return records, false, err
	}
	allowed := slices.Contains(keys, records.target.GetString("target_key"))
	if allowed {
		// Legacy queued targets gain a role only after the live assignment check.
		records.recipientRole = events.RecipientRole(rule)
	}
	return records, allowed, nil
}

func editorNotificationRule(rule, eventType string) (string, error) {
	if rule != events.RuleWelcome {
		return rule, nil
	}
	matches := events.MatchRules(events.Event{Type: eventType})
	if len(matches) == 0 {
		return "", fmt.Errorf("welcome editor assignment cannot be verified")
	}
	return matches[0].RuleCode, nil
}
