package ojs

import (
	"cmp"
	"slices"
	"strconv"
)

// reviewAssignmentStatusResponseOverdue is OJS pkp-lib's
// REVIEW_ASSIGNMENT_STATUS_RESPONSE_OVERDUE (4), distinct from
// REVIEW_ASSIGNMENT_STATUS_DECLINED (1) — unlike the CSV export's Declined
// column, which is always "Yes" regardless of true state.
const (
	reviewAssignmentStatusResponseOverdue = 4
)

type reviewAssignmentWindow struct {
	StatusID int    `json:"statusId"`
	Round    int    `json:"round"`
	Due      string `json:"due"`
}

// reviewAssignmentResolvedStatuses are OJS pkp-lib reviewAssignment statusId
// values that mean the assignment is no longer an open review-due obligation:
// submitted,
// completed, thanked, and cancelled. Any other statusId, known or not, is
// treated as still pending — the safe default for a reminder/overdue check.
var reviewAssignmentResolvedStatuses = map[int]bool{7: true, 8: true, 9: true, 10: true}

// reviewRoundPayload is one entry of the detail endpoint's "reviewRounds"
// array: one row per review round the submission has had, in stage 3
// (Review). "round" is 1-based and increases with every new round OJS
// opens; "statusId" is OJS pkp-lib's own ReviewRound::REVIEW_ROUND_STATUS_*
// value, recomputed fresh on every read, never cached.
type reviewRoundPayload struct {
	Round    int `json:"round"`
	StatusID int `json:"statusId"`
}

// OJS pkp-lib ReviewRound::REVIEW_ROUND_STATUS_* values this pipeline reads.
// REVISIONS_REQUESTED and RESUBMIT_FOR_REVIEW are the two decisions that ask
// an author for a revision; REVISIONS_SUBMITTED and RESUBMIT_SUBMITTED are
// OJS's own computed confirmation that the revision landed.
const (
	reviewRoundStatusRevisionsRequested = 1
	reviewRoundStatusResubmitForReview  = 2
	reviewRoundStatusRevisionsSubmitted = 11
	reviewRoundStatusResubmitSubmitted  = 15
)

// LatestRoundRevisionPending reports whether the submission's most recent
// review round still shows OJS's own state for "waiting on the author's
// revision" (statusId 1 or 2), as opposed to "the revision already landed"
// (11 or 15) or anything else (a later round moved on, the round was
// accepted or declined). observed is false when the submission has no
// review rounds at all, for RN5 and RN11.
func (d SubmissionDetail) LatestRoundRevisionPending() (pending bool, observed bool) {
	return latestRoundRevisionPending(d.ReviewRounds)
}

func latestRoundRevisionPending(rounds []reviewRoundPayload) (bool, bool) {
	status, ok := latestRoundStatus(rounds)
	if !ok {
		return false, false
	}
	return status == reviewRoundStatusRevisionsRequested || status == reviewRoundStatusResubmitForReview, true
}

// latestRoundStatus is OJS's own status for the most recent review round. It
// reports false when the submission has had no review round at all.
func latestRoundStatus(rounds []reviewRoundPayload) (int, bool) {
	if len(rounds) == 0 {
		return 0, false
	}
	latest := slices.MaxFunc(rounds, func(a, b reviewRoundPayload) int {
		return cmp.Compare(a.Round, b.Round)
	})
	return latest.StatusID, true
}

// OverdueReviewRounds returns the set of review rounds where at least one
// reviewAssignment has missed its response due date, per OJS's own
// statusId, for RN2.
func (d SubmissionDetail) OverdueReviewRounds() map[int]bool {
	return overdueReviewRounds(d.ReviewAssignments)
}

func overdueReviewRounds(assignments []reviewAssignmentWindow) map[int]bool {
	rounds := map[int]bool{}
	for _, assignment := range assignments {
		if assignment.StatusID == reviewAssignmentStatusResponseOverdue {
			rounds[assignment.Round] = true
		}
	}
	return rounds
}

// ResolvedReviewWindows reports, per (round, review due date), whether OJS's
// live state shows the window as resolved: every reviewAssignment sharing
// that round and due date has a resolved statusId. A window this map has no
// entry for at all had no live reviewAssignment matching it — the caller
// must treat a missing entry as resolved too, since that means the CSV
// export's row is stale (the reviewer was replaced, or the round moved on).
// For RN7/RN8.
func (d SubmissionDetail) ResolvedReviewWindows() map[string]bool {
	return resolvedReviewWindows(d.ReviewAssignments)
}

func resolvedReviewWindows(assignments []reviewAssignmentWindow) map[string]bool {
	pending, resolved := splitReviewWindowsByStatus(assignments)
	// Every key either map saw gets an entry: a window with no resolved
	// assignment at all (pending-only, no competing resolved row) must read
	// as still open, not as "no live match" — before this fix, such a key
	// never entered windows, so the caller (reviewWindowSuperseded) treated
	// a genuinely open, unresolved review the same as a reassigned one with
	// no live counterpart at all.
	windows := map[string]bool{}
	for key := range resolved {
		windows[key] = !pending[key]
	}
	addOpenPendingOnlyWindows(windows, pending)
	return windows
}

func splitReviewWindowsByStatus(assignments []reviewAssignmentWindow) (pending, resolved map[string]bool) {
	pending, resolved = map[string]bool{}, map[string]bool{}
	for _, assignment := range assignments {
		key := strconv.Itoa(assignment.Round) + "|" + assignment.Due
		if reviewAssignmentResolvedStatuses[assignment.StatusID] {
			resolved[key] = true
		} else {
			pending[key] = true
		}
	}
	return pending, resolved
}

func addOpenPendingOnlyWindows(windows, pending map[string]bool) {
	for key := range pending {
		if _, ok := windows[key]; !ok {
			windows[key] = false
		}
	}
}
