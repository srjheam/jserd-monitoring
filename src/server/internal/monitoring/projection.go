package monitoring

import (
	"server/internal/editorial"
	"server/internal/normalizer"
	"strings"
)

// findingsView drops the private contact of every finding so the projection
// only carries what the client is allowed to render.
func findingsView(findings []editorial.Finding) []Finding {
	items := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		items = append(items, Finding{Kind: string(finding.Kind), Responsible: finding.Responsible, Deadline: finding.Deadline})
	}
	return items
}

// curatedFields are the submission fields a chief editor owns, rather than OJS.
var curatedFields = []string{"statusDetail", "editorNote", "lastVerifiedAt", "verifiedBy", "observations", "decisionDetail", "conference", "created", "updated"}

// markCuratedProvenance records whether the curated fields carry local values
// or nothing at all.
func markCuratedProvenance(provenance Provenance, tracking *LocalTracking) {
	source := unavailableProvenance
	if tracking != nil {
		source = "local"
		provenance["tracking"] = "local"
	}
	for _, field := range curatedFields {
		provenance[field] = source
	}
}

// addEditorsAndDecisions fills the assigned editors and the decisions they took.
func addEditorsAndDecisions(item *Submission, editors []normalizer.Editor) {
	for _, editor := range editors {
		addEditorName(item, editor)
		for _, decision := range editor.Decisions {
			item.Decisions = append(item.Decisions, Decision{Decision: decision.Decision, Date: decision.DateDecided})
			item.Provenance["decisions"] = knownProvenance
		}
	}
}

// addEditorName adds the display name of one assigned editor. An editor with no
// given name and no family name has no display name, so nothing is added.
func addEditorName(item *Submission, editor normalizer.Editor) {
	name := strings.TrimSpace(editor.GivenName + " " + editor.FamilyName)
	if name == "" {
		return
	}
	item.Editors = append(item.Editors, name)
	item.Provenance["editors"] = knownProvenance
}

// addReviews fills the review assignments that name this submission.
func addReviews(item *Submission, ojsSubmissionID string, reviews []normalizer.ReviewAssignment) {
	for _, review := range reviews {
		if review.SubmissionID != ojsSubmissionID {
			continue
		}
		item.Reviews = append(item.Reviews, safeReview(review))
		item.Provenance["reviews"] = knownProvenance
	}
}

func availability(value string) string {
	if value == "" {
		return unavailableProvenance
	}
	return knownProvenance
}

// safeReview never reads review.Declined: OJS's live "Reviews" export marks
// that field true on almost every row regardless of the reviewer's real
// status (see isOpenForOverdueCheck in internal/editorial/review_status.go),
// so it cannot tell a declined review from an accepted or completed one.
func safeReview(review normalizer.ReviewAssignment) Review {
	status := "awaiting_review"
	if review.DateCompleted != "" {
		status = "completed"
	} else if review.ResponseOverdueDays > 0 || review.ReviewOverdueDays > 0 {
		status = "overdue"
	}
	return Review{Round: review.Round, Status: status, ResponseDeadline: review.ResponseDueDate, ReviewDeadline: review.ReviewDueDate}
}
