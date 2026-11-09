package main

import (
	netmail "net/mail"
	"server/internal/config"

	"github.com/pocketbase/pocketbase/core"
)

// registerSMTPSettings points PocketBase's own mailer at the same SMTP_*
// credentials internal/email uses, so a chief_editors password reset e-mail
// sends without a Superuser. Enabled follows whether SMTP credentials are configured,
// not cfg.DeliveryMode: that flag governs internal/email's mock-vs-real
// transport only, and the two mail paths must stay independent.
func registerSMTPSettings(app core.App, cfg config.Config) {
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		return applySMTPSettings(e.App, cfg)
	})
}

// applySMTPSettings is registerSMTPSettings' effect, split out so tests can
// apply it directly against an already-bootstrapped test app.
func applySMTPSettings(app core.App, cfg config.Config) error {
	settings := app.Settings()
	settings.SMTP.Enabled = cfg.SMTPHost != ""
	settings.SMTP.Host = cfg.SMTPHost
	settings.SMTP.Port = cfg.SMTPPort
	settings.SMTP.Username = cfg.SMTPUsername
	settings.SMTP.Password = cfg.SMTPPassword
	settings.SMTP.TLS = cfg.SMTPTLS
	// config.Validate already requires SMTPFromAddress to parse as a mail
	// address; Settings().Meta.SenderAddress rejects the "Name <addr>" form
	// internal/email's From header accepts, so only the bare address carries
	// over here.
	if from, err := netmail.ParseAddress(cfg.SMTPFromAddress); err == nil {
		settings.Meta.SenderAddress = from.Address
		if from.Name != "" {
			settings.Meta.SenderName = from.Name
		}
	}
	// AppURL feeds the {APP_URL} placeholder in every PocketBase system
	// e-mail, including the chief_editors reset-password link (see the
	// 1790000000 migration). Left at PocketBase's own default when unset.
	if cfg.AppURL != "" {
		settings.Meta.AppURL = cfg.AppURL
	}
	return app.Save(settings)
}
