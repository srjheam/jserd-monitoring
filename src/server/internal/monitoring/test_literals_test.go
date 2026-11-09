package monitoring

// Fixture literals repeated across this package's tests. Centralised here
// so each is spelled once (goconst). statusFieldName, defaultQueue, and
// activityKindDelivery already exist in non-test files and are reused here
// rather than redeclared.
const (
	statusRejected     = "Rejected"
	statusWithdrawn    = "Withdrawn"
	finalAccepted      = "accepted"
	finalRejected      = "rejected"
	finalWithdrawn     = "withdrawn"
	fieldPayload       = "payload"
	sourceModeFixtures = "fixtures"
	fieldEvent         = "event"
	fieldRuleCode      = "rule_code"
	targetKindEditor   = "editor"
	targetKindReviewer = "reviewer"

	testDate20260105 = "2026-01-05"
	testDate20260101 = "2026-01-01"
	testDate20260301 = "2026-03-01"

	testSubmissionTitle     = "Fixture"
	testSubmissionTitleFull = "Fixture submission"
	statusQueued            = "Queued"
	statusPublished         = "Published"
	statusDeclined          = "Declined"
	statusCopyediting       = "Copyediting"
	testSubmissionIDUnknown = "unknown"

	testReviewerEmail    = "reviewer@example.test"
	findingOverdueReview = "overdue_review"

	testEditorNameAlex  = "Alex"
	testEditorNameBlake = "Blake"

	worklistKindWork = "work"
	testCodeSmell    = "code smell"
	testChiefCasey   = "Casey"

	rankRecent1  = "recent-rank-1"
	rankOldest1  = "oldest-rank-1"
	rankAncient2 = "ancient-rank-2"
	rankMutedOne = "muted-one"

	knownDateKey   = "known-date"
	unknownDateKey = "unknown-date"

	reviewerAndersonS     = "Anderson S."
	reviewerAndersonSSlug = "anderson-s"
	reviewerBlakeSlug     = "blake"
)
