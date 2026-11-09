package editorial

// catalog is the one place a finding kind is declared. It lists every kind the
// evaluator can produce, from the worst to the mildest, with the sentence the
// editorial summary e-mail writes for it.
//
// The order is the rank: Rank, PriorityFor and the sort in evaluateSubmission
// all read it, and so does the work list, which receives its findings already
// sorted rather than ranking them again. A kind's two client-facing labels stay
// in messages/en.json and messages/pt-BR.json, because they are translated; a
// test asserts that every kind here has both of them.
//
// To add a finding kind: declare the constant in evaluator.go, add one row
// here at its rank, then run the tests. Two will fail, and each one names the
// file it wants you to update.
var catalog = []catalogEntry{
	{FindingMissingEditor, "The submission has no editor assigned."},
	{FindingMissingCopyeditor, "The Copyediting stage has no editor assigned."},
	{FindingUnsentReviewCompletion, "The review completion notice was not sent yet."},
	{FindingReviewsCompleteNoDecision, "Every review is complete and no decision followed."},
	{FindingRevisionPendingDecision, "The revised version arrived and no editorial decision followed."},
	{FindingMissingRevisionIndication, "No sign of a revised version submission was found."},
	{FindingOverdueReview, "The accepted review is past its deadline."},
	{FindingMissingReviewer, "There is no active reviewer for this submission."},
	// GAP: overdue_review_invitation has no sentence of its own and falls back
	// to findingText's generic one. That is the behavior before this catalog
	// existed, kept unchanged here on purpose. Writing a real sentence is a
	// change to the editorial summary e-mail and belongs in its own ticket.
	{FindingOverdueReviewInvitation, ""},
	{FindingAwaitingDeskDecision, "The submission is waiting for a decision to send it to review."},
}

type catalogEntry struct {
	Kind          FindingKind
	EmailSentence string
}

// emailSentence returns the sentence the editorial summary e-mail writes for a
// kind. A kind the catalog does not declare, and a kind whose sentence is still
// empty, both get the generic one.
func emailSentence(kind FindingKind) string {
	rank := rankOf(kind)
	if rank == len(catalog) || catalog[rank].EmailSentence == "" {
		return "The submission requires editorial follow-up."
	}
	return catalog[rank].EmailSentence
}

// rankOf returns a kind's position in the catalog, worst first counting from
// zero. An unknown kind ranks one past the mildest known one, so it sorts last
// and never displaces a kind the catalog does declare.
func rankOf(kind FindingKind) int {
	for i, entry := range catalog {
		if entry.Kind == kind {
			return i
		}
	}
	return len(catalog)
}
