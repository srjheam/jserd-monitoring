package editorial

import (
	"encoding/json"
	"server/internal/collections"
	"server/internal/normalizer"
	"server/internal/snapshotpayload"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	_ "server/migrations"
)

func TestPocketBaseDeliveryLookupRN4Targets(t *testing.T) {
	app := editorialTestApp(t)
	snapshot := saveEvidenceSnapshot(t, app, "", "42")
	event := saveRecord(t, app, collections.Events, map[string]any{fieldSnapshot: snapshot.Id, "type": "reviews_completed", "entity_type": "submission", "entity_key": "42", "transition": "completed", "occurrence_key": "rn4", "idempotency_key": "rn4-event", "occurred_at": time.Now(), fieldPayload: map[string]any{"submissionId": "42"}})
	match := saveRecord(t, app, collections.EventRuleMatches, map[string]any{fieldEvent: event.Id, fieldRuleCode: ruleRN4, fieldStatus: "pending"})
	notification := saveRecord(t, app, collections.Notifications, map[string]any{fieldEvent: event.Id, "rule_match": match.Id, fieldRuleCode: ruleRN4})
	recipient := saveRecord(t, app, collections.Recipients, map[string]any{"identity": "editor-1", "name_slug": "editor-1", "is_editor": true, "email": testChiefEmail})
	target := saveRecord(t, app, collections.NotificationTargets, map[string]any{"notification": notification.Id, "recipient": recipient.Id, "target_kind": targetKindEditor, "target_key": "42|editor|1"})
	request := saveRecord(t, app, collections.EmailDeliveryRequests, map[string]any{"target": target.Id})
	lookup := PocketBaseDeliveryLookup{App: app}
	if unsent, known := lookup.HasUnsentRN4("42"); !known || !unsent {
		t.Fatalf("before sent: unsent=%v known=%v", unsent, known)
	}
	saveRecord(t, app, collections.EmailDeliveryAttempts, map[string]any{"request": request.Id, fieldEvent: event.Id, "rule_match": match.Id, "target": target.Id, "recipient": recipient.Id, fieldRuleCode: ruleRN4, "target_kind": targetKindEditor, "target_key": "42|editor|1", "from_address": "noreply@example.test", "template_version": "rn4:v1", "template_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "status": "sent", "result_class": "success"})
	if unsent, known := lookup.HasUnsentRN4("42"); !known || unsent {
		t.Fatalf("after sent: unsent=%v known=%v", unsent, known)
	}
}

func saveEvidenceSnapshot(t *testing.T, app core.App, previous, submissionID string) *core.Record {
	t.Helper()
	return saveEvidenceSnapshotAt(t, app, previous, submissionID, time.Now())
}

func saveEvidenceSnapshotAt(t *testing.T, app core.App, previous, submissionID string, finishedAt time.Time) *core.Record {
	t.Helper()
	cycle := saveRecord(t, app, collections.CollectionCycles, map[string]any{fieldSourceMode: sourceModeFixtures, fieldStatus: statusSucceeded, fieldStartedAt: finishedAt, fieldFinishedAt: finishedAt, fieldManifest: map[string]any{fieldSourceMode: sourceModeFixtures}})
	payload, err := json.Marshal(normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{OJSSubmissionID: submissionID}}})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]any{fieldCycle: cycle.Id, "schema_version": "v1", fieldPayload: payload, "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if previous != "" {
		values["previous_snapshot"] = previous
	}
	return saveRecord(t, app, collections.Snapshots, values)
}

func saveRecord(t *testing.T, app core.App, collection string, values map[string]any) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(c)
	for key, value := range values {
		if collection == collections.Snapshots && key == fieldPayload {
			payload, ok := value.([]byte)
			if !ok {
				t.Fatalf("snapshot payload must be []byte, got %T", value)
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
		t.Fatalf("save %s: %v", collection, err)
	}
	return record
}
