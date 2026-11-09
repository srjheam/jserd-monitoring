package migrations

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestNotificationCollectionsSchema(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	for _, name := range []string{"recipients", "notifications", "notification_targets", "email_delivery_requests", "email_delivery_claims", "email_delivery_attempts"} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if collection.ListRule != nil || collection.ViewRule != nil || collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
			t.Fatalf("%s CRUD rules must be closed", name)
		}
	}
	attempts, _ := app.FindCollectionByNameOrId("email_delivery_attempts")
	for _, name := range []string{fieldRequest, fieldEvent, "rule_match", "target", "recipient", fieldRuleCode, "target_kind", "target_key", "to_address", "from_address", "reply_to_addresses", "subject", "template_version", "template_hash", fieldStatus, "result_class", "smtp_code", fieldSanitizedError, "raw_error"} {
		if attempts.Fields.GetByName(name) == nil {
			t.Errorf("attempts field %s missing", name)
		}
	}
	for _, pair := range []struct{ name, index string }{{"notifications", "rule_match"}, {"notification_targets", "target_kind"}, {"email_delivery_requests", "target"}, {"email_delivery_claims", fieldRequest}, {"email_delivery_attempts", fieldRequest}} {
		c, _ := app.FindCollectionByNameOrId(pair.name)
		if !hasUnique(c, pair.index) {
			t.Errorf("%s unique index for %s missing", pair.name, pair.index)
		}
	}
}

func hasUnique(c *core.Collection, fragment string) bool {
	for _, index := range c.Indexes {
		if strings.Contains(index, "UNIQUE") && strings.Contains(index, fragment) {
			return true
		}
	}
	return false
}
