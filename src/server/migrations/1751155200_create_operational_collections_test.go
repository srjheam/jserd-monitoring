package migrations

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestOperationalCollections(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	expectedFields := map[string][]string{
		"collection_cycles":  {"source_mode", fieldStatus, "started_at", "finished_at", "manifest", "error"},
		"snapshots":          {fieldCycle, "previous_snapshot", "schema_version", fieldPayload, "sha256"},
		"events":             {"snapshot", "type", "entity_type", "entity_key", "transition", "occurrence_key", "idempotency_key", fieldPayload, "occurred_at"},
		"event_rule_matches": {fieldEvent, fieldRuleCode, fieldStatus},
	}
	for name, fields := range expectedFields {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("collection %q: %v", name, err)
		}
		if collection.ListRule != nil || collection.ViewRule != nil || collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
			t.Fatalf("collection %q API rules must be locked", name)
		}
		for _, field := range fields {
			if collection.Fields.GetByName(field) == nil {
				t.Errorf("collection %q missing field %q", name, field)
			}
		}
	}

	assertUniqueIndex(t, app, "snapshots", fieldCycle)
	assertUniqueIndex(t, app, "events", "idempotency_key")
	assertUniqueIndex(t, app, "event_rule_matches", fieldEvent, fieldRuleCode)
}

func assertUniqueIndex(t *testing.T, app *tests.TestApp, collectionName string, fields ...string) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range collection.Indexes {
		lower := strings.ToLower(index)
		if !strings.Contains(lower, "unique") {
			continue
		}
		matched := true
		for _, field := range fields {
			matched = matched && strings.Contains(lower, field)
		}
		if matched {
			return
		}
	}
	t.Fatalf("collection %q missing unique index for %v: %v", collectionName, fields, collection.Indexes)
}
