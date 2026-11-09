package email

import (
	"net/mail"
	"server/internal/collections"
	"server/internal/testseed"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "server/migrations"
)

func emailTestApp(t *testing.T) *tests.TestApp {
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

func TestReviewerReplyToUsesFrozenEditorAssociations(t *testing.T) {
	app := emailTestApp(t)
	collection, err := app.FindCollectionByNameOrId(collections.Recipients)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ key, name, address string }{
		{key: "editor-1", name: "Antiga", address: "old@example.test"},
		{key: "editor-2", name: "Atual", address: "current@example.test"},
	} {
		record := core.NewRecord(collection)
		record.Set("identity", item.key)
		record.Set("name_slug", item.key)
		record.Set("is_editor", true)
		record.Set("name", item.name)
		record.Set("email", item.address)
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	worker := &Worker{App: app, ReplyTo: []string{"fallback@example.test"}}
	replyTo, err := worker.reviewerReplyTo([]string{"editor-2"})
	if err != nil || len(replyTo) != 1 {
		t.Fatalf("replyTo=%v err=%v", replyTo, err)
	}
	address, err := mail.ParseAddress(replyTo[0])
	if err != nil || address.Name != "Atual" || address.Address != "current@example.test" {
		t.Fatalf("replyTo=%v parsed=%v err=%v", replyTo, address, err)
	}
}
