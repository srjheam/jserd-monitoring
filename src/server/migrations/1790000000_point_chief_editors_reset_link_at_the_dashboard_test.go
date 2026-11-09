package migrations

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestChiefEditorsResetLinkPointsAtTheDashboard(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	chiefEditors, err := app.FindCollectionByNameOrId("chief_editors")
	if err != nil {
		t.Fatal(err)
	}
	body := chiefEditors.ResetPasswordTemplate.Body
	if strings.Contains(body, "/_/#/auth/confirm-password-reset/") {
		t.Fatal("reset link must not point at the Superuser-only admin UI")
	}
	if !strings.Contains(body, "/reset-password?token={TOKEN}") {
		t.Fatalf("reset link must point at the dashboard's confirm-reset page, got: %s", body)
	}

	if err := revertChiefEditorsResetLink(app); err != nil {
		t.Fatal(err)
	}
	reverted, err := app.FindCollectionByNameOrId("chief_editors")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reverted.ResetPasswordTemplate.Body, "/_/#/auth/confirm-password-reset/") {
		t.Fatal("rollback must restore PocketBase's default reset link")
	}

	if err := pointChiefEditorsResetLinkAtTheDashboard(app); err != nil {
		t.Fatal(err)
	}
}
