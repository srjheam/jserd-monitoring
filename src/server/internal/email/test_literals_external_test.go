package email_test

// Fixture literals repeated across smtp_integration_test.go and
// worker_test.go. Centralised here so each is spelled once (goconst).
const (
	testSubmissionTitle = "Artigo"
	testSubmissionURL   = "https://ojs.example.test/42"
	fieldTitle          = "title"
	testFromAddress     = "noreply@jserd.test"
	statusSent          = "sent"
	statusFailed        = "failed"
	ownerIDWorker       = "worker"
	fieldOJSURL         = "ojs_url"
)
