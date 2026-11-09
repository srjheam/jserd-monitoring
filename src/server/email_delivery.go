package main

import (
	"context"
	"log"
	"os"
	"server/internal/config"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"

	mail "server/internal/email"
)

// newTransport is the e-mail transport for the configured delivery mode. Every
// mode but smtp intercepts the message instead of sending it.
func newTransport(cfg config.Config) mail.Transport {
	if cfg.DeliveryMode == config.DeliveryModeSMTP {
		return mail.SMTPTransport{Mailer: &mailer.SMTPClient{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, TLS: cfg.SMTPTLS}}
	}
	return &mail.MockTransport{Output: os.Stdout}
}

// newWorker builds the notification e-mail worker.
func newWorker(app core.App, cfg config.Config, transport mail.Transport) *mail.Worker {
	replyTo, err := mail.EditorReplyTo(cfg.SMTPReplyTo)
	if err != nil {
		log.Fatalf("invalid configuration: SMTP_REPLY_TO")
	}
	return &mail.Worker{App: app, Transport: transport, Renderer: mail.Renderer{}, OwnerID: "email-delivery-worker", From: cfg.SMTPFromAddress, ReplyTo: replyTo}
}

// newRunDue is the scheduled job that sends every due notification e-mail.
func newRunDue(app core.App, worker *mail.Worker) func() {
	return func() {
		if err := worker.RunDue(context.Background()); err != nil {
			app.Logger().Error("email delivery worker failed", "error", err)
		}
	}
}
