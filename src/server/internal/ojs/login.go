package ojs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxLoginResponseSize caps how much of OJS's sign-in answer this client reads.
const maxLoginResponseSize = 1 << 20

func (c *Client) Login(ctx context.Context, username, password string) error {
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.contextURL("login/signIn"), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := c.signIn(req); err != nil {
		return err
	}
	if len(c.httpClient.Jar.Cookies(c.baseURL)) == 0 {
		return fmt.Errorf("OJS login failed: session cookie missing")
	}
	return nil
}

// signIn posts the sign-in form and reads the answer. OJS answers a rejected
// sign-in with a normal page, so the body decides, not the status alone.
func (c *Client) signIn(req *http.Request) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("OJS login failed")
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponseSize))
	if err != nil {
		return fmt.Errorf("OJS login failed")
	}
	return signInResult(resp.StatusCode, body)
}

// signInResult reads what OJS answered the sign-in form with.
func signInResult(status int, body []byte) error {
	if status < 200 || status >= 300 {
		return fmt.Errorf("OJS login failed with status %d", status)
	}
	if looksLikeLoginFailure(body) {
		return fmt.Errorf("OJS login failed")
	}
	return nil
}

func looksLikeLoginFailure(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "login failed") || strings.Contains(text, "invalid username") || strings.Contains(text, "invalid password")
}
