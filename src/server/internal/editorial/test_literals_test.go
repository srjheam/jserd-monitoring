package editorial

// Fixture literals repeated across this package's tests. Centralised here
// so each is spelled once (goconst).
const (
	testDate20260101 = "2026-01-01"
	testDate20260201 = "2026-02-01"
	testDate20260601 = "2026-06-01"
	testDate20260610 = "2026-06-10"
	testDate20260620 = "2026-06-20"
	testDate20260629 = "2026-06-29"
	testDate20260630 = "2026-06-30"
	testDate20260710 = "2026-07-10"
	testDate20260901 = "2026-09-01"
	testDate20260925 = "2026-09-25"

	testEditorGivenName          = "Ana"
	testSubmissionTitle          = "Artigo"
	testDecisionAcceptSubmission = "Accept Submission"
	ruleRN4                      = "RN4"
	statusReview                 = "Review"
	statusRevisionsRequired      = "Revisions Required"

	testEditorEmail  = "casey@example.test"
	testOJSEmail     = "jserd@example.test"
	testOJSEmailTypo = "jsderd@example.test"
	testChiefEmail   = "editor@example.test"

	statusBlocked       = "blocked"
	testGroupIDChief    = "chief"
	fieldCycle          = "cycle"
	targetKindEditor    = "editor"
	fieldEvent          = "event"
	statusFailed        = "failed"
	fieldFinishedAt     = "finished_at"
	sourceModeFixtures  = "fixtures"
	testSubmissionURL   = "https://ojs.example.test/42"
	fieldManifest       = "manifest"
	testOldSubmissionID = "old"
	fieldPayload        = "payload"
	targetKindReviewer  = "reviewer"
	fieldRuleCode       = "rule_code"
	fieldSnapshot       = "snapshot"
	fieldSourceMode     = "source_mode"
	fieldStartedAt      = "started_at"
	fieldStatus         = "status"
	statusSucceeded     = "succeeded"
)
