package notifications

import (
	"server/internal/collections"
	"server/internal/events"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// enqueueWelcomeIfNeeded gives a recipient one welcome e-mail, the first
// time any notification is created for them, enqueued alongside the real
// one with no delivery-order guarantee (ADR 0016). A recipient counts as
// welcomed the moment this is enqueued, not once delivery is confirmed.
func enqueueWelcomeIfNeeded(app core.App, event *core.Record, recipient *core.Record, targetKind, targetKey, role string) error {
	if recipient.GetString("welcomed_at") != "" {
		return nil
	}
	notification, err := createWelcomeNotification(app, event)
	if err != nil {
		return err
	}
	welcome := resolvedRecipient{Record: recipient, TargetKey: targetKey}
	if err := queueDelivery(app, notification, welcome, targetKind, role); err != nil {
		return err
	}
	recipient.Set("welcomed_at", time.Now().UTC())
	return app.Save(recipient)
}

func createWelcomeNotification(app core.App, event *core.Record) (*core.Record, error) {
	nc, err := app.FindCollectionByNameOrId(collections.Notifications)
	if err != nil {
		return nil, err
	}
	notification := core.NewRecord(nc)
	notification.Set("event", event.Id)
	notification.Set("rule_code", events.RuleWelcome)
	return notification, app.Save(notification)
}
