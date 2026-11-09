package diff

import (
	"server/internal/audit"
	"server/internal/normalizer"
	"testing"
	"time"
)

const rn5HintType = "revised_version_hint"

func TestRN5HintFiresWhenRevisionPendingClears(t *testing.T) {
	previous := rn5Cycle("42", true)
	current := rn5Cycle("42", false)
	current.CollectedAt = previous.CollectedAt.Add(time.Hour)

	changes := Compare(previous, current)
	if got := countType(changes, rn5HintType); got != 1 {
		t.Fatalf("RN5 hint missing: got %d changes; all=%#v", got, changes)
	}
	change := firstType(changes, rn5HintType)
	if change.EntityKey != "42" {
		t.Fatalf("RN5 hint evidence = %#v", change)
	}
}

func TestRN5HintRejectsUnobservedOrUnchangedEvidence(t *testing.T) {
	tests := []struct {
		name    string
		before  *bool
		current *bool
	}{
		{"still pending", boolPtr(true), boolPtr(true)},
		{"still not pending", boolPtr(false), boolPtr(false)},
		{"newly observed, not a transition", nil, boolPtr(false)},
		{"no longer observed", boolPtr(true), nil},
		{"reverted from submitted back to pending", boolPtr(false), boolPtr(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := rn5CycleWith("42", tt.before)
			current := rn5CycleWith("42", tt.current)
			current.CollectedAt = previous.CollectedAt.Add(time.Hour)
			if changes := Compare(previous, current); countType(changes, rn5HintType) != 0 {
				t.Fatalf("invalid evidence produced RN5 hint: %#v", changes)
			}
		})
	}
}

func TestRN5HintRequiresSameSubmission(t *testing.T) {
	previous := rn5Cycle("42", true)
	current := rn5Cycle("43", false)
	current.CollectedAt = previous.CollectedAt.Add(time.Hour)
	if changes := Compare(previous, current); countType(changes, rn5HintType) != 0 {
		t.Fatalf("different submissions produced RN5 hint: %#v", changes)
	}
}

func rn5Cycle(submissionID string, pending bool) normalizer.NormalizedCycle {
	return rn5CycleWith(submissionID, &pending)
}

func rn5CycleWith(submissionID string, pending *bool) normalizer.NormalizedCycle {
	c := cycle()
	c.Submissions[0].OJSSubmissionID = submissionID
	c.Submissions[0].RevisionPending = pending
	return c
}

func boolPtr(v bool) *bool { return &v }

func firstType(changes []Change, want string) Change {
	for _, change := range changes {
		if change.Type == want {
			return change
		}
	}
	return Change{}
}

func TestCompareDetectsOnlyEditorialTransitions(t *testing.T) {
	base := cycle(review("r1", "", "", false), review("r2", "", "", false))
	tests := []struct {
		name     string
		mutate   func(*normalizer.NormalizedCycle)
		wantType string
	}{
		{"invitation accepted", func(c *normalizer.NormalizedCycle) { c.Reviews[0].DateConfirmed = "2026-06-01T10:00:00Z" }, ChangeReviewInvitationAccepted},
		{"invitation rejected", func(c *normalizer.NormalizedCycle) { c.Reviews[0].Declined = true }, ChangeReviewInvitationRejected},
		{"review submitted", func(c *normalizer.NormalizedCycle) { c.Reviews[0].DateCompleted = "2026-06-02T10:00:00Z" }, ChangeReviewSubmitted},
		{"all reviews completed", func(c *normalizer.NormalizedCycle) {
			c.Reviews[0].DateCompleted = "2026-06-02T10:00:00Z"
			c.Reviews[1].DateCompleted = "2026-06-03T10:00:00Z"
		}, ChangeAllReviewsCompleted},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			current := cloneCycle(base)
			testCase.mutate(&current)
			changes := Compare(base, current)
			if !containsType(changes, testCase.wantType) {
				t.Fatalf("Compare() types = %v, want %q", changeTypes(changes), testCase.wantType)
			}
		})
	}
}

func TestCompareIgnoresMetadataOrderAndMissingRecords(t *testing.T) {
	base := cycle(review("r1", "2026-06-01", "", false), review("r2", "", "", false))
	current := cloneCycle(base)
	current.CollectedAt = current.CollectedAt.Add(time.Hour)
	current.Artifacts = []audit.RawArtifact{{Kind: "reviews", SHA256: "different"}}
	current.Submissions[0].LastModified = "2026-06-28"
	current.Submissions[0].Provenance = map[string]normalizer.FieldProvenance{"title": {Source: "other"}}
	current.Reviews[0], current.Reviews[1] = current.Reviews[1], current.Reviews[0]
	current.Reviews = current.Reviews[:1]

	if changes := Compare(base, current); len(changes) != 0 {
		t.Fatalf("metadata, order, LastModified, provenance, and missing records produced changes: %#v", changes)
	}
}

func TestCompareRejectsWithoutAcceptingWhenConfirmationAndDeclineArriveTogether(t *testing.T) {
	previous := cycle(review("r1", "", "", false))
	current := cycle(review("r1", "2026-06-01T10:00:00Z", "", true))

	changes := Compare(previous, current)
	if got := countType(changes, ChangeReviewInvitationRejected); got != 1 {
		t.Fatalf("rejected changes = %d, want 1; changes = %#v", got, changes)
	}
	if got := countType(changes, ChangeReviewInvitationAccepted); got != 0 {
		t.Fatalf("accepted changes = %d, want 0; changes = %#v", got, changes)
	}
}

// TestCompareDetectsAcceptanceWhenDeclinedIsPermanentlyTrue reproduces OJS's
// ReviewReportPlugin export, which marks every row Declined: Yes regardless
// of real status. An accepted invitation must still be detectable from the
// DateConfirmed transition alone when Declined was already true before the
// transition too, since that is the export's constant noise, not a real
// decline.
func TestCompareDetectsAcceptanceWhenDeclinedIsPermanentlyTrue(t *testing.T) {
	previous := cycle(review("r1", "", "", true))
	current := cycle(review("r1", "2026-06-01T10:00:00Z", "", true))

	changes := Compare(previous, current)
	if got := countType(changes, ChangeReviewInvitationAccepted); got != 1 {
		t.Fatalf("accepted changes = %d, want 1; changes = %#v", got, changes)
	}
	if got := countType(changes, ChangeReviewInvitationRejected); got != 0 {
		t.Fatalf("rejected changes = %d, want 0; changes = %#v", got, changes)
	}
}

func TestCompareDoesNotCompleteGroupWhenIncompleteAssignmentIsRemoved(t *testing.T) {
	previous := cycle(
		review("r1", "", "", false),
		review("r2", "", "", false),
	)
	current := cycle(review("r2", "", "2026-06-02T10:00:00Z", false))

	if changes := Compare(previous, current); countType(changes, ChangeAllReviewsCompleted) != 0 {
		t.Fatalf("removed incomplete assignment produced completion: %#v", changes)
	}
}

func TestCompareDoesNotCompleteNewAlreadyCompletedGroup(t *testing.T) {
	previous := cycle()
	current := cycle(review("r1", "", "2026-06-02T10:00:00Z", false))

	if changes := Compare(previous, current); countType(changes, ChangeAllReviewsCompleted) != 0 {
		t.Fatalf("new completed group produced completion: %#v", changes)
	}
}

func TestCompareCompletesPreexistingGroupWithIdenticalAssignments(t *testing.T) {
	previous := cycle(
		review("r1", "", "", false),
		review("r2", "", "2026-06-02T10:00:00Z", false),
	)
	current := cycle(
		review("r1", "", "2026-06-03T10:00:00Z", false),
		review("r2", "", "2026-06-02T10:00:00Z", false),
	)

	changes := Compare(previous, current)
	if got := countType(changes, ChangeAllReviewsCompleted); got != 1 {
		t.Fatalf("completion changes = %d, want 1; changes = %#v", got, changes)
	}
}

func cycle(reviews ...normalizer.ReviewAssignment) normalizer.NormalizedCycle {
	return normalizer.NormalizedCycle{
		SourceMode: "fixtures", CollectedAt: time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		Submissions: []normalizer.Submission{{OJSSubmissionID: "42", Title: "Paper"}}, Reviews: reviews,
	}
}

func review(key, confirmed, completed string, declined bool) normalizer.ReviewAssignment {
	return normalizer.ReviewAssignment{Key: key, SubmissionID: "42", Stage: "Review", Round: 1, DateAssigned: "2026-05-01", DateConfirmed: confirmed, DateCompleted: completed, Declined: declined}
}

func cloneCycle(in normalizer.NormalizedCycle) normalizer.NormalizedCycle {
	out := in
	out.Submissions = append([]normalizer.Submission(nil), in.Submissions...)
	out.Reviews = append([]normalizer.ReviewAssignment(nil), in.Reviews...)
	out.Artifacts = append([]audit.RawArtifact(nil), in.Artifacts...)
	return out
}

func containsType(changes []Change, want string) bool {
	for _, change := range changes {
		if change.Type == want {
			return true
		}
	}
	return false
}

func changeTypes(changes []Change) []string {
	types := make([]string, len(changes))
	for i := range changes {
		types[i] = changes[i].Type
	}
	return types
}

func countType(changes []Change, want string) int {
	count := 0
	for _, change := range changes {
		if change.Type == want {
			count++
		}
	}
	return count
}
