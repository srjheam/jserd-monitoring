package normalizer

import (
	"strings"
	"testing"
)

func TestNormalizeReviewsPreservesReviewerDeadlines(t *testing.T) {
	csv := `Stage,Round,"Submission Title","Submission ID",Reviewer,"Date Assigned","Date Completed","Response Due Date","Review Due Date",Declined,Unconsidered
Review,1,"Sample article",100002,reviewer-one,"2025-01-02 03:04:05","2025-01-03 04:05:06","2025-01-10 23:59:59","2025-02-07 23:59:59",Yes,Yes
`
	reviews, err := NormalizeReviews(strings.NewReader(csv), ArtifactRef{Kind: "reviews", PathOrURL: "synthetic/reviews.csv"})
	if err != nil {
		t.Fatalf("normalize synthetic reviews: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("reviews = %d, want 1", len(reviews))
	}
	review := reviews[0]
	if review.Stage != "Review" || review.Round != 1 || review.SubmissionID != "100002" || review.ReviewerUsername != "reviewer-one" {
		t.Fatalf("review identity and round not preserved: %#v", review)
	}
	if review.DateAssigned != "2025-01-02 03:04:05" || review.DateCompleted != "2025-01-03 04:05:06" {
		t.Fatalf("review dates not preserved: %#v", review)
	}
	if review.ResponseDueDate != "2025-01-10 23:59:59" || review.ReviewDueDate != "2025-02-07 23:59:59" {
		t.Fatalf("due dates not preserved: %#v", review)
	}
	if !review.Declined || !review.Unconsidered {
		t.Fatalf("declined/unconsidered flags not preserved: %#v", review)
	}
}

func TestNormalizeReviewsHandlesBOMEmptyDatesAndOptionalColumns(t *testing.T) {
	csv := "\ufeffStage,Round,\"Submission Title\",\"Submission ID\",Reviewer,\"Given Name\",\"Family Name\",Email,\"Date Assigned\",\"Date Notified\",\"Date Confirmed\",\"Date Completed\",Unconsidered,\"Response Due Date\",\"Response Overdue Days\",\"Review Due Date\",\"Review Overdue Days\",Declined,Recommendation,\"Comments On Submission\"\n" +
		"Review,2,\"Paper\",77,jdoe,Jane,Doe,jane@example.test,\"2026-05-01 10:00:00\",,,,No,,3,,5,No,\"Accept Submission\",\"<p>ok</p>\"\n"

	reviews, err := NormalizeReviews(strings.NewReader(csv), ArtifactRef{Kind: "reviews", PathOrURL: "synthetic/reviews.csv"})
	if err != nil {
		t.Fatalf("normalize synthetic reviews: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("reviews = %d, want 1", len(reviews))
	}

	review := reviews[0]
	if review.Stage != "Review" || review.Round != 2 || review.SubmissionID != "77" {
		t.Fatalf("BOM or core fields not handled: %#v", review)
	}
	if review.DateConfirmed != "" || review.DateCompleted != "" {
		t.Fatalf("empty dates should stay empty, got confirmed=%q completed=%q", review.DateConfirmed, review.DateCompleted)
	}
	if review.ResponseOverdueDays != 3 || review.ReviewOverdueDays != 5 {
		t.Fatalf("overdue days not parsed: %#v", review)
	}
	if review.Declined || review.Unconsidered {
		t.Fatalf("No flags should be false: %#v", review)
	}
	if review.Key != "77|Review|2|jane@example.test|2026-05-01 10:00:00" {
		t.Fatalf("provisional review key = %q", review.Key)
	}
}
