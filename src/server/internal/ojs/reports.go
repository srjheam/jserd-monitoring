package ojs

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ReportKind string

const (
	ReportKindArticles ReportKind = "ArticleReportPlugin"
	ReportKindReviews  ReportKind = "ReviewReportPlugin"
	ReportKindViews    ReportKind = "ViewReportPlugin"
)

func (c *Client) DownloadReport(ctx context.Context, kind ReportKind) ([]byte, string, error) {
	reportURL, err := c.reportURL(kind)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reportURL, nil)
	if err != nil {
		return nil, "", err
	}
	body, err := c.readReport(req, kind)
	if err != nil {
		return nil, reportURL, err
	}
	return body, reportURL, nil
}

// reportURL is the address of one OJS report export.
func (c *Client) reportURL(kind ReportKind) (string, error) {
	if err := validateReportKind(kind); err != nil {
		return "", err
	}
	parsed, err := url.Parse(c.contextURL("management/tools/report"))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("pluginName", string(kind))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// readReport performs the request and returns the report body, once the answer
// passes every check that tells a report from an error page.
func (c *Client) readReport(req *http.Request, kind ReportKind) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download report %s failed", kind)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read report %s failed", kind)
	}
	if !okStatus(resp.StatusCode) {
		return nil, fmt.Errorf("download report %s failed with status %d", kind, resp.StatusCode)
	}
	return body, validateReportBody(kind, resp.Header.Get("Content-Type"), body)
}

func validateReportKind(kind ReportKind) error {
	switch kind {
	case ReportKindArticles, ReportKindReviews, ReportKindViews:
		return nil
	default:
		return fmt.Errorf("unsupported report kind %q", kind)
	}
}

func validateReportBody(kind ReportKind, contentType string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("report %s is empty", kind)
	}
	if isHTMLBody(contentType, body) {
		return fmt.Errorf("report %s returned HTML", kind)
	}
	seen, err := reportHeaderNames(body)
	if err != nil {
		return fmt.Errorf("report %s has invalid CSV header", kind)
	}
	return missingHeaderError(kind, seen)
}

// reportHeaderNames is the set of column names in the header row of a report.
func reportHeaderNames(body []byte) (map[string]bool, error) {
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for i, column := range header {
		if i == 0 {
			column = strings.TrimPrefix(column, "\ufeff")
		}
		seen[strings.TrimSpace(column)] = true
	}
	return seen, nil
}

// missingHeaderError names the first required column the report does not carry.
func missingHeaderError(kind ReportKind, seen map[string]bool) error {
	for _, required := range requiredHeaders(kind) {
		if !seen[required] {
			return fmt.Errorf("report %s missing required header %q", kind, required)
		}
	}
	return nil
}

func requiredHeaders(kind ReportKind) []string {
	switch kind {
	case ReportKindArticles:
		return []string{"Submission ID", "Title"}
	case ReportKindReviews:
		return []string{"Submission ID", "Reviewer"}
	case ReportKindViews:
		return []string{"Article ID", "Abstract Views"}
	default:
		return nil
	}
}
