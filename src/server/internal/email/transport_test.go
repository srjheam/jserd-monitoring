package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tools/mailer"
)

type captureMailer struct {
	message *mailer.Message
	err     error
}

func TestMockTransportRedactsAllDynamicFields(t *testing.T) {
	var output bytes.Buffer
	fields := []SensitiveField{
		{Name: "recipient_name", Value: "Pessoa Real"},
		{Name: "submission_id", Value: "SUBMISSION-42"},
		{Name: fieldTitle, Value: "Título confidencial"},
		{Name: "ojs_url", Value: "https://journal.invalid/submission/42"},
		{Name: fieldDeadline, Value: testDeadlineValue},
		{Name: "reviewer_name", Value: testReviewerName},
	}
	message := Message{
		To: "Pessoa Real <real@journal.invalid>", From: "JSERD <sender@journal.invalid>", ReplyTo: []string{"reply@journal.invalid"},
		Subject: "Título confidencial — SUBMISSION-42 — Revisora Silva", Text: "Pessoa Real SUBMISSION-42 Título confidencial https://journal.invalid/submission/42 10/06/2026 Revisora Silva real@journal.invalid sender@journal.invalid reply@journal.invalid",
		HTML: fmt.Sprintf("<p>%s %s %s %s %s %s</p>", fields[0].Value, fields[1].Value, fields[2].Value, fields[3].Value, fields[4].Value, fields[5].Value), SensitiveFields: fields,
	}
	transport := &MockTransport{Output: &output}
	if err := transport.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	serialized := fmt.Sprintf("%#v\n%s", transport.Sent, output.String())
	for _, value := range []string{"Pessoa Real", "SUBMISSION-42", "Título confidencial", "https://journal.invalid/submission/42", testDeadlineValue, testReviewerName, "real@journal.invalid", "sender@journal.invalid", "reply@journal.invalid"} {
		if strings.Contains(serialized, value) {
			t.Fatalf("mock leaked %q: %s", value, serialized)
		}
	}
	if !strings.Contains(serialized, "example.test") {
		t.Fatalf("mock lacks safe domain: %s", serialized)
	}
}

func TestMockTransportRedactionIsDeterministicByField(t *testing.T) {
	message := Message{To: "real@journal.invalid", From: "sender@journal.invalid", Subject: sameValue, Text: sameValue, HTML: sameValue, SensitiveFields: []SensitiveField{{Name: fieldTitle, Value: sameValue}, {Name: "submission_id", Value: sameValue}}}
	first, second := &MockTransport{}, &MockTransport{}
	if err := first.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if err := second.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%#v", first.Sent) != fmt.Sprintf("%#v", second.Sent) {
		t.Fatalf("nondeterministic: first=%#v second=%#v", first.Sent, second.Sent)
	}
	if first.Sent[0].Subject == sameValue {
		t.Fatal("sensitive value was not redacted")
	}
}

func (m *captureMailer) Send(message *mailer.Message) error {
	m.message = message
	return m.err
}

func TestSMTPTransportEnvelope(t *testing.T) {
	backend := &captureMailer{}
	transport := SMTPTransport{Mailer: backend}
	message := Message{To: "Editora <editor@example.test>", From: "JSERD <noreply@example.test>", ReplyTo: []string{"editorial@example.test"}, Subject: "Assunto", Text: "Texto", HTML: "<p>HTML</p>", SensitiveFields: []SensitiveField{{Name: fieldTitle, Value: "Assunto"}}}
	if err := transport.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	got := backend.message
	if got == nil || got.From.Address != "noreply@example.test" || len(got.To) != 1 || got.To[0].Address != "editor@example.test" || got.Headers["Reply-To"] != "editorial@example.test" || got.Subject != message.Subject || got.Text != message.Text || got.HTML != message.HTML {
		t.Fatalf("SMTP envelope=%#v", got)
	}
	backend.err = errors.New("authentication failed: sentinel-password sentinel-token")
	err := transport.Send(context.Background(), message)
	if err == nil || strings.Contains(err.Error(), "sentinel-password") || strings.Contains(err.Error(), "sentinel-token") {
		t.Fatalf("unsafe SMTP error: %v", err)
	}
}
