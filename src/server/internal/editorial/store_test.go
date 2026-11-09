package editorial

import (
	"server/internal/collections"
	"testing"

	_ "server/migrations"
)

func TestReplaceFindingsReplacesWholesale(t *testing.T) {
	app := editorialTestApp(t)
	store := NewStore(app)
	first := []ArticleFinding{
		{SubmissionID: "42", Title: testSubmissionTitle, URL: testSubmissionURL, DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingMissingReviewer}}},
		{SubmissionID: "43", DateSubmitted: "2026-01-02", Findings: []Finding{{Kind: FindingMissingEditor}}},
	}
	if err := store.ReplaceFindings("cycle-1", first); err != nil {
		t.Fatal(err)
	}
	rows, err := app.FindAllRecords(collections.EditorialFindings)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}

	second := []ArticleFinding{{SubmissionID: "44", DateSubmitted: testDate20260201, Findings: []Finding{{Kind: FindingOverdueReview}}}}
	if err := store.ReplaceFindings("cycle-2", second); err != nil {
		t.Fatal(err)
	}
	rows, err = app.FindAllRecords(collections.EditorialFindings)
	if err != nil || len(rows) != 1 || rows[0].GetString("submission_key") != "44" {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func TestReplaceFindingsEmptySetClearsTable(t *testing.T) {
	app := editorialTestApp(t)
	store := NewStore(app)
	if err := store.ReplaceFindings("cycle-1", []ArticleFinding{{SubmissionID: "42", DateSubmitted: testDate20260101, Findings: []Finding{{Kind: FindingMissingReviewer}}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceFindings("cycle-2", nil); err != nil {
		t.Fatal(err)
	}
	rows, err := app.FindAllRecords(collections.EditorialFindings)
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
}
