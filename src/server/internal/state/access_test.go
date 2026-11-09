package state

import (
	"encoding/json"
	"server/internal/collections"
	"server/internal/snapshotpayload"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	_ "server/migrations"
)

const (
	fieldPayload       = "payload"
	sourceModeFixtures = "fixtures"
	fieldStatus        = "status"
	fieldEvent         = "event"
	fieldRuleCode      = "rule_code"
	ruleCodeRN1        = "RN1"
	editorIdentity     = "42|editor|1"
	fieldTarget        = "target"
	fieldRequest       = "request"
)

func TestOperationalAccessIsLockedAndAdminReadable(t *testing.T) {
	app := newOperationalTestApp(t)
	for _, name := range []string{collections.CollectionCycles, collections.Snapshots, collections.Events, collections.EventRuleMatches} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatal(err)
		}
		if collection.ListRule != nil || collection.ViewRule != nil || collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
			t.Fatalf("%s must deny guest and ordinary auth API access", name)
		}
		if _, err := app.FindAllRecords(name); err != nil {
			t.Fatalf("admin/backend read %s: %v", name, err)
		}
	}
}

func TestNotificationCollectionsDenyPublicAccess(t *testing.T) {
	app := newOperationalTestApp(t)
	for _, name := range []string{collections.Recipients, collections.Notifications, collections.NotificationTargets, collections.EmailDeliveryRequests, collections.EmailDeliveryClaims, collections.EmailDeliveryAttempts} {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatal(err)
		}
		if collection.ListRule != nil || collection.ViewRule != nil || collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
			t.Fatalf("%s must deny public API access", name)
		}
	}
}

func TestNotificationRecordsAreImmutable(t *testing.T) {
	app := newOperationalTestApp(t)
	records := seedNotificationAuditRecords(t, app)
	RegisterImmutableHooks(app)
	for name, record := range records {
		record.Set("updated", "2026-06-02 00:00:00.000Z")
		if err := app.Save(record); err == nil {
			t.Fatalf("%s update unexpectedly allowed", name)
		}
		if err := app.Delete(record); err == nil {
			t.Fatalf("%s delete unexpectedly allowed", name)
		}
	}
}

func seedNotificationAuditRecords(t *testing.T, app core.App) map[string]*core.Record {
	t.Helper()
	create := func(collectionName string, values map[string]any) *core.Record {
		collection, err := app.FindCollectionByNameOrId(collectionName)
		if err != nil {
			t.Fatal(err)
		}
		record := core.NewRecord(collection)
		for key, value := range values {
			if collectionName == collections.Snapshots && key == fieldPayload {
				payload, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				file, err := snapshotpayload.NewFile(payload)
				if err != nil {
					t.Fatal(err)
				}
				value = file
			}
			record.Set(key, value)
		}
		if err := app.Save(record); err != nil {
			t.Fatalf("save %s: %v", collectionName, err)
		}
		return record
	}
	cycle := create(collections.CollectionCycles, map[string]any{"source_mode": sourceModeFixtures, fieldStatus: "succeeded", "started_at": "2026-06-01 09:00:00.000Z", "finished_at": "2026-06-01 09:01:00.000Z", "manifest": map[string]any{"sourceMode": sourceModeFixtures}})
	snapshot := create(collections.Snapshots, map[string]any{"cycle": cycle.Id, "schema_version": "snapshot:v1", fieldPayload: map[string]any{"sourceMode": sourceModeFixtures}, "sha256": "abc"})
	event := create(collections.Events, map[string]any{"snapshot": snapshot.Id, "type": "review_invitation_accepted", "entity_type": "submission", "entity_key": "42", "transition": "pending->accepted", "occurrence_key": "42|1", "idempotency_key": "immutable-notification", fieldPayload: map[string]any{"title": "Artigo"}, "occurred_at": "2026-06-01 10:00:00.000Z"})
	match := create(collections.EventRuleMatches, map[string]any{fieldEvent: event.Id, fieldRuleCode: ruleCodeRN1, fieldStatus: "pending"})
	recipient := create(collections.Recipients, map[string]any{"identity": editorIdentity, "name_slug": editorIdentity, "is_editor": true, "name": "Editora", "email": "editor@example.test"})
	notification := create(collections.Notifications, map[string]any{fieldEvent: event.Id, "rule_match": match.Id, fieldRuleCode: ruleCodeRN1})
	target := create(collections.NotificationTargets, map[string]any{"notification": notification.Id, "recipient": recipient.Id, "target_kind": "editor", "target_key": editorIdentity})
	request := create(collections.EmailDeliveryRequests, map[string]any{fieldTarget: target.Id})
	claim := create(collections.EmailDeliveryClaims, map[string]any{fieldRequest: request.Id, "claimed_at": "2026-06-01 10:01:00.000Z", "owner_id": "worker"})
	attempt := create(collections.EmailDeliveryAttempts, map[string]any{fieldRequest: request.Id, fieldEvent: event.Id, "rule_match": match.Id, fieldTarget: target.Id, "recipient": recipient.Id, fieldRuleCode: ruleCodeRN1, "target_kind": "editor", "target_key": editorIdentity, "to_address": "editor@example.test", "from_address": "noreply@example.test", "subject": "Assunto", "template_version": "rn1:v1", "template_hash": strings.Repeat("a", 64), fieldStatus: "sent", "result_class": "success"})
	return map[string]*core.Record{"notification": notification, fieldTarget: target, fieldRequest: request, "claim": claim, "attempt": attempt}
}

func TestImmutableOperationalRecordsRejectUpdateAndDelete(t *testing.T) {
	app := newOperationalTestApp(t)
	RegisterImmutableHooks(app)
	collection, err := app.FindCollectionByNameOrId(collections.CollectionCycles)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(collection)
	record.Set("source_mode", sourceModeFixtures)
	record.Set(fieldStatus, "failed")
	record.Set("started_at", "2026-06-01 09:00:00.000Z")
	record.Set("finished_at", "2026-06-01 09:01:00.000Z")
	record.Set("manifest", map[string]any{"version": "1"})
	if err := app.Save(record); err != nil {
		t.Fatalf("create operational record: %v", err)
	}
	record.Set(fieldStatus, "succeeded")
	if err := app.Save(record); err == nil {
		t.Fatal("superuser/backend update unexpectedly allowed")
	}
	if err := app.Delete(record); err == nil {
		t.Fatal("superuser/backend delete unexpectedly allowed")
	}
}
