package main

import (
	"server/internal/config"
	mail "server/internal/email"
	"testing"
)

func TestNewTransportPicksSMTPOnlyForSMTPDeliveryMode(t *testing.T) {
	smtp := newTransport(config.Config{DeliveryMode: config.DeliveryModeSMTP, SMTPHost: "smtp.example.test"})
	if _, ok := smtp.(mail.SMTPTransport); !ok {
		t.Fatalf("expected an SMTPTransport, got %T", smtp)
	}

	mock := newTransport(config.Config{DeliveryMode: "console"})
	if _, ok := mock.(*mail.MockTransport); !ok {
		t.Fatalf("expected a MockTransport, got %T", mock)
	}
}

func TestNewWorkerAppliesEditorReplyToPolicy(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()

	worker := newWorker(app, config.Config{SMTPFromAddress: "noreply@example.test", SMTPReplyTo: "JSERD <editorial@example.test>"}, &mail.MockTransport{})
	if worker == nil {
		t.Fatal("newWorker returned nil")
	}
	if len(worker.ReplyTo) != 1 || worker.ReplyTo[0] != `"JSERD" <editorial@example.test>` {
		t.Fatalf("ReplyTo=%v", worker.ReplyTo)
	}
}

func TestNewRunDueRunsTheWorkerOnce(t *testing.T) {
	app := serverTestApp(t)
	defer app.Cleanup()

	worker := newWorker(app, config.Config{SMTPFromAddress: "noreply@example.test"}, &mail.MockTransport{})
	runDue := newRunDue(app, worker)
	runDue()
}
