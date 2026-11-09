package notifications

import (
	"server/internal/collections"
	"server/internal/events"
	"server/internal/testseed"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "server/migrations"
)

const (
	targetKindEditor = "editor"
	testEditorKey2   = "editor-2"
	testEditorKey1   = "editor-1"
)

func TestEditorTargetsUsePersistedCSVAssociations(t *testing.T) {
	app := recipientApp(t)
	addRecipient(t, app, targetKindEditor, testEditorKey1, "same@example.test")
	addRecipient(t, app, targetKindEditor, testEditorKey2, "same@example.test")
	addRecipient(t, app, targetKindEditor, "editor-3", "other@example.test")
	records, kind, err := resolveRecipients(app, "RN2", "", []string{testEditorKey1, testEditorKey2})
	if err != nil {
		t.Fatal(err)
	}
	if kind != targetKindEditor || len(records) != 2 || records[0].Record.Id == records[1].Record.Id {
		t.Fatalf("editor targets=%#v kind=%s", records, kind)
	}
}

func TestEditorTargetsUseFrozenEventAssociations(t *testing.T) {
	app := recipientApp(t)
	addRecipient(t, app, targetKindEditor, testEditorKey1, "old@example.test")
	addRecipient(t, app, targetKindEditor, testEditorKey2, "current@example.test")
	records, kind, err := resolveRecipients(app, "RN3", "", []string{testEditorKey2})
	if err != nil || kind != targetKindEditor || len(records) != 1 || records[0].TargetKey != testEditorKey2 {
		t.Fatalf("records=%#v kind=%s err=%v", records, kind, err)
	}
}

func TestRN18TargetsOnlyTheSubmissionsEditors(t *testing.T) {
	app := recipientApp(t)
	addRecipient(t, app, targetKindEditor, testEditorKey1, "first@example.test")
	addRecipient(t, app, targetKindEditor, testEditorKey2, "second@example.test")
	addRecipient(t, app, targetKindEditor, "unrelated", "other@example.test")
	records, kind, err := resolveRecipients(app, events.RuleRN18, "", []string{testEditorKey1, testEditorKey2})
	if err != nil || kind != targetKindEditor || len(records) != 2 || records[0].TargetKey != testEditorKey1 || records[1].TargetKey != testEditorKey2 {
		t.Fatalf("targets=%#v kind=%s err=%v", records, kind, err)
	}
}

func TestRN18WithoutAssignedEditorsDoesNotUseSubmissionIDAsARecipient(t *testing.T) {
	app := recipientApp(t)
	addRecipient(t, app, targetKindEditor, "42", "unrelated@example.test")
	records, _, err := resolveRecipients(app, events.RuleRN18, "", nil)
	if err != nil || len(records) != 0 {
		t.Fatalf("targets=%#v err=%v", records, err)
	}
}

func TestRN1ToRN4EditorTargets(t *testing.T) {
	app := recipientApp(t)
	addRecipient(t, app, targetKindEditor, testEditorKey1, "editor@example.test")
	for _, rule := range []string{"RN1", "RN2", "RN3", "RN4"} {
		records, kind, err := resolveRecipients(app, rule, "", []string{testEditorKey1})
		if err != nil || kind != targetKindEditor || len(records) != 1 {
			t.Fatalf("%s targets=%#v kind=%s err=%v", rule, records, kind, err)
		}
	}
}

func TestReviewerTargetsForRN6ToRN8(t *testing.T) {
	app := recipientApp(t)
	addRecipient(t, app, "reviewer", "reviewer-1", "reviewer@example.test")
	for _, rule := range []string{"RN6", "RN7", "RN8"} {
		records, kind, err := resolveRecipients(app, rule, "reviewer-1", nil)
		if err != nil || kind != "reviewer" || len(records) != 1 {
			t.Fatalf("%s targets=%#v kind=%s err=%v", rule, records, kind, err)
		}
	}
}

func recipientApp(t *testing.T) *tests.TestApp {
	t.Helper()
	dir, err := testseed.Dir()
	if err != nil {
		t.Fatal(err)
	}
	app, err := tests.NewTestApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)
	return app
}

func addRecipient(t *testing.T, app core.App, kind, key, address string) {
	t.Helper()
	collection, _ := app.FindCollectionByNameOrId(collections.Recipients)
	record := core.NewRecord(collection)
	record.Set("identity", key)
	record.Set("name_slug", key)
	record.Set("is_editor", kind == targetKindEditor)
	record.Set("is_reviewer", kind == "reviewer")
	record.Set("email", address)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}
}
