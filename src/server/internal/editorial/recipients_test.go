package editorial

import (
	"context"
	"errors"
	"server/internal/collections"
	"server/internal/testseed"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "server/migrations"
)

type chiefSource struct {
	groups []ChiefEditorGroup
	err    error
}

func (s chiefSource) Groups(context.Context) ([]ChiefEditorGroup, error) { return s.groups, s.err }

func TestRecipientResolverPrefersUnambiguousOJS(t *testing.T) {
	app := editorialTestApp(t)
	seedChief(t, app, "Fallback", "fallback@example.test")
	r := RecipientResolver{App: app, OJS: chiefSource{groups: []ChiefEditorGroup{{ID: testGroupIDChief, Name: "Chefes", Specific: true, Members: []ChiefEditor{{Name: "OJS", Email: "ojs@example.test"}}}}}, GroupID: testGroupIDChief}
	got, err := r.Resolve(context.Background())
	if err != nil || len(got.Valid) != 1 || got.Valid[0].Address != "ojs@example.test" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestRecipientResolverFallsBackAndBlocksInvalid(t *testing.T) {
	for _, source := range []ChiefEditorSource{chiefSource{err: errors.New("unavailable")}, chiefSource{groups: []ChiefEditorGroup{{ID: testGroupIDChief, Specific: true}, {ID: testGroupIDChief, Specific: true}}}} {
		app := editorialTestApp(t)
		seedChief(t, app, "Válida", "valid@example.test")
		got, err := (RecipientResolver{App: app, OJS: source, GroupID: testGroupIDChief}).Resolve(context.Background())
		if err != nil || len(got.Valid) != 1 {
			t.Fatalf("got=%#v err=%v", got, err)
		}
	}
	app := editorialTestApp(t)
	got, err := (RecipientResolver{App: app, OJS: chiefSource{groups: []ChiefEditorGroup{{ID: testGroupIDChief, Specific: true, Members: []ChiefEditor{{Email: "valid@example.test"}, {Email: "bad"}}}}}, GroupID: testGroupIDChief}).Resolve(context.Background())
	if err != nil || len(got.Valid) != 1 || len(got.Blocked) != 1 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func editorialTestApp(t *testing.T) *tests.TestApp {
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

func seedChief(t *testing.T, app core.App, name, address string) {
	t.Helper()
	c, _ := app.FindCollectionByNameOrId(collections.EditorialRecipients)
	r := core.NewRecord(c)
	r.Set("name", name)
	r.Set("email", address)
	r.Set("active", true)
	if err := app.Save(r); err != nil {
		t.Fatal(err)
	}
}
