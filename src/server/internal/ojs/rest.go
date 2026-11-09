package ojs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"server/internal/audit"
)

func (c *Client) GetJSON(ctx context.Context, path string, out any) ([]byte, string, error) {
	restURL, err := c.restURL(path)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restURL.String(), nil)
	if err != nil {
		return nil, "", err
	}
	source := audit.RedactURL(restURL.String())
	body, err := c.readJSON(req, out)
	if err != nil {
		return nil, source, err
	}
	return body, source, nil
}

// readJSON performs the request and returns the raw body, decoded into out when
// out is not nil.
func (c *Client) readJSON(req *http.Request, out any) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("REST GET failed")
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("REST read failed")
	}
	if err := validateRESTResponse(resp, body); err != nil {
		return nil, err
	}
	return body, decodeJSON(body, out)
}

// validateRESTResponse rejects an answer that is not the JSON the REST API owes.
func validateRESTResponse(resp *http.Response, body []byte) error {
	if err := validateRESTStatus(resp.StatusCode); err != nil {
		return err
	}
	return validateRESTBody(resp, body)
}

// validateRESTStatus rejects a status outside the success range.
func validateRESTStatus(status int) error {
	if !okStatus(status) {
		return fmt.Errorf("REST GET failed with status %d", status)
	}
	return nil
}

// validateRESTBody rejects an empty body, an HTML error page, and broken JSON.
func validateRESTBody(resp *http.Response, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("REST response is empty")
	}
	if isHTMLBody(resp.Header.Get("Content-Type"), body) {
		return fmt.Errorf("REST returned HTML")
	}
	if !json.Valid(body) {
		return fmt.Errorf("REST returned invalid JSON")
	}
	return nil
}

// decodeJSON fills out with the body, when the caller asked for a decode.
func decodeJSON(body []byte, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("REST decode failed")
	}
	return nil
}
