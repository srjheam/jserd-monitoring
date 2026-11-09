package ojs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"server/internal/audit"
	"strings"
	"testing"
	"time"
)

const (
	loginPath        = "/index.php/jserd/login/signIn"
	sessionCookieKey = "OJSSID"
	sessionCookieVal = "session-1"
	contentTypeCSV   = "text/csv"
)

func TestSessionLoginUsesCookieJar(t *testing.T) {
	var loginSeen bool
	var protectedSeen bool

	server := httptest.NewServer(cookieJarTestHandler(t, &loginSeen, &protectedSeen))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor@example.test", "top-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/index.php/jserd/protected", nil)
	if err != nil {
		t.Fatalf("new protected request: %v", err)
	}
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		t.Fatalf("protected request: %v", err)
	}
	_ = resp.Body.Close()

	if !loginSeen || !protectedSeen {
		t.Fatalf("loginSeen=%v protectedSeen=%v", loginSeen, protectedSeen)
	}
}

func cookieJarTestHandler(t *testing.T, loginSeen, protectedSeen *bool) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case loginPath:
			if req.Method != http.MethodPost {
				t.Fatalf("login method = %s, want POST", req.Method)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse login form: %v", err)
			}
			if req.Form.Get("username") != "editor@example.test" || req.Form.Get("password") != "top-secret" {
				t.Fatalf("login form = %#v", req.Form)
			}
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			*loginSeen = true
			resp.WriteHeader(http.StatusOK)
			_, _ = resp.Write([]byte("ok"))
		case "/index.php/jserd/protected":
			if !hasCookie(req, sessionCookieKey, sessionCookieVal) {
				t.Fatalf("protected request missing OJSSID cookie: %#v", req.Cookies())
			}
			*protectedSeen = true
			resp.WriteHeader(http.StatusOK)
			_, _ = resp.Write([]byte("ok"))
		default:
			t.Fatalf("unexpected path %s", req.URL.String())
		}
	}
}

func TestSessionLoginAcceptsSuccessfulHTMLRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case loginPath:
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			http.Redirect(resp, req, "/index.php/jserd/submissions", http.StatusFound)
		case "/index.php/jserd/submissions":
			if !hasCookie(req, sessionCookieKey, sessionCookieVal) {
				t.Fatalf("redirect missing OJSSID cookie: %#v", req.Cookies())
			}
			resp.Header().Set("Content-Type", "text/html")
			_, _ = resp.Write([]byte("<!doctype html><html><body>Submissions</body></html>"))
		default:
			t.Fatalf("unexpected path %s", req.URL.String())
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor@example.test", "top-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestSessionLoginFailsClosedWithoutSecretLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>Login failed</html>"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	err := client.Login(context.Background(), "editor@example.test", "top-secret")
	if err == nil {
		t.Fatal("expected invalid login to fail")
	}
	if strings.Contains(err.Error(), "top-secret") || strings.Contains(err.Error(), "session") {
		t.Fatalf("login error leaked secret material: %v", err)
	}
}

func TestReportDownloadUsesSessionCookieAndDownloadsReports(t *testing.T) {
	var loginSeen bool
	reportPlugins := map[string]bool{}

	server := httptest.NewServer(reportDownloadTestHandler(t, &loginSeen, reportPlugins))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor@example.test", "top-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}
	for _, plugin := range []ReportKind{ReportKindArticles, ReportKindReviews, ReportKindViews} {
		body, artifactURL, err := client.DownloadReport(context.Background(), plugin)
		if err != nil {
			t.Fatalf("download %s: %v", plugin, err)
		}
		if len(body) == 0 {
			t.Fatalf("%s report body is empty", plugin)
		}
		if !strings.Contains(artifactURL, "pluginName="+url.QueryEscape(string(plugin))) {
			t.Fatalf("artifact URL did not preserve plugin/path: %s", artifactURL)
		}
	}

	if !loginSeen {
		t.Fatal("login endpoint was not called")
	}
	for _, plugin := range []ReportKind{ReportKindArticles, ReportKindReviews, ReportKindViews} {
		if !reportPlugins[string(plugin)] {
			t.Fatalf("report plugin %s was not requested", plugin)
		}
	}
}

func reportDownloadTestHandler(t *testing.T, loginSeen *bool, reportPlugins map[string]bool) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case loginPath:
			if req.Method != http.MethodPost {
				t.Fatalf("login method = %s, want POST", req.Method)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse login form: %v", err)
			}
			if req.Form.Get("username") != "editor@example.test" || req.Form.Get("password") != "top-secret" {
				t.Fatalf("login form = %#v", req.Form)
			}
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			*loginSeen = true
			resp.WriteHeader(http.StatusOK)
			_, _ = resp.Write([]byte("ok"))
		case "/index.php/jserd/management/tools/report":
			if !hasCookie(req, sessionCookieKey, sessionCookieVal) {
				t.Fatalf("report request missing OJSSID cookie: %#v", req.Cookies())
			}
			plugin := req.URL.Query().Get("pluginName")
			reportPlugins[plugin] = true
			resp.Header().Set("Content-Type", contentTypeCSV)
			switch plugin {
			case string(ReportKindArticles):
				_, _ = resp.Write([]byte("Submission ID,Title\n600140,Example\n"))
			case string(ReportKindReviews):
				_, _ = resp.Write([]byte("Stage,Round,Submission ID,Reviewer\nReview,1,600219,reviewer-one\n"))
			case string(ReportKindViews):
				_, _ = resp.Write([]byte("Article ID,Article Title,Abstract Views\n9,Welcome,556\n"))
			default:
				t.Fatalf("unexpected report plugin %q", plugin)
			}
		default:
			t.Fatalf("unexpected path %s", req.URL.String())
		}
	}
}

func TestClientGetJSONUsesSelectedRESTEndpointWithSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case loginPath:
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			_, _ = resp.Write([]byte("ok"))
		case "/index.php/jserd/api/v1/submissions":
			if !hasCookie(req, sessionCookieKey, sessionCookieVal) {
				t.Fatalf("REST request missing OJSSID cookie")
			}
			if req.URL.Query().Get("count") != "1" {
				t.Fatalf("REST count query = %q, want 1", req.URL.RawQuery)
			}
			resp.Header().Set("Content-Type", "application/json")
			_, _ = resp.Write([]byte(`{"items":[{"id":600140}],"itemsMax":1}`))
		default:
			t.Fatalf("unexpected path %s", req.URL.String())
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor@example.test", "top-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}

	body, artifactURL := assertRESTSubmissionsDecoded(t, client)
	assertRESTArtifactStored(t, body, artifactURL)
}

func assertRESTSubmissionsDecoded(t *testing.T, client *Client) ([]byte, string) {
	t.Helper()
	var response struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
		ItemsMax int `json:"itemsMax"`
	}
	body, artifactURL, err := client.GetJSON(context.Background(), "submissions?count=1", &response)
	if err != nil {
		t.Fatalf("REST submissions: %v", err)
	}
	if len(body) == 0 || !strings.Contains(artifactURL, "/api/v1/submissions?count=1") {
		t.Fatalf("REST artifact not preserved: body=%d url=%s", len(body), artifactURL)
	}
	if len(response.Items) != 1 || response.Items[0].ID != 600140 || response.ItemsMax != 1 {
		t.Fatalf("REST response not decoded: %#v", response)
	}
	return body, artifactURL
}

func assertRESTArtifactStored(t *testing.T, body []byte, artifactURL string) {
	t.Helper()
	store := audit.ArtifactStore{Root: filepath.Join(t.TempDir(), "collector_runs")}
	artifact, err := StoreDownloadedArtifact(store, "run-1", "rest-submissions", "json", artifactURL, body, time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("store REST artifact: %v", err)
	}
	if artifact.Status != audit.ArtifactStatusOK || artifact.SHA256 == "" || artifact.SizeBytes != int64(len(body)) {
		t.Fatalf("REST artifact missing audit metadata: %#v", artifact)
	}
	if !strings.Contains(filepath.ToSlash(artifact.PathOrURL), "collector_runs/run-1/raw/rest-submissions.json") {
		t.Fatalf("REST artifact path = %q", artifact.PathOrURL)
	}
	if _, err := os.ReadFile(artifact.PathOrURL); err != nil {
		t.Fatalf("stored REST bytes not readable: %v", err)
	}
}

func TestClientPaginatesSubmissions(t *testing.T) {
	var offsets []string
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		offsets = append(offsets, req.URL.Query().Get("offset"))
		resp.Header().Set("Content-Type", "application/json")
		switch req.URL.Query().Get("offset") {
		case "0":
			_, _ = resp.Write([]byte(`{"items":[{"id":42,"dateLastActivity":"2026-08-15 14:18:37"},{"id":43}],"itemsMax":3}`))
		case "2":
			_, _ = resp.Write([]byte(`{"items":[{"id":44}],"itemsMax":3}`))
		default:
			t.Fatalf("unexpected offset %q", req.URL.Query().Get("offset"))
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL+"/index.php")
	pages, err := client.ListSubmissions(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || len(pages[0].Body) == 0 || len(pages[1].Body) == 0 {
		t.Fatalf("pages=%#v", pages)
	}
	if strings.Join(offsets, ",") != "0,2" {
		t.Fatalf("offsets=%v", offsets)
	}
	// dateLastActivity anchors the dashboard's "Waiting for" line. It is the
	// only field that moves when a reviewer completes a review.
	if pages[0].Items[0].DateLastActivity != "2026-08-15 14:18:37" {
		t.Fatalf("dateLastActivity=%q", pages[0].Items[0].DateLastActivity)
	}
	if pages[0].Items[1].DateLastActivity != "" {
		t.Fatalf("absent dateLastActivity became evidence: %#v", pages[0].Items[1])
	}
}

func TestListUserGroupsPaginatesWithSession(t *testing.T) {
	var offsets []string
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case loginPath:
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			_, _ = resp.Write([]byte("ok"))
		case "/index.php/jserd/api/v1/users":
			if !hasCookie(req, sessionCookieKey, sessionCookieVal) {
				t.Fatal("missing session")
			}
			offsets = append(offsets, req.URL.Query().Get("offset"))
			resp.Header().Set("Content-Type", "application/json")
			if req.URL.Query().Get("offset") == "0" {
				_, _ = resp.Write([]byte(`{"items":[{"id":1,"fullName":"A","email":"a@example.test","groups":[{"id":"chief","name":"Chefes"}]}],"itemsMax":2}`))
			} else {
				_, _ = resp.Write([]byte(`{"items":[{"id":2,"fullName":"B","email":"b@example.test","groups":[{"id":"chief","name":"Chefes"}]}],"itemsMax":2}`))
			}
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor", "secret"); err != nil {
		t.Fatal(err)
	}
	users, err := client.ListUserGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].Groups[0].ID != "chief" || strings.Join(offsets, ",") != "0,1" {
		t.Fatalf("users=%#v offsets=%v", users, offsets)
	}
}

func TestClientFetchesSubmissionDetailFromDetailEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case loginPath:
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			_, _ = resp.Write([]byte("ok"))
		case "/index.php/jserd/api/v1/submissions/600348":
			if !hasCookie(req, sessionCookieKey, sessionCookieVal) {
				t.Fatalf("detail request missing OJSSID cookie")
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("detail request carried unexpected query: %s", req.URL.RawQuery)
			}
			resp.Header().Set("Content-Type", "application/json")
			_, _ = resp.Write([]byte(`{"id":600348,"reviewRounds":[{"round":1,"statusId":11}],"reviewAssignments":[
				{"id":110001,"statusId":4,"round":1,"due":"2026-08-27"}
			]}`))
		default:
			t.Fatalf("unexpected path %s", req.URL.String())
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor@example.test", "top-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}
	detail, err := client.SubmissionDetail(context.Background(), "600348")
	if err != nil {
		t.Fatalf("submission detail: %v", err)
	}
	pending, observed := detail.LatestRoundRevisionPending()
	if !observed || pending {
		t.Fatalf("pending=%v observed=%v, want observed and not pending (statusId 11 is revisions submitted)", pending, observed)
	}
	if rounds := detail.OverdueReviewRounds(); len(rounds) != 1 || !rounds[1] {
		t.Fatalf("rounds=%#v, want only round 1", rounds)
	}
}

func TestSubmissionDetailLatestRoundRevisionPending(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantPending  bool
		wantObserved bool
	}{
		{"revisions requested", `{"reviewRounds":[{"round":1,"statusId":1}]}`, true, true},
		{"resubmit for review", `{"reviewRounds":[{"round":1,"statusId":2}]}`, true, true},
		{"revisions submitted", `{"reviewRounds":[{"round":1,"statusId":11}]}`, false, true},
		{"resubmit submitted", `{"reviewRounds":[{"round":1,"statusId":15}]}`, false, true},
		{"latest round wins over an earlier pending round", `{"reviewRounds":[{"round":1,"statusId":1},{"round":2,"statusId":11}]}`, false, true},
		{"no rounds", `{}`, false, false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var detail SubmissionDetail
			if err := json.Unmarshal([]byte(testCase.body), &detail); err != nil {
				t.Fatalf("decode: %v", err)
			}
			pending, observed := detail.LatestRoundRevisionPending()
			if pending != testCase.wantPending || observed != testCase.wantObserved {
				t.Fatalf("pending=%v observed=%v, want pending=%v observed=%v", pending, observed, testCase.wantPending, testCase.wantObserved)
			}
		})
	}
}

func TestSubmissionDetailOverdueReviewRounds(t *testing.T) {
	detail := SubmissionDetail{ReviewAssignments: []reviewAssignmentWindow{
		{StatusID: 4, Round: 1},
		{StatusID: 9, Round: 1},
		{StatusID: 7, Round: 2},
		{StatusID: 1, Round: 2},
	}}
	rounds := detail.OverdueReviewRounds()
	if len(rounds) != 1 || !rounds[1] {
		t.Fatalf("rounds=%#v, want only round 1 (statusId 4 is response-overdue, not statusId 9 complete, 7 submitted, or 1 declined)", rounds)
	}
}

func TestSubmissionDetailResolvedReviewWindows(t *testing.T) {
	detail := SubmissionDetail{ReviewAssignments: []reviewAssignmentWindow{
		{StatusID: 8, Round: 1, Due: "2026-08-24"},
		{StatusID: 8, Round: 1, Due: "2026-08-24"},
		{StatusID: 5, Round: 2, Due: "2026-09-09"},
		{StatusID: 7, Round: 2, Due: "2026-09-09"},
		{StatusID: 5, Round: 1, Due: "2026-10-05"},
	}}
	windows := detail.ResolvedReviewWindows()
	if !windows["1|2026-08-24"] {
		t.Fatalf("windows=%#v, want round 1 due 2026-08-24 resolved (both assignments statusId 8, completed)", windows)
	}
	if windows["2|2026-09-09"] {
		t.Fatalf("windows=%#v, want round 2 due 2026-09-09 not resolved (statusId 5 accepted is still pending, even though statusId 7 submitted shares the window)", windows)
	}
	if _, ok := windows["1|2026-08-27"]; ok {
		t.Fatalf("windows=%#v, want no entry for a due date with no live reviewAssignment", windows)
	}
	// A window with a live reviewAssignment that is pending, and no
	// resolved assignment sharing the same key, must still read as "not
	// resolved" — before this fix, only keys seen by a resolved assignment
	// ever entered the map, so a purely pending window was
	// indistinguishable from "no live match at all" and read as
	// superseded when no live assignment shares its due window.
	if value, ok := windows["1|2026-10-05"]; !ok || value {
		t.Fatalf("windows=%#v, want round 1 due 2026-10-05 present and false: no resolved assignment shares this window with statusId 5 (accepted)", windows)
	}
}

func TestClientRejectsUnsafeSubmissionPagination(t *testing.T) {
	client := newTestClient(t, "https://example.test/index.php")
	for _, count := range []int{-1, 0, 101} {
		if _, err := client.ListSubmissions(context.Background(), count); err == nil {
			t.Fatalf("count %d accepted", count)
		}
	}
	for _, path := range []string{
		"submissions?count=2&offset=-1",
		"submissions?count=2&offset=0&filter=x",
		"users?count=2&offset=-1",
		"users?count=2&offset=0&filter=x",
		"submissions/",
		"submissions/0",
		"submissions/-1",
		"submissions/abc",
		"submissions/600348?count=1",
	} {
		if _, err := client.restURL(path); err == nil {
			t.Fatalf("unsafe path accepted: %s", path)
		}
	}
	if _, err := client.restURL("users?count=2&offset=0"); err != nil {
		t.Fatalf("safe users path rejected: %v", err)
	}
	if _, err := client.restURL("submissions/600348"); err != nil {
		t.Fatalf("safe submission detail path rejected: %v", err)
	}
}

func TestValidateReportBodyAcceptsUTF8BOMBeforeQuotedHeader(t *testing.T) {
	body := []byte("\ufeff\"Submission ID\",Title\n600140,Example\n")
	if err := validateReportBody(ReportKindArticles, contentTypeCSV, body); err != nil {
		t.Fatalf("validate BOM-prefixed report: %v", err)
	}
}

type invalidResponseCase struct {
	name        string
	status      int
	contentType string
	body        string
	run         func(*Client) ([]byte, string, error)
}

func invalidDownloadAndRESTCases() []invalidResponseCase {
	return append(invalidReportDownloadCases(), invalidRESTCases()...)
}

func invalidReportDownloadCases() []invalidResponseCase {
	return []invalidResponseCase{
		{
			name:        "HTML report response",
			status:      http.StatusOK,
			contentType: "text/html",
			body:        "<html>login</html>",
			run: func(c *Client) ([]byte, string, error) {
				return c.DownloadReport(context.Background(), ReportKindArticles)
			},
		},
		{
			name:        "empty report response",
			status:      http.StatusOK,
			contentType: contentTypeCSV,
			body:        "",
			run: func(c *Client) ([]byte, string, error) {
				return c.DownloadReport(context.Background(), ReportKindReviews)
			},
		},
		{
			name:        "non-2xx report response",
			status:      http.StatusForbidden,
			contentType: contentTypeCSV,
			body:        "Submission ID,Title\n",
			run: func(c *Client) ([]byte, string, error) {
				return c.DownloadReport(context.Background(), ReportKindArticles)
			},
		},
		{
			name:        "CSV without expected header",
			status:      http.StatusOK,
			contentType: contentTypeCSV,
			body:        "wrong,header\n1,2\n",
			run: func(c *Client) ([]byte, string, error) {
				return c.DownloadReport(context.Background(), ReportKindViews)
			},
		},
	}
}

func invalidRESTCases() []invalidResponseCase {
	return []invalidResponseCase{
		{
			name:        "invalid REST JSON",
			status:      http.StatusOK,
			contentType: "application/json",
			body:        "{",
			run: func(c *Client) ([]byte, string, error) {
				var dst json.RawMessage
				return c.GetJSON(context.Background(), "submissions?count=1", &dst)
			},
		},
	}
}

func TestClientFailsClosedForInvalidDownloadsAndREST(t *testing.T) {
	for _, testCase := range invalidDownloadAndRESTCases() {
		t.Run(testCase.name, func(t *testing.T) {
			assertFailsClosedWithoutSecretLeak(t, testCase.contentType, testCase.status, testCase.body, testCase.run)
		})
	}
}

func assertFailsClosedWithoutSecretLeak(t *testing.T, contentType string, status int, body string, run func(*Client) ([]byte, string, error)) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.URL.Path == loginPath {
			http.SetCookie(resp, &http.Cookie{Name: sessionCookieKey, Value: sessionCookieVal, Path: "/"})
			_, _ = resp.Write([]byte("ok"))
			return
		}
		resp.Header().Set("Content-Type", contentType)
		resp.WriteHeader(status)
		_, _ = resp.Write([]byte(body))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/index.php")
	if err := client.Login(context.Background(), "editor@example.test", "top-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}

	responseBody, _, err := run(client)
	if err == nil {
		t.Fatal("expected fail-closed error")
	}
	if len(responseBody) != 0 {
		t.Fatalf("invalid response returned trusted body: %q", string(responseBody))
	}
	if strings.Contains(err.Error(), "top-secret") || strings.Contains(err.Error(), sessionCookieVal) {
		t.Fatalf("error leaked secret material: %v", err)
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()

	client, err := NewClient(baseURL, "jserd", time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func hasCookie(r *http.Request, name, value string) bool {
	for _, cookie := range r.Cookies() {
		if cookie.Name == name && cookie.Value == value {
			return true
		}
	}
	return false
}

// TestSubmissionDetailActiveReviewers counts the latest round's live
// assignments that still staff it. The detail query already omits declined
// assignments, so a decline never reaches this count. A cancelled (10),
// response-overdue (4), or declined (1) row does not count.
func TestSubmissionDetailActiveReviewers(t *testing.T) {
	detail := SubmissionDetail{
		ReviewRounds: []reviewRoundPayload{{Round: 1, StatusID: 11}, {Round: 2, StatusID: 6}},
		ReviewAssignments: []reviewAssignmentWindow{
			{StatusID: 5, Round: 1},
			{StatusID: 5, Round: 2},
			{StatusID: 0, Round: 2},
			{StatusID: 4, Round: 2},
			{StatusID: 10, Round: 2},
			{StatusID: 1, Round: 2},
		},
	}
	if got := detail.ActiveReviewers(); got != 2 {
		t.Fatalf("ActiveReviewers()=%d, want 2 (round 2: one accepted, one awaiting response)", got)
	}
}

// TestSubmissionDetailActiveReviewersEmptyLatestRound reports zero for a
// latest round with no live assignment, even when an older round had some.
func TestSubmissionDetailActiveReviewersEmptyLatestRound(t *testing.T) {
	detail := SubmissionDetail{
		ReviewRounds:      []reviewRoundPayload{{Round: 1}, {Round: 2}},
		ReviewAssignments: []reviewAssignmentWindow{{StatusID: 5, Round: 1}},
	}
	if got := detail.ActiveReviewers(); got != 0 {
		t.Fatalf("ActiveReviewers()=%d, want 0 for an unstaffed round 2", got)
	}
}
