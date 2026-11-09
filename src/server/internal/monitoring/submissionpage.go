package monitoring

import (
	"server/internal/editorial"

	"github.com/pocketbase/pocketbase/core"
)

// submissionIDs lists the OJS identifiers of the rows on this page.
func submissionIDs(rows []pageRow) []string {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.OjsSubmissionID
	}
	return ids
}

// submissionViews decodes the rows of one page, together with the curation and
// the findings of each submission.
func submissionViews(app core.App, rows []pageRow) ([]Submission, error) {
	ids := submissionIDs(rows)
	tracking, err := loadTrackingFor(app, ids)
	if err != nil {
		return nil, err
	}
	findings, err := latestFindingsFor(app, ids)
	if err != nil {
		return nil, err
	}
	return decodedRows(rows, tracking, findings)
}

// decodedRows builds the list view of each row. The list drops provenance,
// which only the single-submission route reports.
func decodedRows(rows []pageRow, tracking map[string]*LocalTracking, findings map[string][]editorial.Finding) ([]Submission, error) {
	items := make([]Submission, 0, len(rows))
	for _, row := range rows {
		submission, err := submissionFromStored(storedSubmission{
			ojsSubmissionID: row.OjsSubmissionID,
			title:           row.Title,
			status:          row.Status,
			finalState:      row.EffectiveFinalState,
			queue:           row.EffectiveQueue,
			attention:       row.EffectiveAttention,
			findingDigest:   row.FindingDigest,
			evidenceJSON:    row.Evidence,
		}, tracking[row.OjsSubmissionID])
		if err != nil {
			return nil, err
		}
		submission.Findings = findingsView(findings[submission.OJSSubmissionID])
		submission.Provenance = nil
		items = append(items, submission)
	}
	return items, nil
}

// latestFindingsFor resolves the findings of the most recent completed
// editorial run, scoped to the given submission ids, so pages always reflect
// the current editorial state without scanning every editorial_items row.
func latestFindingsFor(app core.App, ids []string) (map[string][]editorial.Finding, error) {
	return editorial.LatestFindingsFor(app, ids)
}
