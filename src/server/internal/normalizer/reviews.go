package normalizer

import (
	"io"
	"strconv"
)

func NormalizeReviews(r io.Reader, artifact ArtifactRef) ([]ReviewAssignment, error) {
	table, err := readCSV(r, artifact, []string{"Stage", "Round", submissionIDColumn, "Reviewer", "Date Assigned"})
	if err != nil {
		return nil, err
	}

	reviews := make([]ReviewAssignment, 0, len(table.rows))
	for _, row := range table.rows {
		if review, ok := normalizeReview(table, row); ok {
			reviews = append(reviews, review)
		}
	}
	return reviews, nil
}

// reviewProvenanceColumns maps each review field to the report column it comes
// from.
var reviewProvenanceColumns = map[string]string{
	"stage":               "Stage",
	"round":               "Round",
	"submissionId":        submissionIDColumn,
	"reviewerEmail":       "Email",
	"dateAssigned":        "Date Assigned",
	"responseDueDate":     "Response Due Date",
	"responseOverdueDays": "Response Overdue Days",
	"reviewDueDate":       "Review Due Date",
	"reviewOverdueDays":   "Review Overdue Days",
	"declined":            "Declined",
	"unconsidered":        "Unconsidered",
}

// normalizeReview reads one row of the Reviews report. A row that names no
// submission id is not a review assignment.
// Review content (Recommendation, Comments On Submission) and reviewer
// profile columns (ORCID iD, Country, Affiliation) are not copied: no rule
// reads them, and the snapshot must not keep personal data it does not use.
func normalizeReview(table csvTable, row []string) (ReviewAssignment, bool) {
	submissionID := table.value(row, submissionIDColumn)
	if submissionID == "" {
		return ReviewAssignment{}, false
	}
	review := ReviewAssignment{
		Stage:               table.value(row, "Stage"),
		Round:               parseInt(table.value(row, "Round")),
		SubmissionTitle:     table.value(row, "Submission Title"),
		SubmissionID:        submissionID,
		ReviewerUsername:    table.value(row, "Reviewer"),
		ReviewerGivenName:   table.value(row, "Given Name"),
		ReviewerFamilyName:  table.value(row, "Family Name"),
		ReviewerEmail:       table.value(row, "Email"),
		DateAssigned:        table.value(row, "Date Assigned"),
		DateNotified:        table.value(row, "Date Notified"),
		DateConfirmed:       table.value(row, "Date Confirmed"),
		DateCompleted:       table.value(row, "Date Completed"),
		DateAcknowledged:    table.value(row, "Date Acknowledged"),
		DateReminded:        table.value(row, "Date Reminded"),
		ResponseDueDate:     table.value(row, "Response Due Date"),
		ResponseOverdueDays: parseInt(table.value(row, "Response Overdue Days")),
		ReviewDueDate:       table.value(row, "Review Due Date"),
		ReviewOverdueDays:   parseInt(table.value(row, "Review Overdue Days")),
		Declined:            parseYes(table.value(row, "Declined")),
		Unconsidered:        parseYes(table.value(row, "Unconsidered")),
		Provenance:          map[string]FieldProvenance{},
	}
	review.Key = review.SubmissionID + "|" + review.Stage + "|" + strconv.Itoa(review.Round) + "|" + review.ReviewerEmail + "|" + review.DateAssigned
	addProvenance(table, row, review.Provenance, reviewProvenanceColumns)
	return review, true
}
