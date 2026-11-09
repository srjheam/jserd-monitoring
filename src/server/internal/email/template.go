package email

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

//go:embed templates/*.tmpl
var templates embed.FS

func renderTemplate(name, version string, data any) (Rendered, error) {
	source, err := templates.ReadFile("templates/" + name + ".tmpl")
	if err != nil {
		return Rendered{}, err
	}
	parts, err := renderParts(name, source, data)
	if err != nil {
		return Rendered{}, err
	}
	hash := sha256.Sum256(source)
	return Rendered{Subject: parts.subject, Text: parts.text, HTML: parts.html, Version: version, Hash: hex.EncodeToString(hash[:])}, nil
}

// renderedParts are the three rendered forms of one e-mail.
type renderedParts struct {
	subject, text, html string
}

// renderParts renders the subject, the plain text body and the HTML body.
func renderParts(name string, source []byte, data any) (renderedParts, error) {
	subject, err := renderSubject(name, source, data)
	if err != nil {
		return renderedParts{}, err
	}
	text, err := renderTextBlock(name, source, "text", data)
	if err != nil {
		return renderedParts{}, fmt.Errorf("render text: %w", err)
	}
	html, err := renderHTMLBlock(name, source, data)
	if err != nil {
		return renderedParts{}, fmt.Errorf("render html: %w", err)
	}
	return renderedParts{subject: subject, text: text, html: html}, nil
}

// renderSubject renders the subject line. The subject must hold no line break,
// because a line break in a subject header injects a header.
func renderSubject(name string, source []byte, data any) (string, error) {
	subject, err := renderTextBlock(name, source, "subject", data)
	if err != nil {
		return "", fmt.Errorf("render subject: %w", err)
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", fmt.Errorf("subject contains CR/LF")
	}
	return subject, nil
}

// renderTextBlock renders one named block of a template as plain text.
func renderTextBlock(name string, source []byte, block string, data any) (string, error) {
	tt, err := texttemplate.New(name).Option("missingkey=error").Parse(string(source))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tt.ExecuteTemplate(&out, block, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// renderHTMLBlock renders the html block of a template, with HTML escaping.
func renderHTMLBlock(name string, source []byte, data any) (string, error) {
	ht, err := htmltemplate.New(name).Option("missingkey=error").Parse(string(source))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := ht.ExecuteTemplate(&out, "html", data); err != nil {
		return "", err
	}
	return out.String(), nil
}
