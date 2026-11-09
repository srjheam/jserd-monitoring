package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addWelcomeEmailSupport, removeWelcomeEmailSupport)
}

const (
	notificationsRuleMatchIndex        = "CREATE UNIQUE INDEX `idx_notifications_rule_match` ON `notifications` (`rule_match`)"
	notificationsRuleMatchPartialIndex = "CREATE UNIQUE INDEX `idx_notifications_rule_match` ON `notifications` (`rule_match`) WHERE `rule_match` != ''"
)

// addWelcomeEmailSupport adds recipients.welcomed_at, so "this recipient's
// first e-mail ever" becomes a well-defined question (ADR 0016), and makes
// notifications.rule_match optional, since a welcome notification has no
// event_rule_matches row to point at.
//
// An unset PocketBase relation field stores "" rather than SQL NULL, so a
// plain unique index on rule_match would let only one WELCOME notification
// ever exist across every recipient. The index is replaced with a partial
// one that ignores empty rule_match, leaving the one-notification-per-match
// guarantee in place for every rule that still points at a real match.
func addWelcomeEmailSupport(app core.App) error {
	if err := addRecipientWelcomedAt(app); err != nil {
		return err
	}
	if err := relaxNotificationsRuleMatch(app); err != nil {
		return err
	}
	return relaxAttemptsRuleMatch(app, false)
}

func removeWelcomeEmailSupport(app core.App) error {
	if err := relaxAttemptsRuleMatch(app, true); err != nil {
		return err
	}
	if err := restoreNotificationsRuleMatch(app); err != nil {
		return err
	}
	return removeRecipientWelcomedAt(app)
}

func addRecipientWelcomedAt(app core.App) error {
	recipients, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		return err
	}
	recipients.Fields.Add(&core.DateField{Name: "welcomed_at"})
	return app.Save(recipients)
}

func removeRecipientWelcomedAt(app core.App) error {
	recipients, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		return err
	}
	recipients.Fields.RemoveByName("welcomed_at")
	return app.Save(recipients)
}

func relaxNotificationsRuleMatch(app core.App) error {
	notifications, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		return err
	}
	ruleMatch, ok := notifications.Fields.GetByName("rule_match").(*core.RelationField)
	if !ok {
		return fmt.Errorf("notifications rule_match field is invalid")
	}
	ruleMatch.Required = false
	replaceIndex(notifications, notificationsRuleMatchIndex, notificationsRuleMatchPartialIndex)
	return app.Save(notifications)
}

func restoreNotificationsRuleMatch(app core.App) error {
	notifications, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		return err
	}
	ruleMatch, ok := notifications.Fields.GetByName("rule_match").(*core.RelationField)
	if !ok {
		return fmt.Errorf("notifications rule_match field is invalid")
	}
	ruleMatch.Required = true
	replaceIndex(notifications, notificationsRuleMatchPartialIndex, notificationsRuleMatchIndex)
	return app.Save(notifications)
}

func relaxAttemptsRuleMatch(app core.App, required bool) error {
	attempts, err := app.FindCollectionByNameOrId("email_delivery_attempts")
	if err != nil {
		return err
	}
	ruleMatch, ok := attempts.Fields.GetByName("rule_match").(*core.RelationField)
	if !ok {
		return fmt.Errorf("email delivery attempts rule_match field is invalid")
	}
	ruleMatch.Required = required
	return app.Save(attempts)
}

func replaceIndex(collection *core.Collection, from, to string) {
	for i, index := range collection.Indexes {
		if index == from {
			collection.Indexes[i] = to
			return
		}
	}
}
