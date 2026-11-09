package editorial

import (
	"strings"
	"testing"
)

func TestRendererCollectiveSummary(t *testing.T) {
	articles := []ArticleFinding{
		{SubmissionID: "1", Title: "Primeiro", URL: "https://ojs.example.test/1", Findings: []Finding{{Kind: FindingMissingReviewer, Responsible: targetKindEditor}}},
		{SubmissionID: "2", Findings: []Finding{{Kind: FindingMissingRevisionIndication, Responsible: "author", Deadline: "2026-07-01", Contact: "author@example.test"}}},
	}
	got, err := (Renderer{}).Render("24/06/2026 a 01/07/2026", articles)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "[JSERD] Editorial summary — 24/06/2026 a 01/07/2026" {
		t.Fatalf("subject=%q", got.Subject)
	}
	for _, body := range []string{got.Text, got.HTML} {
		for _, want := range []string{"Dear editors-in-chief", "Primeiro", "Submission 2", "sign", "Suggested owner: author", "Deadline: 2026-07-01", "Contact: author@example.test"} {
			if !strings.Contains(body, want) {
				t.Fatalf("missing %q in %q", want, body)
			}
		}
		for _, code := range []string{"RN9", "RN10", "RN11", "RN12", "RN13"} {
			if strings.Contains(body, code) {
				t.Fatalf("leaked %s", code)
			}
		}
	}
}

func TestRendererRejectsHeaderInjection(t *testing.T) {
	_, err := (Renderer{}).Render("período\r\nBcc: x@example.test", []ArticleFinding{{SubmissionID: "1", Findings: []Finding{{Kind: FindingMissingEditor}}}})
	if err == nil {
		t.Fatal("accepted CR/LF")
	}
}
