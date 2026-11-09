package migrations

// Field, value, and collection names that several migrations reference for
// the same schema element. Centralised here so each name is spelled once
// (goconst).
const (
	fieldStatus              = "status"
	fieldCycle               = "cycle"
	fieldRun                 = "run"
	fieldEvent               = "event"
	fieldPayload             = "payload"
	fieldMuted               = "muted"
	fieldRuleCode            = "rule_code"
	fieldName                = "name"
	fieldActive              = "active"
	fieldOperator            = "operator"
	fieldOJSSubmissionID     = "ojs_submission_id"
	fieldStatusDetail        = "status_detail"
	fieldEditorNote          = "editor_note"
	fieldObservations        = "observations"
	fieldDecisionDetail      = "decision_detail"
	fieldEffectivePriority   = "effective_priority"
	fieldUpToDateDigest      = "up_to_date_digest"
	fieldDecisionDate        = "decision_date"
	fieldSanitizedError      = "sanitized_error"
	collectionEditorialItems = "editorial_items"

	statusValueFailed  = "failed"
	statusValueBlocked = "blocked"
	statusValueSent    = "sent"
	queueValueArchived = "archived"
	queueValueDaily    = "daily"
	queueValueUpToDate = "up_to_date"
	sourceModeFixtures = "fixtures"

	fieldRequest            = "request"
	fieldFindingDigest      = "finding_digest"
	fieldMutedDigest        = "muted_digest"
	fieldLocalState         = "local_state"
	collectionEditorialRuns = "editorial_runs"
	testMissingPath         = "/missing"
	fieldUpToDateAt         = "up_to_date_at"
	fieldUpToDateUntil      = "up_to_date_until"
)
