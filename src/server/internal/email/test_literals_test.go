package email

// Fixture literals repeated across renderer_test.go and transport_test.go.
// Centralised here so each is spelled once (goconst).
const (
	ruleRN1 = "RN1"
	ruleRN2 = "RN2"
	ruleRN3 = "RN3"
	ruleRN4 = "RN4"
	ruleRN5 = "RN5"
	ruleRN6 = "RN6"
	ruleRN7 = "RN7"
	ruleRN8 = "RN8"

	testEditorName      = "Editora"
	testSubmissionTitle = "Artigo"
	testReviewerName    = "Revisora Silva"
	wantPleaseText      = "Please"
	testSubmissionURL   = "https://ojs.example.test/42"
	invalidEmailValue   = "bad"
	fieldDeadline       = "deadline"
	testDeadlineValue   = "10/06/2026"
	fieldTitle          = "title"
	sameValue           = "same"
)
