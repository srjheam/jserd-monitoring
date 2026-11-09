package editorial

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
var summaryTemplates embed.FS

const SummaryTemplateVersion = "editorial-summary:v1"

type SummaryData struct {
	Period   string
	Articles []SummaryArticle
}

type SummaryArticle struct {
	Label, URL string
	Bullets    []string
}

type RenderedSummary struct {
	Subject, Text, HTML, Version, Hash string
}

type Renderer struct{}

func (Renderer) Render(period string, articles []ArticleFinding) (RenderedSummary, error) {
	if err := validateSummaryInput(period, articles); err != nil {
		return RenderedSummary{}, err
	}
	data := summaryData(period, articles)
	source, err := summaryTemplates.ReadFile("templates/summary.tmpl")
	if err != nil {
		return RenderedSummary{}, err
	}
	parts, err := renderSummaryParts(source, data)
	if err != nil {
		return RenderedSummary{}, err
	}
	hash := sha256.Sum256(source)
	return RenderedSummary{Subject: parts.subject, Text: parts.text, HTML: parts.html, Version: SummaryTemplateVersion, Hash: hex.EncodeToString(hash[:])}, nil
}

// validateSummaryInput rejects a summary that names no period, or that lists no
// article. An e-mail with nothing to report must not be sent.
func validateSummaryInput(period string, articles []ArticleFinding) error {
	if strings.TrimSpace(period) == "" || len(articles) == 0 {
		return fmt.Errorf("period and articles are required")
	}
	return nil
}

// summaryData is the article list the summary template reads.
func summaryData(period string, articles []ArticleFinding) SummaryData {
	data := SummaryData{Period: strings.TrimSpace(period), Articles: make([]SummaryArticle, 0, len(articles))}
	for _, article := range articles {
		data.Articles = append(data.Articles, summaryArticle(article))
	}
	return data
}

// summaryArticle is one article of the summary. An article with no title is
// labelled by its submission id.
func summaryArticle(article ArticleFinding) SummaryArticle {
	label := strings.TrimSpace(article.Title)
	if label == "" {
		label = "Submission " + article.SubmissionID
	}
	item := SummaryArticle{Label: label, URL: article.URL}
	for _, finding := range article.Findings {
		item.Bullets = append(item.Bullets, findingText(finding))
	}
	return item
}

// summaryParts are the three rendered forms of one summary e-mail.
type summaryParts struct {
	subject, text, html string
}

// renderSummaryParts renders the subject, the plain text body and the HTML
// body. The subject must hold no line break, because a line break in a subject
// header injects a header.
func renderSummaryParts(source []byte, data SummaryData) (summaryParts, error) {
	subject, err := renderText(source, "subject", data)
	if err != nil {
		return summaryParts{}, err
	}
	if strings.ContainsAny(subject, "\r\n") {
		return summaryParts{}, fmt.Errorf("subject contains CR/LF")
	}
	text, err := renderText(source, "text", data)
	if err != nil {
		return summaryParts{}, err
	}
	html, err := renderHTML(source, data)
	return summaryParts{subject: subject, text: text, html: html}, err
}

// renderText renders one named block of the template as plain text.
func renderText(source []byte, name string, data SummaryData) (string, error) {
	tt, err := texttemplate.New("summary").Option("missingkey=error").Parse(string(source))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tt.ExecuteTemplate(&out, name, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// renderHTML renders the html block of the template, with HTML escaping.
func renderHTML(source []byte, data SummaryData) (string, error) {
	ht, err := htmltemplate.New("summary").Option("missingkey=error").Parse(string(source))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := ht.ExecuteTemplate(&out, "html", data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func findingText(finding Finding) string {
	parts := []string{emailSentence(finding.Kind)}
	if finding.Responsible != "" {
		parts = append(parts, "Suggested owner: "+finding.Responsible+".")
	}
	if finding.Deadline != "" {
		parts = append(parts, "Deadline: "+finding.Deadline+".")
	}
	if finding.Contact != "" {
		parts = append(parts, "Contact: "+finding.Contact+".")
	}
	return strings.Join(parts, " ")
}
