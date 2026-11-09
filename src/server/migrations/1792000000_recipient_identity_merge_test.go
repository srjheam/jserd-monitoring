package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestMergeRecipientIdentityMergesCrossRoleDuplicatesAndDropsUnmailable(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if err := restoreRecipientKindKey(app); err != nil {
		t.Fatal(err)
	}

	seedLegacyRecipients(t, app)

	if err := mergeRecipientIdentity(app); err != nil {
		t.Fatal(err)
	}
	t.Run("cross-role duplicates merge into one recipient", func(t *testing.T) {
		assertMergedRecipient(t, app)
	})
	t.Run("kind/key are replaced by identity fields", func(t *testing.T) {
		assertRecipientIdentityFields(t, app)
	})

	if err := restoreRecipientKindKey(app); err != nil {
		t.Fatal(err)
	}
	t.Run("rollback restores kind and key", func(t *testing.T) {
		assertRecipientKindKeyPresent(t, app)
	})
}

func seedLegacyRecipients(t *testing.T, app core.App) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		t.Fatal(err)
	}
	legacyAdd := func(kind, key, name, email string) {
		record := core.NewRecord(collection)
		record.Set("kind", kind)
		record.Set("key", key)
		record.Set(fieldName, name)
		record.Set("email", email)
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	legacyAdd("editor", "42|editor|1", "Casey Lee", "casey@example.test")
	legacyAdd("reviewer", "casey-reviewer", "Casey Lee", "casey@example.test")
	legacyAdd("editor", "7|editor|1", "Morgan Reed", "")
}

func assertMergedRecipient(t *testing.T, app core.App) {
	t.Helper()
	recipients, err := app.FindAllRecords("recipients")
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) != 1 {
		t.Fatalf("recipients=%d, want 1 (cross-role merge, unmailable dropped): %#v", len(recipients), recipients)
	}
	merged := recipients[0]
	if merged.GetString("identity") != "casey-reviewer" {
		t.Fatalf("identity=%q, want the reviewer username", merged.GetString("identity"))
	}
	if merged.GetString("name_slug") != "casey-lee" {
		t.Fatalf("name_slug=%q", merged.GetString("name_slug"))
	}
	if !merged.GetBool("is_editor") || !merged.GetBool("is_reviewer") {
		t.Fatalf("is_editor=%v is_reviewer=%v, want both true", merged.GetBool("is_editor"), merged.GetBool("is_reviewer"))
	}
}

func assertRecipientIdentityFields(t *testing.T, app core.App) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"kind", "key"} {
		if collection.Fields.GetByName(field) != nil {
			t.Fatalf("field %s survived the up migration", field)
		}
	}
	for _, field := range []string{"identity", "name_slug", "is_editor", "is_reviewer"} {
		if collection.Fields.GetByName(field) == nil {
			t.Fatalf("field %s missing after migration", field)
		}
	}
}

func assertRecipientKindKeyPresent(t *testing.T, app core.App) {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("recipients")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"kind", "key"} {
		if collection.Fields.GetByName(field) == nil {
			t.Fatalf("rollback must restore field %s", field)
		}
	}
}
