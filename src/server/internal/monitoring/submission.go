package monitoring

import (
	"encoding/json"
	"fmt"
	"server/internal/collections"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// computedProvenance marks a field the server derives rather than stores, the
// same mark materialize.go's acceptedSubmissions uses for the same fields.
const computedProvenance = "computed"

// knownProvenance marks a field materialize.go's acceptedSubmissions read a
// real value for, as opposed to unavailableProvenance.
const knownProvenance = "known"

// unavailableProvenance marks a field materialize.go's acceptedSubmissions
// read no value for.
const unavailableProvenance = "unavailable"

// FindSubmission loads one materialized submission by its OJS identifier.
func FindSubmission(app core.App, ojsSubmissionID string) (*Submission, bool, error) {
	rows, err := app.FindRecordsByFilter(collections.OJSSubmissions, "ojs_submission_id = {:id}", "", 1, 0, dbx.Params{"id": ojsSubmissionID})
	if err != nil {
		return nil, false, err
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	submission, err := submissionDetail(app, rows[0], ojsSubmissionID)
	if err != nil {
		return nil, false, err
	}
	return submission, true, nil
}

// submissionDetail is the single-submission view: the stored row, the curation
// on it, and its current findings.
func submissionDetail(app core.App, row *core.Record, ojsSubmissionID string) (*Submission, error) {
	tracking, err := submissionTracking(app, ojsSubmissionID)
	if err != nil {
		return nil, err
	}
	submission, err := submissionFromStored(storedSubmission{
		ojsSubmissionID: row.GetString("ojs_submission_id"),
		title:           row.GetString("title"),
		status:          row.GetString(statusFieldName),
		finalState:      row.GetString("effective_final_state"),
		queue:           row.GetString("effective_queue"),
		attention:       row.GetString("effective_attention"),
		findingDigest:   row.GetString("finding_digest"),
		evidenceJSON:    row.GetString("evidence"),
	}, tracking)
	if err != nil {
		return nil, err
	}
	findings, err := latestFindingsFor(app, []string{submission.OJSSubmissionID})
	if err != nil {
		return nil, err
	}
	submission.Findings = findingsView(findings[submission.OJSSubmissionID])
	submission.Provenance["findings"] = computedProvenance
	return &submission, nil
}

// submissionTracking is the curation on one submission, or nil when a chief
// editor curated nothing on it yet.
func submissionTracking(app core.App, ojsSubmissionID string) (*LocalTracking, error) {
	records, err := app.FindRecordsByFilter(collections.SubmissionTracking, "ojs_submission_id = {:id}", "", 1, 0, dbx.Params{"id": ojsSubmissionID})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return trackingView(records[0]), nil
}

// orEmpty returns an empty list in place of a missing one, so the JSON view
// always carries a list instead of a null.
func orEmpty[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// storedSubmission is one materialized row, in the shape submissionFromStored
// reads it. Eight of its nine values used to be positional strings, so a
// transposition at either call site compiled: effectiveQueue and
// effectiveAttention are adjacent, and swapping them put a queue name where the
// client expects a finding kind. Named fields cannot be transposed.
type storedSubmission struct {
	ojsSubmissionID string
	title           string
	status          string
	finalState      string
	queue           string
	attention       string
	findingDigest   string
	evidenceJSON    string
}

func submissionFromStored(stored storedSubmission, tracking *LocalTracking) (Submission, error) {
	var evidence Evidence
	if err := json.Unmarshal([]byte(stored.evidenceJSON), &evidence); err != nil {
		return Submission{}, fmt.Errorf("decode evidence of %s: %w", stored.ojsSubmissionID, err)
	}
	evidence.Editors = orEmpty(evidence.Editors)
	evidence.Decisions = orEmpty(evidence.Decisions)
	evidence.Reviews = orEmpty(evidence.Reviews)
	return Submission{
		OJSSubmissionID:     stored.ojsSubmissionID,
		Title:               stored.title,
		Status:              stored.status,
		URL:                 evidence.URL,
		DateSubmitted:       evidence.DateSubmitted,
		LastModified:        evidence.LastModified,
		DateLastActivity:    evidence.DateLastActivity,
		Editors:             evidence.Editors,
		Decisions:           evidence.Decisions,
		CopyeditorAssigned:  evidence.CopyeditorAssigned,
		Reviews:             evidence.Reviews,
		Findings:            []Finding{},
		Tracking:            tracking,
		EffectiveFinalState: stored.finalState,
		EffectiveQueue:      stored.queue,
		EffectiveAttention:  stored.attention,
		FindingDigest:       stored.findingDigest,
		Provenance:          evidence.Provenance,
	}, nil
}
