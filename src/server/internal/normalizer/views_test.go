package normalizer

import (
	"strings"
	"testing"
)

func TestNormalizeViewsPreservesArticleMetricsAndDynamicGalleys(t *testing.T) {
	csv := `"Article ID","Article Title",Issue,"Date Published","Abstract Views","Total Galley Views",PDF,HTML
200001,"Sample article","Vol. 1",2025-03-04,12,0,0,0
`
	metrics, err := NormalizeViews(strings.NewReader(csv), ArtifactRef{Kind: "views", PathOrURL: "synthetic/views.csv"})
	if err != nil {
		t.Fatalf("normalize synthetic views: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("metrics = %d, want 1", len(metrics))
	}
	metric := metrics[0]
	if metric.ArticleID != "200001" {
		t.Fatalf("article id = %q, want 200001", metric.ArticleID)
	}
	if metric.Issue != "Vol. 1" || metric.DatePublished != "2025-03-04" {
		t.Fatalf("issue/date not preserved: %#v", metric)
	}
	if metric.AbstractViews != 12 || metric.TotalGalleyViews != 0 {
		t.Fatalf("view totals not preserved: %#v", metric)
	}
	if _, ok := metric.GalleyViews["PDF"]; !ok {
		t.Fatalf("expected dynamic PDF galley column, got %#v", metric.GalleyViews)
	}
	if _, ok := metric.GalleyViews["HTML"]; !ok {
		t.Fatalf("expected dynamic HTML galley column, got %#v", metric.GalleyViews)
	}
	assertProvenance(t, metric.Provenance["abstractViews"], "views", "Abstract Views")
}

func TestNormalizeViewsHandlesEmptyGalleyValuesAndOptionalDynamicColumns(t *testing.T) {
	csv := "\"Article ID\",\"Article Title\",Issue,\"Date Published\",\"Abstract Views\",\"Total Galley Views\",PDF,HTML\n" +
		"99,\"Paper\",\"Vol. 9\",,10,15,,15\n"

	metrics, err := NormalizeViews(strings.NewReader(csv), ArtifactRef{Kind: "views", PathOrURL: "synthetic/views.csv"})
	if err != nil {
		t.Fatalf("normalize synthetic views: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("metrics = %d, want 1", len(metrics))
	}

	metric := metrics[0]
	if metric.DatePublished != "" {
		t.Fatalf("empty date should stay empty, got %q", metric.DatePublished)
	}
	if metric.GalleyViews["PDF"] != 0 || metric.GalleyViews["HTML"] != 15 {
		t.Fatalf("galley views not preserved with empty value: %#v", metric.GalleyViews)
	}
}
