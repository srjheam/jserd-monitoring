package ojs

import (
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) restURL(path string) (*url.URL, error) {
	parsed, err := relativeRESTPath(path)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(c.contextURL("api/v1/" + parsed.Path))
	if err != nil {
		return nil, err
	}
	u.RawQuery = parsed.RawQuery
	return u, nil
}

// relativeRESTPath parses a REST path and rejects anything that is not a
// relative path this client is allowed to call.
func relativeRESTPath(path string) (*url.URL, error) {
	trimmed := strings.TrimLeft(strings.TrimSpace(path), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" {
		return nil, fmt.Errorf("REST path must be relative")
	}
	return parsed, validateRESTPath(parsed)
}
