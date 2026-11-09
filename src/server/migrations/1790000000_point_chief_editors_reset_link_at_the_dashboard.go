package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(pointChiefEditorsResetLinkAtTheDashboard, revertChiefEditorsResetLink)
}

// chiefEditorsResetPasswordBody is PocketBase's default reset-password body,
// with its link pointed at the client dashboard's own confirm-reset page
// instead of the default `/_/#/auth/confirm-password-reset/{TOKEN}`. The
// default link opens the Superuser-only admin UI, which Caddy blocks from
// the public internet, so a chief editor could never
// open it.
const chiefEditorsResetPasswordBody = `<p>Hello,</p>
<p>Click on the button below to reset your password.</p>
<p>
  <a class="btn" href="` + core.EmailPlaceholderAppURL + "/reset-password?token=" + core.EmailPlaceholderToken + `" target="_blank" rel="noopener">Reset password</a>
</p>
<p><i>If you didn't ask to reset your password, please ignore this email.</i></p>
<p>
  Thanks,<br/>
  ` + core.EmailPlaceholderAppName + ` team
</p>`

// defaultResetPasswordBody is PocketBase's own default, kept here only so
// the rollback can restore it (core.EmailTemplate defaults are unexported).
const defaultResetPasswordBody = `<p>Hello,</p>
<p>Click on the button below to reset your password.</p>
<p>
  <a class="btn" href="` + core.EmailPlaceholderAppURL + "/_/#/auth/confirm-password-reset/" + core.EmailPlaceholderToken + `" target="_blank" rel="noopener">Reset password</a>
</p>
<p><i>If you didn't ask to reset your password, please ignore this email.</i></p>
<p>
  Thanks,<br/>
  ` + core.EmailPlaceholderAppName + ` team
</p>`

func pointChiefEditorsResetLinkAtTheDashboard(app core.App) error {
	return setChiefEditorsResetBody(app, chiefEditorsResetPasswordBody)
}

func revertChiefEditorsResetLink(app core.App) error {
	return setChiefEditorsResetBody(app, defaultResetPasswordBody)
}

func setChiefEditorsResetBody(app core.App, body string) error {
	chiefEditors, err := app.FindCollectionByNameOrId("chief_editors")
	if err != nil {
		return err
	}
	chiefEditors.ResetPasswordTemplate.Body = body
	return app.Save(chiefEditors)
}
