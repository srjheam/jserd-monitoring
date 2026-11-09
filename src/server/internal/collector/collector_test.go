package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"server/internal/config"
	"server/internal/normalizer"
	"server/internal/ojs"
	"strings"
	"testing"
	"time"
)

const (
	testSMTPFromAddress = "noreply@example.test"
	testOJSUsername     = "editor@example.test"
	testOJSPassword     = "top-secret"
)

func TestCollectorAppliesDateLastActivityBySubmission(t *testing.T) {
	submissions := []normalizer.Submission{{OJSSubmissionID: "42"}, {OJSSubmissionID: "43"}}
	pages := []ojs.SubmissionPage{{Items: []ojs.SubmissionFileCount{{SubmissionID: "42", DateLastActivity: "2026-08-15 14:18:37"}, {SubmissionID: "43"}}}}
	if err := applySubmissionMetadata(submissions, pages); err != nil {
		t.Fatal(err)
	}
	if submissions[0].DateLastActivity != "2026-08-15 14:18:37" {
		t.Fatalf("submission 42=%q", submissions[0].DateLastActivity)
	}
	if submissions[1].DateLastActivity != "" {
		t.Fatalf("unobserved submission got evidence: %q", submissions[1].DateLastActivity)
	}
}

func TestCollectorAppliesSubmissionProgressBySubmission(t *testing.T) {
	submissions := []normalizer.Submission{{OJSSubmissionID: "42"}, {OJSSubmissionID: "43"}, {OJSSubmissionID: "44"}}
	pages := []ojs.SubmissionPage{{Items: []ojs.SubmissionFileCount{{SubmissionID: "42", SubmissionProgress: 0}, {SubmissionID: "43", SubmissionProgress: 2}}}}
	if err := applySubmissionMetadata(submissions, pages); err != nil {
		t.Fatal(err)
	}
	if submissions[0].SubmissionProgress != 0 {
		t.Fatalf("submission 42 progress = %v, want 0", submissions[0].SubmissionProgress)
	}
	if submissions[1].SubmissionProgress != 2 {
		t.Fatalf("submission 43 progress = %v, want 2", submissions[1].SubmissionProgress)
	}
	if submissions[2].SubmissionProgress != 0 {
		t.Fatalf("unobserved submission 44 progress = %v, want default 0", submissions[2].SubmissionProgress)
	}
}

func TestCollectorRejectsDuplicateSubmissionEvidence(t *testing.T) {
	submissions := []normalizer.Submission{{OJSSubmissionID: "42"}}
	pages := []ojs.SubmissionPage{{Items: []ojs.SubmissionFileCount{{SubmissionID: "42"}, {SubmissionID: "42"}}}}
	if err := applySubmissionMetadata(submissions, pages); err == nil {
		t.Fatal("duplicate submission accepted")
	}
}

func TestFixtureCycleUsesSyntheticCSVsAndProducesArtifacts(t *testing.T) {
	cfg := config.Config{
		SourceMode:        config.SourceModeFixtures,
		FixturesDir:       syntheticFixtures(t),
		RequestTimeout:    30 * time.Second,
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}
	collector := Collector{
		Config: cfg,
		Clock: func() time.Time {
			return time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
		},
	}

	result, err := collector.Run(context.Background())
	if err != nil {
		t.Fatalf("fixture cycle failed: %v", err)
	}
	if result.Status != CycleStatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if len(result.Normalized.Submissions) == 0 || len(result.Normalized.Reviews) == 0 || len(result.Normalized.ViewMetrics) == 0 {
		t.Fatalf("fixture cycle missing normalized data: %#v", result.Normalized)
	}
	if len(result.Normalized.Artifacts) != 3 {
		t.Fatalf("normalized artifacts = %d, want 3", len(result.Normalized.Artifacts))
	}

	want := map[string]string{
		"articles": "data/examples/articles-JSERD-20260512.csv",
		"reviews":  "data/examples/reviews-20260512.csv",
		"views":    "data/examples/views-20260512.csv",
	}
	for kind, path := range want {
		assertFixtureArtifact(t, result, kind, path)
	}
}

func assertFixtureArtifact(t *testing.T, result CycleResult, kind, path string) {
	t.Helper()
	artifact, ok := result.Manifest.Artifact(kind)
	if !ok {
		t.Fatalf("missing %s artifact in manifest", kind)
	}
	if artifact.SourceMode != string(config.SourceModeFixtures) {
		t.Fatalf("%s sourceMode = %q", kind, artifact.SourceMode)
	}
	if filepath.ToSlash(artifact.PathOrURL) != path {
		t.Fatalf("%s path = %q, want %q", kind, artifact.PathOrURL, path)
	}
	if artifact.SHA256 == "" || artifact.SizeBytes == 0 {
		t.Fatalf("%s artifact missing hash/size: %#v", kind, artifact)
	}
}

func TestFixtureCycleDoesNotCopyBytesToArtifactRoot(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		SourceMode:        config.SourceModeFixtures,
		FixturesDir:       syntheticFixtures(t),
		ArtifactRoot:      filepath.Join(root, "collector_runs"),
		RequestTimeout:    30 * time.Second,
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}
	collector := Collector{Config: cfg}

	result, err := collector.Run(context.Background())
	if err != nil {
		t.Fatalf("fixture cycle failed: %v", err)
	}
	if result.Status != CycleStatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if _, err := os.Stat(cfg.ArtifactRoot); !os.IsNotExist(err) {
		t.Fatalf("fixtures should not create artifact root, stat err = %v", err)
	}
}

func TestFixtureCycleFailsClosedWhenRequiredFileMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "articles-JSERD-20260512.csv"), []byte("Submission ID,Title\n600140,Example\n"), 0o644); err != nil {
		t.Fatalf("write articles fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "reviews-20260512.csv"), []byte("Submission ID,Reviewer\n600140,Example\n"), 0o644); err != nil {
		t.Fatalf("write reviews fixture: %v", err)
	}
	cfg := config.Config{
		SourceMode:        config.SourceModeFixtures,
		FixturesDir:       dir,
		RequestTimeout:    30 * time.Second,
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}
	collector := Collector{Config: cfg}

	result, err := collector.Run(context.Background())
	if err == nil {
		t.Fatal("expected missing views fixture to fail")
	}
	if result.Status != CycleStatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if len(result.Normalized.Submissions) != 0 || len(result.Normalized.Reviews) != 0 || len(result.Normalized.ViewMetrics) != 0 {
		t.Fatalf("failed fixture cycle returned trusted normalized data: %#v", result.Normalized)
	}
	artifact, ok := result.Manifest.Artifact("views")
	if !ok {
		t.Fatal("missing failed views artifact")
	}
	if artifact.Status != "failed" || artifact.Error == "" {
		t.Fatalf("views artifact should record failure, got %#v", artifact)
	}
}

func TestLiveCycleFailsClosedWithoutFixtureFallback(t *testing.T) {
	cfg := config.Config{
		SourceMode:        config.SourceModeLive,
		BaseURL:           "https://journals-sol.sbc.org.br/index.php",
		JournalContext:    "jserd",
		Username:          testOJSUsername,
		Password:          testOJSPassword,
		FixturesDir:       syntheticFixtures(t),
		RequestTimeout:    time.Second,
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}
	collector := Collector{
		Config:     cfg,
		LiveClient: FailingLiveClient{Err: ErrAuthenticationFailed},
		Clock: func() time.Time {
			return time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
		},
	}

	result, err := collector.Run(context.Background())
	if err == nil {
		t.Fatal("expected live auth failure")
	}
	if result.Status != CycleStatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if len(result.Normalized.Submissions) != 0 || len(result.Normalized.Reviews) != 0 || len(result.Normalized.ViewMetrics) != 0 {
		t.Fatalf("failed live cycle returned trusted normalized data: %#v", result.Normalized)
	}
	for _, artifact := range result.Manifest.Artifacts {
		if artifact.SourceMode == string(config.SourceModeFixtures) {
			t.Fatalf("live failure fell back to fixtures artifact: %#v", artifact)
		}
		if artifact.Error != "" && (contains(artifact.Error, cfg.Password) || contains(artifact.Error, "cookie")) {
			t.Fatalf("artifact error leaked secret material: %#v", artifact)
		}
	}
}

func TestLiveCycleDownloadsReportsRESTAndNormalizesWithoutFixtureFallback(t *testing.T) {
	var reportCount int
	var restSeen bool
	server := newLiveOJSTestServer(t, liveCycleFullHandler(t, &reportCount, &restSeen))
	defer server.Close()

	cfg := liveTestConfig(server.URL+"/index.php", t.TempDir())
	collector := Collector{
		Config: cfg,
		Clock: func() time.Time {
			return time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
		},
	}

	result, err := collector.Run(context.Background())
	if err != nil {
		t.Fatalf("live cycle failed: %v", err)
	}
	if result.Status != CycleStatusOK {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if reportCount != 3 || !restSeen {
		t.Fatalf("live cycle did not request all sources: reports=%d rest=%v", reportCount, restSeen)
	}
	if len(result.Normalized.Submissions) != 1 || len(result.Normalized.Reviews) != 1 || len(result.Normalized.ViewMetrics) != 1 {
		t.Fatalf("live cycle missing normalized data: %#v", result.Normalized)
	}
	if len(result.Manifest.Artifacts) != 4 {
		t.Fatalf("manifest artifacts = %d, want 4", len(result.Manifest.Artifacts))
	}
	for _, kind := range []string{"articles", "reviews", "views", "rest-submissions"} {
		assertLiveArtifact(t, result, kind)
	}
}

// liveCycleFullHandler serves reports and the REST submissions endpoint,
// counting report requests and recording whether the REST endpoint was hit.
func liveCycleFullHandler(t *testing.T, reportCount *int, restSeen *bool) func(http.ResponseWriter, *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/index.php/jserd/management/tools/report":
			*reportCount++
			resp.Header().Set("Content-Type", "text/csv")
			switch req.URL.Query().Get("pluginName") {
			case "ArticleReportPlugin":
				_, _ = resp.Write([]byte("Submission ID,Title,Abstract,Status,URL,Date submitted,Last modified\n600140,Example,Abstract,Review,https://example.test/600140,2026-01-01,2026-01-02\n"))
			case "ReviewReportPlugin":
				_, _ = resp.Write([]byte("Stage,Round,Submission ID,Submission Title,Reviewer,Date Assigned\nReview,1,600140,Example,reviewer-one,2026-01-03\n"))
			case "ViewReportPlugin":
				_, _ = resp.Write([]byte("Article ID,Article Title,Issue,Date Published,Abstract Views,Total Galley Views\n9,Welcome,Vol 1,2026-01-04,556,10\n"))
			default:
				t.Fatalf("unexpected pluginName %q", req.URL.Query().Get("pluginName"))
			}
		case "/index.php/jserd/api/v1/submissions":
			if req.URL.Query().Get("count") != "100" || req.URL.Query().Get("offset") != "0" {
				t.Fatalf("REST query = %q, want count=100&offset=0", req.URL.RawQuery)
			}
			*restSeen = true
			resp.Header().Set("Content-Type", "application/json")
			_, _ = resp.Write([]byte(`{"items":[{"id":600140}],"itemsMax":1}`))
		default:
			t.Fatalf("unexpected live path %s", req.URL.String())
		}
	}
}

func assertLiveArtifact(t *testing.T, result CycleResult, kind string) {
	t.Helper()
	artifact, ok := result.Manifest.Artifact(kind)
	if !ok {
		t.Fatalf("missing %s artifact", kind)
	}
	if artifact.SourceMode != string(config.SourceModeLive) || artifact.Status != "ok" || artifact.SHA256 == "" || artifact.SizeBytes == 0 {
		t.Fatalf("%s artifact missing live audit metadata: %#v", kind, artifact)
	}
	if !strings.Contains(filepath.ToSlash(artifact.PathOrURL), "/raw/"+kind) {
		t.Fatalf("%s artifact path = %q", kind, artifact.PathOrURL)
	}
	if _, err := os.ReadFile(artifact.PathOrURL); err != nil {
		t.Fatalf("%s raw bytes not stored: %v", kind, err)
	}
}

func TestLiveCycleFailsClosedWhenRESTInvalid(t *testing.T) {
	server := newLiveOJSTestServer(t, func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/index.php/jserd/management/tools/report":
			resp.Header().Set("Content-Type", "text/csv")
			switch req.URL.Query().Get("pluginName") {
			case "ArticleReportPlugin":
				_, _ = resp.Write([]byte("Submission ID,Title,Abstract,Status,URL,Date submitted,Last modified\n600140,Example,Abstract,Review,https://example.test/600140,2026-01-01,2026-01-02\n"))
			case "ReviewReportPlugin":
				_, _ = resp.Write([]byte("Stage,Round,Submission ID,Reviewer,Date Assigned\nReview,1,600140,reviewer-one,2026-01-03\n"))
			case "ViewReportPlugin":
				_, _ = resp.Write([]byte("Article ID,Article Title,Issue,Date Published,Abstract Views,Total Galley Views\n9,Welcome,Vol 1,2026-01-04,556,10\n"))
			}
		case "/index.php/jserd/api/v1/submissions":
			resp.Header().Set("Content-Type", "application/json")
			_, _ = resp.Write([]byte("{"))
		default:
			t.Fatalf("unexpected live path %s", req.URL.String())
		}
	})
	defer server.Close()

	result, err := (Collector{Config: liveTestConfig(server.URL+"/index.php", t.TempDir())}).Run(context.Background())
	if err == nil {
		t.Fatal("expected invalid REST to fail")
	}
	if result.Status != CycleStatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if len(result.Normalized.Submissions) != 0 || len(result.Normalized.Reviews) != 0 || len(result.Normalized.ViewMetrics) != 0 {
		t.Fatalf("failed live cycle returned trusted normalized data: %#v", result.Normalized)
	}
	if _, ok := result.Manifest.Artifact("rest-submissions"); !ok {
		t.Fatal("missing failed REST artifact")
	}
}

func newLiveOJSTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/index.php/jserd/login/signIn":
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse login form: %v", err)
			}
			if req.Form.Get("username") != testOJSUsername || req.Form.Get("password") != testOJSPassword {
				t.Fatalf("bad login form: %#v", req.Form)
			}
			http.SetCookie(resp, &http.Cookie{Name: "OJSSID", Value: "session-1", Path: "/"})
			_, _ = resp.Write([]byte("ok"))
		default:
			if !hasLiveCookie(req) {
				t.Fatalf("live request missing OJSSID cookie: %#v", req.Cookies())
			}
			handler(resp, req)
		}
	}))
}

func liveTestConfig(baseURL, artifactRoot string) config.Config {
	return config.Config{
		SourceMode:        config.SourceModeLive,
		BaseURL:           baseURL,
		JournalContext:    "jserd",
		Username:          testOJSUsername,
		Password:          testOJSPassword,
		ArtifactRoot:      filepath.Join(artifactRoot, "collector_runs"),
		RequestTimeout:    time.Second,
		SMTPFromAddress:   testSMTPFromAddress,
		AllowTestDelivery: true,
	}
}

func hasLiveCookie(r *http.Request) bool {
	for _, cookie := range r.Cookies() {
		if cookie.Name == "OJSSID" && cookie.Value == "session-1" {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return substr != "" && strings.Contains(s, substr)
}

func syntheticFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"articles-JSERD-20260512.csv": "Submission ID,Title,Abstract,Status,URL,Date submitted,Last modified\n42,Example article,Example abstract,Review,https://example.test/42,2026-01-01,2026-01-02\n",
		"reviews-20260512.csv":        "Stage,Round,Submission ID,Reviewer,Date Assigned\nReview,1,42,reviewer-one,2026-01-03\n",
		"views-20260512.csv":          "Article ID,Article Title,Issue,Date Published,Abstract Views,Total Galley Views\n42,Example article,Example issue,2026-01-01,10,5\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSearchUpForFixturesFindsDirectoryAndPreservesFallback(t *testing.T) {
	const fallback = "fallback"
	root := t.TempDir()
	dir := filepath.Join(root, "data", "examples")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := searchUpForFixtures(filepath.Join(root, "nested", "package"), fallback); got != dir {
		t.Fatalf("fixtures=%q, want ancestor directory %q", got, dir)
	}
	if got := searchUpForFixtures(t.TempDir(), fallback); got != fallback {
		t.Fatalf("fixtures=%q, want unchanged fallback", got)
	}
}

func TestFixtureCycleFailsClosedWhenReportEmpty(t *testing.T) {
	dir := syntheticFixtures(t)
	if err := os.WriteFile(filepath.Join(dir, "articles-JSERD-20260512.csv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	collector := Collector{Config: config.Config{
		SourceMode: config.SourceModeFixtures, FixturesDir: dir,
		SMTPFromAddress: testSMTPFromAddress, AllowTestDelivery: true,
	}}
	result, err := collector.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "fixture is empty") || result.Status != CycleStatusFailed || len(result.Normalized.Submissions) != 0 {
		t.Fatalf("empty report must fail without trusted submissions: result=%#v, err=%v", result, err)
	}
}
