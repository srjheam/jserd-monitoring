package migrations

import (
	"server/internal/snapshotpayload"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestWelcomeEmailMigrationSchema(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	recipients, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		t.Fatal(err)
	}
	if recipients.Fields.GetByName("welcomed_at") == nil {
		t.Fatal("recipients.welcomed_at missing")
	}

	notifications, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		t.Fatal(err)
	}
	ruleMatch, isRelationField := notifications.Fields.GetByName("rule_match").(*core.RelationField)
	if !isRelationField || ruleMatch.Required {
		t.Fatalf("rule_match=%#v, want an optional relation field", ruleMatch)
	}

	attempts, err := app.FindCollectionByNameOrId("email_delivery_attempts")
	if err != nil {
		t.Fatal(err)
	}
	attemptsRuleMatch, ok := attempts.Fields.GetByName("rule_match").(*core.RelationField)
	if !ok || attemptsRuleMatch.Required {
		t.Fatalf("attempts rule_match=%#v, want an optional relation field", attemptsRuleMatch)
	}
}

// TestTwoEmptyRuleMatchNotificationsBothSave proves the assumption the
// ticket asked to verify (a plain unique index treats two "" relation
// values as equal, not distinct) does not hold in this PocketBase/SQLite
// version. The partial index in addWelcomeEmailSupport is what makes this
// test pass.
func TestTwoEmptyRuleMatchNotificationsBothSave(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	event := createWelcomeEmailTestEvent(t, app)

	notifications, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		record := core.NewRecord(notifications)
		record.Set(fieldEvent, event.Id)
		record.Set(fieldRuleCode, "WELCOME")
		if err := app.Save(record); err != nil {
			t.Fatalf("notification %d: %v", i, err)
		}
	}
}

func TestRuleMatchStillUniqueForRealMatches(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	event := createWelcomeEmailTestEvent(t, app)

	matches, err := app.FindCollectionByNameOrId("event_rule_matches")
	if err != nil {
		t.Fatal(err)
	}
	match := core.NewRecord(matches)
	match.Set(fieldEvent, event.Id)
	match.Set(fieldRuleCode, "RN1")
	match.Set(fieldStatus, "pending")
	if err := app.Save(match); err != nil {
		t.Fatal(err)
	}

	notifications, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		t.Fatal(err)
	}
	first := core.NewRecord(notifications)
	first.Set(fieldEvent, event.Id)
	first.Set("rule_match", match.Id)
	first.Set(fieldRuleCode, "RN1")
	if err := app.Save(first); err != nil {
		t.Fatal(err)
	}
	second := core.NewRecord(notifications)
	second.Set(fieldEvent, event.Id)
	second.Set("rule_match", match.Id)
	second.Set(fieldRuleCode, "RN1")
	if err := app.Save(second); err == nil {
		t.Fatal("expected a second notification for the same rule_match to be rejected")
	}
}

func createWelcomeEmailTestEvent(t *testing.T, app core.App) *core.Record {
	t.Helper()
	cycle := createWelcomeEmailTestCycle(t, app)
	snapshot := createWelcomeEmailTestSnapshot(t, app, cycle)
	return createWelcomeEmailTestEventRecord(t, app, snapshot)
}

func createWelcomeEmailTestCycle(t *testing.T, app core.App) *core.Record {
	t.Helper()
	cycles, _ := app.FindCollectionByNameOrId("collection_cycles")
	cycle := core.NewRecord(cycles)
	cycle.Set("source_mode", sourceModeFixtures)
	cycle.Set(fieldStatus, "succeeded")
	cycle.Set("started_at", "2026-06-01 09:00:00.000Z")
	cycle.Set("finished_at", "2026-06-01 09:01:00.000Z")
	cycle.Set("manifest", map[string]any{"sourceMode": sourceModeFixtures})
	if err := app.Save(cycle); err != nil {
		t.Fatal(err)
	}
	return cycle
}

func createWelcomeEmailTestSnapshot(t *testing.T, app core.App, cycle *core.Record) *core.Record {
	t.Helper()
	snapshots, _ := app.FindCollectionByNameOrId("snapshots")
	snapshot := core.NewRecord(snapshots)
	snapshot.Set(fieldCycle, cycle.Id)
	snapshot.Set("schema_version", "snapshot:v1")
	payload, err := snapshotpayload.NewFile([]byte(`{"sourceMode":"fixtures"}`))
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Set(fieldPayload, payload)
	snapshot.Set("sha256", "abc")
	if err := app.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func createWelcomeEmailTestEventRecord(t *testing.T, app core.App, snapshot *core.Record) *core.Record {
	t.Helper()
	events, _ := app.FindCollectionByNameOrId("events")
	event := core.NewRecord(events)
	event.Set("snapshot", snapshot.Id)
	event.Set("type", "welcome_email_test")
	event.Set("entity_type", "submission")
	event.Set("entity_key", "42")
	event.Set("transition", "observed")
	event.Set("occurrence_key", "welcome-email-test")
	event.Set("idempotency_key", "welcome-email-test")
	event.Set(fieldPayload, map[string]any{"sourceMode": sourceModeFixtures})
	event.Set("occurred_at", "2026-06-01 10:00:00.000Z")
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}
	return event
}
