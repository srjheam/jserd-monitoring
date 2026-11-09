package ojs

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL        *url.URL
	journalContext string
	httpClient     *http.Client
}

func NewClient(baseURL, journalContext string, timeout time.Duration) (*Client, error) {
	parsed, err := webURL(baseURL)
	if err != nil {
		return nil, err
	}
	context := strings.Trim(strings.TrimSpace(journalContext), "/")
	if err := validateClientSettings(context, timeout); err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &Client{
		baseURL:        parsed,
		journalContext: context,
		httpClient:     &http.Client{Jar: jar, Timeout: timeout},
	}, nil
}

// validateClientSettings rejects settings that cannot reach a journal.
func validateClientSettings(journalContext string, timeout time.Duration) error {
	if journalContext == "" {
		return fmt.Errorf("journalContext is required")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

// webURL parses an http or https address. Anything else is not an address this
// client can call.
func webURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || !isWebAddress(parsed) {
		return nil, fmt.Errorf("baseURL must be an http or https URL")
	}
	return parsed, nil
}

// isWebAddress reports whether a parsed URL names a host over http or https.
func isWebAddress(parsed *url.URL) bool {
	return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) contextURL(path string) string {
	u := *c.baseURL
	parts := []string{strings.TrimRight(u.Path, "/"), c.journalContext, strings.TrimLeft(path, "/")}
	u.Path = strings.Join(nonEmpty(parts), "/")
	return u.String()
}

func nonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
