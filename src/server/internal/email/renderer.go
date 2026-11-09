package email

import (
	"fmt"
	"server/internal/events"
	"strings"
)

const (
	RN1TemplateVersion     = "rn1:v1"
	WelcomeTemplateVersion = "welcome:v1"
)

type (
	TemplateData         struct{ RecipientName, SubmissionID, Title, OJSURL, ReviewerName string }
	EditorTemplateData   struct{ RecipientName, SubmissionID, Title, Deadline, OJSURL, ReviewerName string }
	ReviewerTemplateData struct{ RecipientName, SubmissionID, Title, Deadline, OJSURL string }
	WelcomeTemplateData  struct{ RecipientName string }
	EditorContact        struct{ Name, Email string }
	Rendered             struct{ Subject, Text, HTML, Version, Hash string }
	Renderer             struct{}
)

func (Renderer) RenderWelcome(data WelcomeTemplateData) (Rendered, error) {
	if strings.TrimSpace(data.RecipientName) == "" {
		return Rendered{}, fmt.Errorf("welcome template data is incomplete")
	}
	return renderTemplate("welcome", WelcomeTemplateVersion, data)
}

func (Renderer) RenderRN1(data TemplateData) (Rendered, error) {
	return (Renderer{}).RenderEditor("RN1", EditorTemplateData{
		RecipientName: data.RecipientName,
		SubmissionID:  data.SubmissionID,
		Title:         data.Title,
		OJSURL:        data.OJSURL,
		ReviewerName:  data.ReviewerName,
	})
}

// editorRules are the notification rules that e-mail an editor.
var editorRules = map[string]bool{"RN1": true, "RN2": true, "RN3": true, "RN4": true, "RN5": true, "RN18": true}

func (Renderer) RenderEditor(rule string, data EditorTemplateData) (Rendered, error) {
	if !editorRules[rule] {
		return Rendered{}, fmt.Errorf("unsupported editor template %q", rule)
	}
	if !completeTemplateData(data.RecipientName, data.SubmissionID, data.Title, data.OJSURL) {
		return Rendered{}, fmt.Errorf("editor template data is incomplete")
	}
	if err := validateEditorFields(rule, data); err != nil {
		return Rendered{}, err
	}
	name := strings.ToLower(rule)
	return renderTemplate(name, name+":v1", data)
}

// editorRequiredField is one extra field an editor rule needs, beyond the
// fields every editor template needs.
type editorRequiredField struct {
	value   func(EditorTemplateData) string
	message string
}

var reviewerNameField = editorRequiredField{func(d EditorTemplateData) string { return d.ReviewerName }, "editor reviewer name is required"}

var deadlineField = editorRequiredField{func(d EditorTemplateData) string { return d.Deadline }, "editor deadline is required"}

// editorRequiredFields lists the extra fields each editor rule needs.
var editorRequiredFields = map[string][]editorRequiredField{
	"RN1": {reviewerNameField},
	"RN2": {reviewerNameField, deadlineField},
	"RN3": {reviewerNameField},
}

// validateEditorFields checks the fields that only some editor rules need.
func validateEditorFields(rule string, data EditorTemplateData) error {
	for _, field := range editorRequiredFields[rule] {
		if strings.TrimSpace(field.value(data)) == "" {
			return fmt.Errorf("%s", field.message)
		}
	}
	return nil
}

func (Renderer) RenderReviewer(rule string, data ReviewerTemplateData) (Rendered, error) {
	if !events.IsReviewerRule(rule) {
		return Rendered{}, fmt.Errorf("unsupported reviewer template %q", rule)
	}
	if !completeTemplateData(data.RecipientName, data.SubmissionID, data.Title, data.OJSURL) {
		return Rendered{}, fmt.Errorf("reviewer template data is incomplete")
	}
	if strings.TrimSpace(data.Deadline) == "" {
		return Rendered{}, fmt.Errorf("reviewer deadline is required")
	}
	name := strings.ToLower(rule)
	return renderTemplate(name, name+":v1", data)
}

func completeTemplateData(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}
