package notifications_test

// Fixture literals repeated across e2e_test.go and welcome_e2e_test.go.
// Centralised here so each is spelled once (goconst).
const (
	testEditorEmail = "editor@journal.test"
	testFromAddress = "noreply@jserd.test"

	ruleRN1 = "RN1"
	ruleRN2 = "RN2"
	ruleRN3 = "RN3"

	statusSent = "sent"

	payloadSubmissionID = "submissionId"
	payloadTargetKind   = "targetKind"
	targetKindReviewer  = "reviewer"
	targetKindEditor    = "editor"
	payloadTargetKey    = "targetKey"
	testReviewerKey     = "reviewer-1"
	payloadEditorKeys   = "editorTargetKeys"
	payloadTitle        = "title"
	testSubmissionTitle = "Artigo"
	payloadOJSURL       = "ojs_url"
	testSubmissionURL   = "https://ojs.example.test/42"
	payloadDeadline     = "deadline"
	testDeadlineValue   = "10/06/2026"
	testReviewerName    = "Revisora Silva"
	testEditorKey       = "editor-1"
	sourceModeFixtures  = "fixtures"
)
