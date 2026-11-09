package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/mail"
	"server/internal/collections"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// deliveryRecords are the rows one delivery request points at.
type deliveryRecords struct {
	target, recipient, notification, event *core.Record
	recipientRole                          string
}

// deliveryRecords follows one request back to the target, the recipient, the
// notification and the event behind it.
func (w *Worker) deliveryRecords(request *core.Record) (deliveryRecords, error) {
	var records deliveryRecords
	target, err := w.App.FindRecordById(collections.NotificationTargets, request.GetString("target"))
	if err != nil {
		return records, err
	}
	records.target = target
	records.recipientRole = target.GetString("recipient_role")
	records.recipient, err = w.App.FindRecordById(collections.Recipients, target.GetString("recipient"))
	if err != nil {
		return records, err
	}
	records.notification, err = w.App.FindRecordById(collections.Notifications, target.GetString("notification"))
	if err != nil {
		return records, err
	}
	records.event, err = w.App.FindRecordById(collections.Events, records.notification.GetString("event"))
	return records, err
}

type deliveryPayload struct {
	Title            string   `json:"title"`
	OJSURL           string   `json:"ojs_url"`
	Deadline         string   `json:"deadline"`
	SubmissionID     string   `json:"submissionId"`
	ReviewerName     string   `json:"reviewerName"`
	EditorTargetKeys []string `json:"editorTargetKeys"`
}

func (w *Worker) deliver(ctx context.Context, request *core.Record, records deliveryRecords) error {
	recipientAddress := records.recipient.GetString("email")
	if _, err := mail.ParseAddress(recipientAddress); err != nil {
		return w.saveAttempt(request, records, sentEmail{rendered: blankRendered(), toAddress: recipientAddress}, attemptRecipientBlocked)
	}
	var payload deliveryPayload
	if err := json.Unmarshal([]byte(records.event.GetString("payload")), &payload); err != nil {
		return w.saveAttempt(request, records, sentEmail{rendered: blankRendered(), toAddress: recipientAddress}, attemptBadPayload)
	}
	return w.renderAndSend(ctx, request, records, recipientAddress, payload)
}

// blankRendered stands in for the template of an e-mail that was never
// rendered, so a blocked or failed attempt still records a template version.
// The placeholder hash reads its width off the hash function itself, rather
// than spelling 64, so it cannot drift from the real digests beside it in the
// column.
func blankRendered() Rendered {
	return Rendered{Version: RN1TemplateVersion, Hash: strings.Repeat("0", hex.EncodedLen(sha256.Size))}
}

// renderAndSend renders the notification e-mail and sends it.
func (w *Worker) renderAndSend(ctx context.Context, request *core.Record, records deliveryRecords, recipientAddress string, payload deliveryPayload) error {
	rule := records.notification.GetString("rule_code")
	submissionID := submissionIDOf(payload, records.event)
	rendered, replyTo, err := w.render(rule, records.recipient, submissionID, payload)
	if err != nil {
		return w.saveAttempt(request, records, sentEmail{rendered: blankRendered(), toAddress: recipientAddress, replyTo: replyTo}, renderFailure(err))
	}
	message := Message{
		To: recipientAddress, From: w.From, ReplyTo: replyTo, Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML,
		SensitiveFields: []SensitiveField{
			{Name: "recipient_name", Value: records.recipient.GetString("name")},
			{Name: "submission_id", Value: submissionID},
			{Name: "title", Value: payload.Title},
			{Name: "ojs_url", Value: payload.OJSURL},
			{Name: "deadline", Value: payload.Deadline},
			{Name: "reviewer_name", Value: payload.ReviewerName},
		},
	}
	if w.BeforeSend != nil {
		w.BeforeSend()
	}
	if err := w.Transport.Send(ctx, message); err != nil {
		return w.saveAttempt(request, records, sentEmail{rendered: rendered, toAddress: recipientAddress, subject: rendered.Subject, replyTo: replyTo}, attemptSMTPFailure)
	}
	return w.saveAttempt(request, records, sentEmail{rendered: rendered, toAddress: recipientAddress, subject: rendered.Subject, replyTo: replyTo}, attemptSent)
}

// submissionIDOf is the submission a notification e-mail is about. An event
// payload that names none falls back to the event's own entity key.
func submissionIDOf(payload deliveryPayload, event *core.Record) string {
	if payload.SubmissionID != "" {
		return payload.SubmissionID
	}
	return event.GetString("entity_key")
}
