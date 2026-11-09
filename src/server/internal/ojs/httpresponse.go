package ojs

import "strings"

// okStatus reports whether an HTTP status is in the success range.
func okStatus(status int) bool {
	return status >= 200 && status < 300
}

// isHTMLBody reports whether an answer is a web page instead of the data the
// request asked for.
func isHTMLBody(contentType string, body []byte) bool {
	return strings.Contains(strings.ToLower(contentType), "html") || looksLikeHTML(body)
}

func looksLikeHTML(body []byte) bool {
	text := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(text, "<!doctype html") || strings.Contains(text, "<html")
}
