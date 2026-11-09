package email

import (
	"server/internal/collections"
	"server/internal/events"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// render picks the template for a notification's rule_code. WELCOME carries
// no event-specific payload requirements: a recipient's name is all it
// needs (ticket welcome-email/02).
func (w *Worker) render(rule string, recipient *core.Record, submissionID string, payload deliveryPayload) (Rendered, []string, error) {
	if rule == events.RuleWelcome {
		rendered, err := w.Renderer.RenderWelcome(WelcomeTemplateData{RecipientName: recipient.GetString("name")})
		return rendered, append([]string(nil), w.ReplyTo...), err
	}
	if events.IsReviewerRule(rule) {
		return w.renderReviewer(rule, recipient, submissionID, payload)
	}
	rendered, err := w.Renderer.RenderEditor(rule, EditorTemplateData{RecipientName: recipient.GetString("name"), SubmissionID: submissionID, Title: payload.Title, Deadline: payload.Deadline, OJSURL: payload.OJSURL, ReviewerName: payload.ReviewerName})
	return rendered, append([]string(nil), w.ReplyTo...), err
}

func (w *Worker) renderReviewer(rule string, recipient *core.Record, submissionID string, payload deliveryPayload) (Rendered, []string, error) {
	replyTo, err := w.reviewerReplyTo(payload.EditorTargetKeys)
	if err != nil {
		return Rendered{}, replyTo, err
	}
	rendered, err := w.Renderer.RenderReviewer(rule, ReviewerTemplateData{RecipientName: recipient.GetString("name"), SubmissionID: submissionID, Title: payload.Title, Deadline: payload.Deadline, OJSURL: payload.OJSURL})
	return rendered, replyTo, err
}

func (w *Worker) reviewerReplyTo(editorTargetKeys []string) ([]string, error) {
	contacts, err := w.editorContacts(editorTargetKeys)
	if err != nil {
		return nil, err
	}
	return ReviewerReplyTo(contacts, w.institutionalReplyTo())
}

// editorContacts are the stored contacts of the assigned editors a set of
// slugs (ADR-0006) names. A slug that names no stored editor adds nobody.
func (w *Worker) editorContacts(editorTargetKeys []string) ([]EditorContact, error) {
	contacts := []EditorContact{}
	for _, key := range editorTargetKeys {
		records, err := w.App.FindRecordsByFilter(collections.Recipients, "name_slug = {:key} && is_editor = true", "", 1, 0, dbx.Params{"key": key})
		if err != nil {
			return nil, err
		}
		if len(records) > 0 {
			contacts = append(contacts, EditorContact{Name: records[0].GetString("name"), Email: records[0].GetString("email")})
		}
	}
	return contacts, nil
}

// institutionalReplyTo is the address a reviewer e-mail falls back to when no
// assigned editor has a usable address.
func (w *Worker) institutionalReplyTo() string {
	if len(w.ReplyTo) == 0 {
		return ""
	}
	return w.ReplyTo[0]
}
