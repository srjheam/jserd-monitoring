package ojs

// reviewAssignmentInactiveStatuses are the statusId values of an assignment
// that no longer staffs its round: declined (1), response overdue (4), and
// cancelled (10). The detail query already omits declined assignments, so 1
// is here only as a guard.
var reviewAssignmentInactiveStatuses = map[int]bool{1: true, 4: true, 10: true}

// ActiveReviewers counts the latest round's live assignments that still
// staff it, for RN9 and RN19. The Reviews export fills DateConfirmed on a
// decline too, so only this live list can tell a decline apart.
func (d SubmissionDetail) ActiveReviewers() int {
	round := latestReviewRound(d)
	count := 0
	for _, assignment := range d.ReviewAssignments {
		if assignment.Round == round && !reviewAssignmentInactiveStatuses[assignment.StatusID] {
			count++
		}
	}
	return count
}

// latestReviewRound is the highest round number in the review rounds or the
// assignments, so a new round with no assignment yet still counts as latest.
func latestReviewRound(d SubmissionDetail) int {
	round := 0
	for _, r := range d.ReviewRounds {
		round = max(round, r.Round)
	}
	for _, a := range d.ReviewAssignments {
		round = max(round, a.Round)
	}
	return round
}
