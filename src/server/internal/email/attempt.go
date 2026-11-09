package email

import (
	"fmt"
	"net/mail"
	"server/internal/collections"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// bareSenderAddress strips the display name off the sender address, when the
// configured value carries one.
func bareSenderAddress(from string) string {
	parsed, err := mail.ParseAddress(from)
	if err != nil {
		return from
	}
	return parsed.Address
}

// attemptOutcome is how one delivery ended: the status the column stores, the
// class the activity timeline translates into words, and the sanitized error.
// All three are strings. They travel as named fields, and mostly as whole named
// values below, so a call site cannot transpose two of them and still compile.
type attemptOutcome struct {
	status      string
	resultClass string
	message     string
}

// The statuses the status column holds. A blocked delivery was refused before
// a send was tried; a failed one was tried and did not arrive.
const (
	attemptStatusSent    = "sent"
	attemptStatusBlocked = "blocked"
	attemptStatusFailed  = "failed"

	// renderFailureClass covers both ways an e-mail can fail before a send:
	// a payload that cannot be read, and a template that refuses the rule.
	renderFailureClass = "render_failure"
)

// The outcomes a delivery ends in. Every attempt row carries one of these, or
// the render failure below.
var (
	attemptSent             = attemptOutcome{status: attemptStatusSent, resultClass: "success"}
	attemptRecipientBlocked = attemptOutcome{status: attemptStatusBlocked, resultClass: "recipient_blocked", message: "invalid recipient address"}
	attemptBadPayload       = attemptOutcome{status: attemptStatusFailed, resultClass: renderFailureClass, message: "invalid event payload"}
	attemptSMTPFailure      = attemptOutcome{status: attemptStatusFailed, resultClass: "smtp_permanent_failure", message: "SMTP send failed"}
)

// renderFailure is the one outcome whose message is not fixed: it carries what
// the renderer said went wrong.
func renderFailure(err error) attemptOutcome {
	return attemptOutcome{status: attemptStatusFailed, resultClass: renderFailureClass, message: err.Error()}
}

// sentEmail is what an attempt row records about the message itself. A delivery
// that never got as far as rendering carries blankRendered and an empty
// subject, which is why every field has a usable zero value.
type sentEmail struct {
	rendered  Rendered
	toAddress string
	subject   string
	replyTo   []string
}

// saveAttempt records one delivery attempt, whatever it ended in. Every value
// it stores comes from one of four places: the request, the records behind it,
// the message, and the outcome.
func (w *Worker) saveAttempt(request *core.Record, records deliveryRecords, email sentEmail, outcome attemptOutcome) error {
	collection, err := w.App.FindCollectionByNameOrId(collections.EmailDeliveryAttempts)
	if err != nil {
		return err
	}
	attempt := core.NewRecord(collection)
	notification := records.notification
	for key, value := range map[string]any{
		"request": request.Id, "event": notification.GetString("event"), "rule_match": notification.GetString("rule_match"),
		"target": records.target.Id, "recipient": records.recipient.Id, "rule_code": notification.GetString("rule_code"),
		"target_kind": records.target.GetString("target_kind"), "target_key": records.target.GetString("target_key"),
		"recipient_role": records.recipientRole,
		"to_address":     email.toAddress, "from_address": bareSenderAddress(w.From), "reply_to_addresses": strings.Join(email.replyTo, ","),
		"subject": email.subject, "template_version": email.rendered.Version, "template_hash": email.rendered.Hash,
		"status": outcome.status, "result_class": outcome.resultClass,
		"sanitized_error": outcome.message, "raw_error": outcome.message,
	} {
		attempt.Set(key, value)
	}
	if err := w.App.Save(attempt); err != nil {
		return fmt.Errorf("save delivery attempt: %w", err)
	}
	return nil
}
