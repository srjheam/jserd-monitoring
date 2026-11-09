package normalizer

import (
	"strings"
	"testing"
)

const (
	articlesArtifactKind = "articles"
	articlesFixturePath  = "synthetic/articles.csv"
)

func TestNormalizeArticlesPreservesDomainFields(t *testing.T) {
	csv := `"Submission ID",Title,Abstract,"Given Name (Author 1)","Family Name (Author 1)","Given Name (Author 2)","Family Name (Author 2)",Status,URL,"Date submitted","Last modified","Given Name (Editor 1)","Family Name (Editor 1)","Email (Editor 1)","Editor Decision 1  (Editor 1)","Date decided 1  (Editor 1)"
100001,"Sample article","Synthetic abstract",Casey,Lee,Jordan,Reed,Declined,https://example.test/workflow/access/100001,"2025-01-02 03:04:05","2025-02-03 04:05:06",Riley,Park,riley@example.test,"Decline Submission","2025-03-04 05:06:07"
`
	submissions, err := NormalizeArticles(strings.NewReader(csv), ArtifactRef{Kind: articlesArtifactKind, PathOrURL: articlesFixturePath})
	if err != nil {
		t.Fatalf("normalize synthetic articles: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("submissions = %d, want 1", len(submissions))
	}
	submission := submissions[0]
	if submission.OJSSubmissionID != "100001" || submission.Title != "Sample article" || submission.AbstractText != "Synthetic abstract" {
		t.Fatalf("submission fields not preserved: %#v", submission)
	}
	if submission.Status != "Declined" || submission.URL != "https://example.test/workflow/access/100001" {
		t.Fatalf("status or URL not preserved: %#v", submission)
	}
	if submission.DateSubmitted != "2025-01-02 03:04:05" || submission.LastModified != "2025-02-03 04:05:06" {
		t.Fatalf("dates not preserved: %#v", submission)
	}
	if len(submission.Authors) != 2 || submission.Authors[0].GivenName != "Casey" || submission.Authors[0].FamilyName != "Lee" || submission.Authors[1].GivenName != "Jordan" || submission.Authors[1].FamilyName != "Reed" {
		t.Fatalf("authors not preserved: %#v", submission.Authors)
	}
	if len(submission.Editors) != 1 || submission.Editors[0].GivenName != "Riley" || submission.Editors[0].FamilyName != "Park" || submission.Editors[0].Email != "riley@example.test" {
		t.Fatalf("editor not preserved: %#v", submission.Editors)
	}
	if len(submission.Editors[0].Decisions) != 1 || submission.Editors[0].Decisions[0].Decision != "Decline Submission" || submission.Editors[0].Decisions[0].DateDecided != "2025-03-04 05:06:07" {
		t.Fatalf("editor decision not preserved: %#v", submission.Editors[0].Decisions)
	}
	assertProvenance(t, submission.Provenance["status"], articlesArtifactKind, "Status")
	assertProvenance(t, submission.Provenance["url"], articlesArtifactKind, "URL")
}

func TestNormalizeArticlesHandlesBOMEmptyDatesMissingOptionalColumnsAndRepeatedPeople(t *testing.T) {
	csv := "\ufeff\"Submission ID\",Title,Abstract,\"Given Name (Author 1)\",\"Family Name (Author 1)\",\"Email (Author 1)\",\"Given Name (Author 2)\",\"Family Name (Author 2)\",Status,URL,DOI,\"Date submitted\",\"Last modified\",\"Given Name (Editor 1)\",\"Family Name (Editor 1)\",\"Email (Editor 1)\",\"Editor Decision 1  (Editor 1)\",\"Date decided 1  (Editor 1)\"\n" +
		"42,\"Edge paper\",\"Resumo\",Ana,Silva,ana@example.test,Ana,Silva,Review,https://example.test/42,,," +
		",Editor,Um,editor@example.test,\"Send to Review\",\"2026-05-12 10:00:00\"\n"

	submissions, err := NormalizeArticles(strings.NewReader(csv), ArtifactRef{Kind: articlesArtifactKind, PathOrURL: articlesFixturePath})
	if err != nil {
		t.Fatalf("normalize synthetic articles: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("submissions = %d, want 1", len(submissions))
	}

	submission := submissions[0]
	if submission.OJSSubmissionID != "42" {
		t.Fatalf("BOM was not handled; submission id = %q", submission.OJSSubmissionID)
	}
	if submission.DateSubmitted != "" || submission.LastModified != "" {
		t.Fatalf("empty dates should stay empty, got submitted=%q modified=%q", submission.DateSubmitted, submission.LastModified)
	}
	if len(submission.Authors) != 2 {
		t.Fatalf("expected repeated authors to be preserved, got %#v", submission.Authors)
	}
	if submission.Authors[1].Sequence != 2 {
		t.Fatalf("second author sequence = %d, want 2", submission.Authors[1].Sequence)
	}
	if len(submission.Editors) != 1 || len(submission.Editors[0].Decisions) != 1 {
		t.Fatalf("expected one editor decision, got %#v", submission.Editors)
	}
}

func TestCSVEdgeCasesRequiredColumnMissingFails(t *testing.T) {
	csv := "\"Submission ID\",Title,Abstract,URL,\"Date submitted\",\"Last modified\"\n" +
		"42,\"Edge paper\",\"Resumo\",https://example.test/42,,\n"

	_, err := NormalizeArticles(strings.NewReader(csv), ArtifactRef{Kind: articlesArtifactKind, PathOrURL: articlesFixturePath})
	if err == nil {
		t.Fatal("expected missing required Status column to fail")
	}
	if !strings.Contains(err.Error(), `missing required column "Status"`) {
		t.Fatalf("error = %q, want missing Status column", err.Error())
	}
}

func assertProvenance(t *testing.T, got FieldProvenance, source, column string) {
	t.Helper()

	if got.Source != source || got.Column != column || got.Artifact == "" {
		t.Fatalf("provenance = %#v, want source=%s column=%s with artifact", got, source, column)
	}
}
