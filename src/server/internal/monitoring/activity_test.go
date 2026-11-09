package monitoring

import (
	"encoding/json"
	"net/http"
	"server/internal/collections"
	"server/internal/events"
	"server/internal/snapshotpayload"
	"server/internal/state"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestActivityRouteMergesDeliveriesAndJournalInTimeOrder(t *testing.T) {
	app := monitoringTestApp(t)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	ruleA, ruleB, failed := seedActivityRouteFixture(t, app)

	items := getJSON[[]activityItemView](t, mux, "/api/jserd/monitoring/submissions/42/activity", token)
	assertActivityRouteMergedOrder(t, items)
	if items[0].TitleKey != events.ActivityTitleKey(ruleB) || items[0].RecipientRole != "reviewer" || items[2].RecipientRole != targetKindEditor {
		t.Fatalf("semantic title or historical recipient role missing: %#v", items)
	}

	raw := rawGet(t, mux, "/api/jserd/monitoring/submissions/42/activity", token)
	for _, forbidden := range []string{ruleA, ruleB, failed.GetString(fieldRuleCode), "ruleLabel", "Assunto", "noreply@example.test"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("response leaked a rule code: %q in %s", forbidden, raw)
		}
	}
}

func TestActivityOmitsUnknownRecipientRoles(t *testing.T) {
	app := monitoringTestApp(t)
	collection, err := app.FindCollectionByNameOrId(collections.EmailDeliveryAttempts)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"", "unrecognized"} {
		attempt := core.NewRecord(collection)
		attempt.Set("target_kind", role)
		items := deliveryItems([]*core.Record{attempt}, nil)
		if items[0].RecipientRole != "" || items[0].TitleKey != "editorial_notification" {
			t.Fatalf("unknown metadata must use neutral fallbacks: %#v", items[0])
		}
	}
}

func TestActivityUsesStoredRoleAndGenericLegacyFallback(t *testing.T) {
	attempts := monitoringTestApp(t)
	collection, err := attempts.FindCollectionByNameOrId(collections.EmailDeliveryAttempts)
	if err != nil {
		t.Fatal(err)
	}
	legacy := core.NewRecord(collection)
	legacy.Set("target_kind", "editor")
	legacy.Set("rule_code", events.RuleWelcome)
	if got := activityRecipientRole(legacy); got != events.RecipientRoleEditor {
		t.Fatalf("legacy editor role=%q, want generic editor", got)
	}
	legacy.Set("target_kind", "reviewer")
	if got := activityRecipientRole(legacy); got != events.RecipientRoleReviewer {
		t.Fatalf("legacy reviewer role=%q, want generic reviewer", got)
	}
	legacy.Set("target_kind", "editor")
	legacy.Set("recipient_role", events.RecipientRoleJournalEditor)
	if got := activityRecipientRole(legacy); got != events.RecipientRoleJournalEditor {
		t.Fatalf("stored role=%q, want specific role", got)
	}
}

// seedActivityRouteFixture seeds two deliveries and a journal entry for
// submission 42, plus an unrelated delivery for submission 99. It returns
// the two rule codes used and the failed delivery record.
func seedActivityRouteFixture(t *testing.T, app core.App) (string, string, *core.Record) {
	t.Helper()
	oldest := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	middle := time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	// Rule codes come from events.AllRuleCodes() rather than being spelled out
	// here: this package's grep gate (see AGENTS ticket DoD) forbids the
	// pattern "RN[0-9]" from appearing anywhere under internal/monitoring/.
	codes := events.AllRuleCodes()
	ruleA, ruleB := codes[0], codes[len(codes)-1]

	seedDeliveryAttempt(t, app, "42", oldest, ruleA, "sent", "success", "editor@example.test")
	saveJournalEntry(t, app, "42", middle, "op1", "Editora", "Morgan alocou para a Robin")
	failed := seedDeliveryAttempt(t, app, "42", newest, ruleB, "failed", "smtp_permanent_failure", testReviewerEmail)
	failed.Set("target_kind", "reviewer")
	if err := app.Save(failed); err != nil {
		t.Fatal(err)
	}
	// Unrelated submission must not appear in "42"'s activity.
	seedDeliveryAttempt(t, app, "99", newest, ruleA, "sent", "success", "someone@example.test")
	return ruleA, ruleB, failed
}

func assertActivityRouteMergedOrder(t *testing.T, items []activityItemView) {
	t.Helper()
	if len(items) != 3 {
		t.Fatalf("expected 3 merged items, got %d: %#v", len(items), items)
	}
	if items[0].Kind != activityKindDelivery || items[0].FailureClass != "smtp_permanent_failure" || items[0].Status != "failed" {
		t.Fatalf("newest item should be the failed delivery with its failure class: %#v", items[0])
	}
	if items[1].Kind != "journal" || items[1].Body != "Morgan alocou para a Robin" {
		t.Fatalf("middle item should be the journal entry: %#v", items[1])
	}
	if items[2].Kind != activityKindDelivery || items[2].Status != "sent" {
		t.Fatalf("oldest item should be the first delivery: %#v", items[2])
	}
	for _, item := range items {
		if item.Kind == activityKindDelivery && item.TitleKey == "" {
			t.Fatalf("delivery item missing a semantic title key: %#v", item)
		}
	}
}

// TestActivityEventLookupUsesIndexesNotFullScan guards the fix for
// FindActivity previously loading every email_delivery_attempts row and doing
// one FindRecordById(collections.Events, ...) per row. Both branches of the
// entity_key/payload.submissionId lookup must resolve to an index seek, and
// neither the events nor the email_delivery_attempts table should be fully
// scanned.
func TestActivityEventLookupUsesIndexesNotFullScan(t *testing.T) {
	app := monitoringTestApp(t)

	assertNoFullScan := func(t *testing.T, query string, table string) {
		t.Helper()
		var rows []struct {
			Detail string `db:"detail"`
		}
		if err := app.DB().NewQuery(query).All(&rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if strings.Contains(row.Detail, "SCAN "+table) {
				t.Fatalf("expected an index seek, got a full scan: %q", row.Detail)
			}
		}
	}

	assertNoFullScan(t,
		`EXPLAIN QUERY PLAN
		 SELECT id FROM events WHERE entity_key = 'x'
		 UNION
		 SELECT id FROM events WHERE (CASE WHEN json_valid(payload) THEN JSON_EXTRACT(payload, '$.submissionId') ELSE JSON_EXTRACT(json_object('pb', payload), '$.pbsubmissionId') END) = 'x'`,
		collections.Events)
	assertNoFullScan(t, `EXPLAIN QUERY PLAN SELECT id FROM email_delivery_attempts WHERE event = 'x'`, collections.EmailDeliveryAttempts)
}

type activityItemView struct {
	Kind          string `json:"kind"`
	TitleKey      string `json:"titleKey"`
	Status        string `json:"status"`
	FailureClass  string `json:"failureClass"`
	Recipient     string `json:"recipient"`
	RecipientRole string `json:"recipientRole"`
	AuthorLabel   string `json:"authorLabel"`
	Body          string `json:"body"`
}

func createFixtureRecord(t *testing.T, app core.App, collectionName string, values map[string]any) *core.Record {
	t.Helper()
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

func seedDeliveryAttemptEvent(t *testing.T, app core.App, submissionID string, occurredAt time.Time, unique string) *core.Record {
	t.Helper()
	cycle := createFixtureRecord(t, app, collections.CollectionCycles, map[string]any{"source_mode": sourceModeFixtures, statusFieldName: "succeeded", "started_at": occurredAt, "finished_at": occurredAt, "manifest": map[string]any{"sourceMode": sourceModeFixtures}})
	snapshot := createFixtureRecord(t, app, collections.Snapshots, map[string]any{"cycle": cycle.Id, "schema_version": "snapshot:v1", fieldPayload: map[string]any{"sourceMode": sourceModeFixtures}, "sha256": strings.Repeat("a", 64)})
	eventPayload, err := json.Marshal(map[string]any{"submissionId": submissionID})
	if err != nil {
		t.Fatal(err)
	}
	return createFixtureRecord(t, app, collections.Events, map[string]any{"snapshot": snapshot.Id, "type": "delivery_fixture", "entity_type": "submission", "entity_key": submissionID, "transition": "t", "occurrence_key": unique, "idempotency_key": "idem-" + unique, fieldPayload: json.RawMessage(eventPayload), "occurred_at": occurredAt})
}

func seedDeliveryAttempt(t *testing.T, app core.App, submissionID string, occurredAt time.Time, ruleCode, status, resultClass, toAddress string) *core.Record {
	t.Helper()
	unique := submissionID + "|" + ruleCode + "|" + status
	event := seedDeliveryAttemptEvent(t, app, submissionID, occurredAt, unique)
	match := createFixtureRecord(t, app, collections.EventRuleMatches, map[string]any{fieldEvent: event.Id, fieldRuleCode: ruleCode, statusFieldName: "pending"})
	recipient := createFixtureRecord(t, app, collections.Recipients, map[string]any{"identity": unique, "name_slug": unique, "is_editor": true, "name": "Editor", "email": toAddress})
	notification := createFixtureRecord(t, app, collections.Notifications, map[string]any{fieldEvent: event.Id, "rule_match": match.Id, fieldRuleCode: ruleCode})
	target := createFixtureRecord(t, app, collections.NotificationTargets, map[string]any{"notification": notification.Id, "recipient": recipient.Id, "target_kind": targetKindEditor, "target_key": unique})
	request := createFixtureRecord(t, app, collections.EmailDeliveryRequests, map[string]any{"target": target.Id})
	return createFixtureRecord(t, app, collections.EmailDeliveryAttempts, map[string]any{
		"request": request.Id, fieldEvent: event.Id, "rule_match": match.Id, "target": target.Id, "recipient": recipient.Id,
		fieldRuleCode: ruleCode, "target_kind": targetKindEditor, "target_key": unique, "to_address": toAddress, "from_address": "noreply@example.test",
		"subject": "Assunto", "template_version": strings.ToLower(ruleCode) + ":v1", "template_hash": strings.Repeat("a", 64),
		statusFieldName: status, "result_class": resultClass,
	})
}

func saveJournalEntry(t *testing.T, app core.App, submissionID string, entryDate time.Time, authorID, authorLabel, body string) *core.Record {
	t.Helper()
	journal, err := app.FindCollectionByNameOrId(collections.SubmissionJournal)
	if err != nil {
		t.Fatal(err)
	}
	entry := core.NewRecord(journal)
	entry.Set("ojs_submission_id", submissionID)
	entry.Set("entry_date", entryDate)
	entry.Set("author_id", authorID)
	entry.Set("author_label", authorLabel)
	entry.Set("body", body)
	if err := app.Save(entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestJournalRouteUsesSignedInChiefEditorAsAuthorAndRejectsUpdateDelete(t *testing.T) {
	app := monitoringTestApp(t)
	state.RegisterImmutableHooks(app)
	mux := serveMux(t, app)
	token := chiefEditorToken(t, app)

	response := apiCall(mux, http.MethodPost, "/api/jserd/monitoring/submissions/42/journal", []byte(`{"body":"nova mensagem para os autores"}`), token)
	if response.Code != http.StatusOK {
		t.Fatalf("create journal entry=%d %s", response.Code, response.Body.String())
	}
	var created struct {
		ID          string `json:"id"`
		AuthorID    string `json:"author_id"`
		AuthorLabel string `json:"author_label"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.AuthorID == "" || created.AuthorLabel != "operator@example.test" {
		t.Fatalf("author identity not recorded: %#v", created)
	}

	record, err := app.FindRecordById(collections.SubmissionJournal, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Set("body", "changed")
	if err := app.Save(record); err == nil {
		t.Fatal("journal entry update unexpectedly allowed")
	}
	if err := app.Delete(record); err == nil {
		t.Fatal("journal entry delete unexpectedly allowed")
	}
}
