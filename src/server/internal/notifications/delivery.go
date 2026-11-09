package notifications

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"
	"server/internal/events"

	"github.com/pocketbase/pocketbase/core"
)

// notificationPayload is the part of an event payload that decides who to
// notify.
type notificationPayload struct {
	SubmissionID     string   `json:"submissionId"`
	TargetKind       string   `json:"targetKind"`
	TargetKey        string   `json:"targetKey"`
	EditorTargetKeys []string `json:"editorTargetKeys"`
}

// eventPayload reads the event behind one rule match, and the part of its
// payload that decides who to notify.
func eventPayload(app core.App, match *core.Record) (*core.Record, notificationPayload, error) {
	var payload notificationPayload
	event, err := app.FindRecordById(collections.Events, match.GetString("event"))
	if err != nil {
		return nil, payload, err
	}
	if err := json.Unmarshal([]byte(event.GetString("payload")), &payload); err != nil {
		return nil, payload, fmt.Errorf("decode event payload: %w", err)
	}
	return event, payload, nil
}

// notifyRecipients resolves who a rule match must notify, then records the
// notification and one delivery request per recipient.
func (s *Service) notifyRecipients(app core.App, match *core.Record, rule string, editorKeys []string) error {
	event, payload, err := eventPayload(app, match)
	if err != nil {
		return err
	}
	recipients, targetKind, err := resolveRecipients(app, rule, payload.TargetKey, editorKeys)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return fmt.Errorf("%s recipient not found", targetKind)
	}
	return saveNotification(app, match, event, rule, recipients, targetKind)
}

// saveNotification writes the notification row and one delivery request per
// recipient.
func saveNotification(app core.App, match, event *core.Record, rule string, recipients []resolvedRecipient, targetKind string) error {
	nc, _ := app.FindCollectionByNameOrId(collections.Notifications)
	notification := core.NewRecord(nc)
	notification.Set("event", event.Id)
	notification.Set("rule_match", match.Id)
	notification.Set("rule_code", rule)
	if err := app.Save(notification); err != nil {
		return err
	}
	role := events.RecipientRole(rule)
	for _, recipient := range recipients {
		if err := deliverToRecipient(app, notification, event, recipient, targetKind, role); err != nil {
			return err
		}
	}
	return nil
}

// deliverToRecipient records one recipient's target and delivery request
// for a notification, then gives them a welcome e-mail if this is their
// first one ever.
func deliverToRecipient(app core.App, notification, event *core.Record, recipient resolvedRecipient, targetKind, role string) error {
	if err := queueDelivery(app, notification, recipient, targetKind, role); err != nil {
		return err
	}
	return enqueueWelcomeIfNeeded(app, event, recipient.Record, targetKind, recipient.TargetKey, role)
}

// queueDelivery records one recipient's target for a notification, then
// queues the e-mail to that target.
func queueDelivery(app core.App, notification *core.Record, recipient resolvedRecipient, targetKind, role string) error {
	tc, err := app.FindCollectionByNameOrId(collections.NotificationTargets)
	if err != nil {
		return err
	}
	target := core.NewRecord(tc)
	target.Set("notification", notification.Id)
	target.Set("recipient", recipient.Record.Id)
	target.Set("target_kind", targetKind)
	target.Set("target_key", recipient.TargetKey)
	target.Set("recipient_role", role)
	if err := app.Save(target); err != nil {
		return err
	}
	rc, err := app.FindCollectionByNameOrId(collections.EmailDeliveryRequests)
	if err != nil {
		return err
	}
	request := core.NewRecord(rc)
	request.Set("target", target.Id)
	return app.Save(request)
}
