package events

import (
	"server/internal/normalizer"
	"testing"
	"time"
)

func copyeditingCycle(assigned *bool) normalizer.NormalizedCycle {
	return normalizer.NormalizedCycle{Submissions: []normalizer.Submission{{
		OJSSubmissionID: "42", DateSubmitted: "2026-09-01", Status: statusCopyediting, CopyeditorAssigned: assigned,
	}}}
}

func TestCopyeditingNotifiesOncePerMissingEpisode(t *testing.T) {
	absent, present := false, true
	missing, assigned := copyeditingCycle(&absent), copyeditingCycle(&present)
	now := time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	first := Copyediting(normalizer.NormalizedCycle{}, missing, now)
	if len(first) != 1 || len(MatchRules(first[0])) != 1 || MatchRules(first[0])[0].RuleCode != RuleRN18 {
		t.Fatalf("first episode=%#v", first)
	}
	if len(Copyediting(missing, missing, now.Add(time.Hour))) != 0 || len(Copyediting(missing, assigned, now)) != 0 {
		t.Fatal("unchanged or resolved absence notified")
	}
	again := Copyediting(assigned, missing, now.Add(2*time.Hour))
	if len(again) != 1 || again[0].IdempotencyKey == first[0].IdempotencyKey {
		t.Fatal("new missing episode did not get a distinct event")
	}
	replay := Copyediting(normalizer.NormalizedCycle{}, missing, now)
	if replay[0].IdempotencyKey != first[0].IdempotencyKey {
		t.Fatal("replaying the same cycle changed the event identity")
	}
}

func TestCopyeditingDoesNotGuessFromGenericEditors(t *testing.T) {
	cycle := copyeditingCycle(nil)
	cycle.Submissions[0].Editors = []normalizer.Editor{{GivenName: "Editor"}}
	if got := Copyediting(normalizer.NormalizedCycle{}, cycle, time.Now()); len(got) != 0 {
		t.Fatalf("unknown copyeditor assignment notified: %#v", got)
	}
}

func TestCopyeditingSkipsOtherStagesAndIncompleteSubmissions(t *testing.T) {
	absent := false
	for _, status := range []string{"Submission", "Review", "Production", "Published", "Declined"} {
		cycle := copyeditingCycle(&absent)
		cycle.Submissions[0].Status = status
		if len(Copyediting(normalizer.NormalizedCycle{}, cycle, time.Now())) != 0 {
			t.Fatalf("notified for %s", status)
		}
	}
	cycle := copyeditingCycle(&absent)
	cycle.Submissions[0].SubmissionProgress = 1
	if len(Copyediting(normalizer.NormalizedCycle{}, cycle, time.Now())) != 0 {
		t.Fatal("incomplete submission notified")
	}
}
