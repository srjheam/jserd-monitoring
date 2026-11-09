package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(createNotificationCollections, dropNotificationCollections)
}

// createNotificationCollections creates the collections that carry one
// notification from a rule match to a sent e-mail.
func createNotificationCollections(app core.App) error {
	found, err := findCollections(app, "events", "event_rule_matches")
	if err != nil {
		return err
	}
	events, matches := found[0], found[1]
	recipients := newRecipientsCollection()
	notifications := newNotificationsCollection(events, matches)
	if err := saveCollections(app, recipients, notifications); err != nil {
		return err
	}
	targets := newTargetsCollection(notifications, recipients)
	if err := app.Save(targets); err != nil {
		return err
	}
	return saveDeliveryCollections(app, targets, events, matches, recipients)
}

func dropNotificationCollections(app core.App) error {
	return dropCollections(app, "email_delivery_attempts", "email_delivery_claims", "email_delivery_requests", "notification_targets", "notifications", "recipients")
}

// saveDeliveryCollections creates the three e-mail delivery collections: the
// request, the claim that makes one worker own it, and the attempt that
// records how the send ended.
func saveDeliveryCollections(app core.App, targets, events, matches, recipients *core.Collection) error {
	requests := newRequestsCollection(targets)
	if err := app.Save(requests); err != nil {
		return err
	}
	claims := newClaimsCollection(requests)
	if err := app.Save(claims); err != nil {
		return err
	}
	return app.Save(newAttemptsCollection(requests, events, matches, targets, recipients))
}

func newRecipientsCollection() *core.Collection {
	recipients := core.NewBaseCollection("recipients")
	recipients.Fields.Add(&core.TextField{Name: "kind", Required: true}, &core.TextField{Name: "key", Required: true}, &core.EmailField{Name: "email"}, &core.TextField{Name: fieldName})
	recipients.Indexes = append(recipients.Indexes, "CREATE UNIQUE INDEX `idx_recipients_kind_key` ON `recipients` (`kind`, `key`)")
	return recipients
}

func newNotificationsCollection(events, matches *core.Collection) *core.Collection {
	notifications := core.NewBaseCollection("notifications")
	notifications.Fields.Add(&core.RelationField{Name: fieldEvent, CollectionId: events.Id, Required: true, MaxSelect: 1}, &core.RelationField{Name: "rule_match", CollectionId: matches.Id, Required: true, MaxSelect: 1}, &core.TextField{Name: fieldRuleCode, Required: true})
	notifications.Indexes = append(notifications.Indexes, "CREATE UNIQUE INDEX `idx_notifications_rule_match` ON `notifications` (`rule_match`)")
	return notifications
}

func newTargetsCollection(notifications, recipients *core.Collection) *core.Collection {
	targets := core.NewBaseCollection("notification_targets")
	targets.Fields.Add(&core.RelationField{Name: "notification", CollectionId: notifications.Id, Required: true, MaxSelect: 1}, &core.RelationField{Name: "recipient", CollectionId: recipients.Id, Required: true, MaxSelect: 1}, &core.TextField{Name: "target_kind", Required: true}, &core.TextField{Name: "target_key", Required: true})
	targets.Indexes = append(targets.Indexes, "CREATE UNIQUE INDEX `idx_notification_targets_entity` ON `notification_targets` (`notification`, `target_kind`, `target_key`)")
	return targets
}

func newRequestsCollection(targets *core.Collection) *core.Collection {
	requests := core.NewBaseCollection("email_delivery_requests")
	requests.Fields.Add(&core.RelationField{Name: "target", CollectionId: targets.Id, Required: true, MaxSelect: 1})
	requests.Indexes = append(requests.Indexes, "CREATE UNIQUE INDEX `idx_email_delivery_requests_target` ON `email_delivery_requests` (`target`)")
	return requests
}

func newClaimsCollection(requests *core.Collection) *core.Collection {
	claims := core.NewBaseCollection("email_delivery_claims")
	claims.Fields.Add(&core.RelationField{Name: fieldRequest, CollectionId: requests.Id, Required: true, MaxSelect: 1}, &core.DateField{Name: "claimed_at", Required: true}, &core.TextField{Name: "owner_id", Required: true})
	claims.Indexes = append(claims.Indexes, "CREATE UNIQUE INDEX `idx_email_delivery_claims_request` ON `email_delivery_claims` (`request`)")
	return claims
}

func newAttemptsCollection(requests, events, matches, targets, recipients *core.Collection) *core.Collection {
	attempts := core.NewBaseCollection("email_delivery_attempts")
	// raw_error stores only redacted transport responses/errors; never credentials,
	// tokens, cookies, DSNs, or complete message bodies.
	attempts.Fields.Add(
		&core.RelationField{Name: fieldRequest, CollectionId: requests.Id, Required: true, MaxSelect: 1}, &core.RelationField{Name: fieldEvent, CollectionId: events.Id, Required: true, MaxSelect: 1}, &core.RelationField{Name: "rule_match", CollectionId: matches.Id, Required: true, MaxSelect: 1}, &core.RelationField{Name: "target", CollectionId: targets.Id, Required: true, MaxSelect: 1}, &core.RelationField{Name: "recipient", CollectionId: recipients.Id, Required: true, MaxSelect: 1},
		&core.TextField{Name: fieldRuleCode, Required: true}, &core.TextField{Name: "target_kind", Required: true}, &core.TextField{Name: "target_key", Required: true}, &core.EmailField{Name: "to_address"}, &core.EmailField{Name: "from_address", Required: true}, &core.TextField{Name: "reply_to_addresses"}, &core.TextField{Name: "subject"}, &core.TextField{Name: "template_version", Required: true}, &core.TextField{Name: "template_hash", Required: true},
		&core.SelectField{Name: fieldStatus, Values: []string{statusValueSent, statusValueBlocked, statusValueFailed}, Required: true}, &core.SelectField{Name: "result_class", Values: []string{"success", "recipient_blocked", "render_failure", "smtp_permanent_failure", "smtp_transient_failure"}, Required: true}, &core.NumberField{Name: "smtp_code"}, &core.TextField{Name: fieldSanitizedError}, &core.TextField{Name: "raw_error"},
	)
	attempts.Indexes = append(attempts.Indexes, "CREATE UNIQUE INDEX `idx_email_delivery_attempts_request` ON `email_delivery_attempts` (`request`)")
	return attempts
}
