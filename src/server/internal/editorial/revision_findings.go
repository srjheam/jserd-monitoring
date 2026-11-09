package editorial

import (
	"server/internal/normalizer"
	"strings"
	"time"
)

// This file holds the findings and helpers around a stale revision
// decision: whether the fallback wait after that decision has passed with
// no later review round opened, and which side of it — the author's
// missing revision, or the editor's missing decision on a revision that
// already arrived — is still open.

// revisionFindings reports which side of a stale revision decision is still
// missing, once the fallback wait after that decision has passed with no
// later review round opened. RevisionPending true means the author never
// sent the revision (missing_revision_indication). RevisionPending false
// means the revision already arrived and the editor never decided on it
// since (revision_pending_decision). The two are mutually exclusive: exactly one can fire for a given
// stale decision, matching whichever side OJS's own state still shows open.
func revisionFindings(sub normalizer.Submission, reviews []normalizer.ReviewAssignment, now time.Time, fallbackDays int) []Finding {
	decision, deadline, ok := pastRevisionDeadline(sub, reviews, now, fallbackDays)
	if !ok {
		return nil
	}
	return revisionDeadlineFinding(decision, deadline, sub.RevisionPending)
}

func pastRevisionDeadline(sub normalizer.Submission, reviews []normalizer.ReviewAssignment, now time.Time, fallbackDays int) (revisionDecision, time.Time, bool) {
	decision, decidedAt, ok := latestRevisionDecision(sub)
	if !ok || hasLaterReviewRound(reviews, decidedAt) {
		return revisionDecision{}, time.Time{}, false
	}
	deadline := decidedAt.AddDate(0, 0, fallbackDays)
	return decision, deadline, now.After(deadline)
}

// revisionDeadlineFinding reads pending — nil means OJS's revision-pending
// state was never observed for this submission, so neither finding can fire.
//
// revision_pending_decision carries no Deadline. Its wait starts the moment
// the revision lands, a date OJS reports through no field this codebase can
// read; deadline here is decidedAt plus the fallback wait, the date the
// author's own reply was due, not a date that means anything once the
// revision already arrived.
// "the deadline" reads as a past-due date for something that already
// happened, which it is not). reviews_complete_no_decision, the sibling
// finding for the same "editor owes a decision" shape, already carries no
// deadline for the identical reason.
func revisionDeadlineFinding(decision revisionDecision, deadline time.Time, pending *bool) []Finding {
	if pending == nil {
		return nil
	}
	if !*pending {
		return []Finding{{Kind: FindingRevisionPendingDecision, Responsible: responsibleEditor, Contact: decision.Email}}
	}
	return []Finding{{Kind: FindingMissingRevisionIndication, Responsible: "author", Deadline: deadline.Format("2006-01-02"), Contact: decision.Email}}
}

type revisionDecision struct{ Email string }

func latestRevisionDecision(sub normalizer.Submission) (revisionDecision, time.Time, bool) {
	var latest time.Time
	var result revisionDecision
	for _, editor := range sub.Editors {
		result, latest = laterEditorDecision(editor, result, latest)
	}
	return result, latest, !latest.IsZero() && result.Email != ""
}

// laterEditorDecision keeps whichever of one editor's decisions is newer than
// the newest decision seen so far. A decision that asks for no revision still
// moves the date forward, and names nobody, because a later decision replaces
// an earlier revision request.
func laterEditorDecision(editor normalizer.Editor, result revisionDecision, latest time.Time) (revisionDecision, time.Time) {
	for _, decision := range editor.Decisions {
		when, ok := normalizer.ParseDate(decision.DateDecided)
		if !ok || !when.After(latest) {
			continue
		}
		latest = when
		result = revisionDecisionOf(editor.Email, decision.Decision)
	}
	return result, latest
}

// revisionDecisionOf names the editor that asked for a revision. A decision
// that asks for no revision names nobody.
func revisionDecisionOf(email, decision string) revisionDecision {
	name := strings.ToLower(strings.TrimSpace(decision))
	if strings.Contains(name, "revision") || strings.Contains(name, "resubmit") {
		return revisionDecision{Email: email}
	}
	return revisionDecision{}
}

func hasLaterReviewRound(reviews []normalizer.ReviewAssignment, decidedAt time.Time) bool {
	for _, review := range reviews {
		if review.Round <= 1 {
			continue
		}
		// Every date OJS stamps on one review assignment.
		dates := []string{review.DateAssigned, review.DateNotified, review.DateConfirmed, review.DateCompleted}
		if hasDateAfter(dates, decidedAt) {
			return true
		}
	}
	return false
}

// hasDateAfter reports whether any readable date falls after a given moment.
func hasDateAfter(values []string, after time.Time) bool {
	for _, value := range values {
		if when, ok := normalizer.ParseDate(value); ok && when.After(after) {
			return true
		}
	}
	return false
}
