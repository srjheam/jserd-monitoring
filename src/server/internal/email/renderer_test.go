package email

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewerTemplateGoldens(t *testing.T) {
	for _, rule := range []string{ruleRN6, ruleRN7, ruleRN8} {
		rendered, err := (Renderer{}).RenderReviewer(rule, reviewerData())
		if err != nil {
			t.Fatal(err)
		}
		got := rendered.Subject + "\n---TEXT---\n" + rendered.Text + "---HTML---\n" + rendered.HTML
		want, err := os.ReadFile(filepath.Join("testdata", strings.ToLower(rule)+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(got) != strings.TrimSpace(string(want)) {
			t.Fatalf("%s golden mismatch\ngot:\n%s\nwant:\n%s", rule, got, want)
		}
	}
}

func TestRN1TemplateRequiredElements(t *testing.T) {
	rendered, err := (Renderer{}).RenderEditor(ruleRN1, editorData())
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{rendered.Text, rendered.HTML} {
		for _, required := range []string{testEditorName, "42", testSubmissionTitle, "was answered by", testReviewerName, wantPleaseText, testSubmissionURL} {
			if !strings.Contains(body, required) {
				t.Fatalf("RN1 missing %q in %q", required, body)
			}
		}
	}
}

func TestRenderRN1Compatibility(t *testing.T) {
	rendered, err := (Renderer{}).RenderRN1(TemplateData{
		RecipientName: testEditorName,
		SubmissionID:  "42",
		Title:         testSubmissionTitle,
		OJSURL:        testSubmissionURL,
		ReviewerName:  testReviewerName,
	})
	if err != nil || rendered.Version != RN1TemplateVersion {
		t.Fatalf("rendered=%#v err=%v", rendered, err)
	}
}

func TestRendererCompilesRN1ToRN5(t *testing.T) {
	for _, rule := range []string{ruleRN1, ruleRN2, ruleRN3, ruleRN4, ruleRN5} {
		rendered, err := (Renderer{}).RenderEditor(rule, editorData())
		if err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
		if !strings.HasPrefix(rendered.Subject, "[JSERD]") || strings.Contains(rendered.Subject, rule) || rendered.Text == "" || rendered.HTML == "" {
			t.Fatalf("%s rendered=%#v", rule, rendered)
		}
	}
}

func TestEditorReplyToPolicy(t *testing.T) {
	got, err := EditorReplyTo("JSERD <editorial@example.test>")
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "editorial@example.test") {
		t.Fatalf("reply-to=%v err=%v", got, err)
	}
	omitted, err := EditorReplyTo("")
	if err != nil || len(omitted) != 0 {
		t.Fatalf("omitted=%v err=%v", omitted, err)
	}
	for _, invalid := range []string{invalidEmailValue, "ok@example.test\r\nBcc: attacker@example.test"} {
		if _, err := EditorReplyTo(invalid); err == nil {
			t.Fatalf("accepted invalid reply-to %q", invalid)
		}
	}
}

func TestRendererRejectsMissingRequiredData(t *testing.T) {
	fields := []func(*EditorTemplateData){
		func(data *EditorTemplateData) { data.RecipientName = "" },
		func(data *EditorTemplateData) { data.SubmissionID = "" },
		func(data *EditorTemplateData) { data.Title = "" },
		func(data *EditorTemplateData) { data.OJSURL = "" },
		func(data *EditorTemplateData) { data.ReviewerName = "" },
	}
	for i, clear := range fields {
		data := editorData()
		clear(&data)
		if _, err := (Renderer{}).RenderEditor(ruleRN1, data); err == nil {
			t.Fatalf("accepted missing field %d", i)
		}
	}
	data := editorData()
	data.Deadline = ""
	if _, err := (Renderer{}).RenderEditor(ruleRN2, data); err == nil {
		t.Fatal("RN2 accepted missing deadline")
	}
	data = editorData()
	data.ReviewerName = ""
	for _, rule := range []string{ruleRN4, ruleRN5} {
		if _, err := (Renderer{}).RenderEditor(rule, data); err != nil {
			t.Fatalf("%s incorrectly requires reviewer name: %v", rule, err)
		}
	}
}

func TestEditorTemplateGoldens(t *testing.T) {
	var got strings.Builder
	for _, rule := range []string{ruleRN2, ruleRN3, ruleRN4, ruleRN5} {
		rendered, err := (Renderer{}).RenderEditor(rule, editorData())
		if err != nil {
			t.Fatal(err)
		}
		got.WriteString("=== " + rule + " ===\n")
		got.WriteString(rendered.Subject + "\n---TEXT---\n" + rendered.Text + "---HTML---\n" + rendered.HTML + "\n")
	}
	want, err := os.ReadFile(filepath.Join("testdata", "editor_templates.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.String()) != strings.TrimSpace(string(want)) {
		t.Fatalf("editor golden mismatch\ngot:\n%s\nwant:\n%s", got.String(), want)
	}
}

func TestRendererEscapesUntrustedOJSData(t *testing.T) {
	data := editorData()
	data.Title = `<script>alert("x")</script>`
	rendered, err := (Renderer{}).RenderEditor(ruleRN5, data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.HTML, "<script>") || !strings.Contains(rendered.HTML, "&lt;script&gt;") {
		t.Fatalf("unsafe HTML: %s", rendered.HTML)
	}
	if !strings.Contains(rendered.Text, data.Title) {
		t.Fatalf("plain text changed title: %s", rendered.Text)
	}
	data.Title = "Título\r\nBcc: attacker@example.test"
	if _, err := (Renderer{}).RenderEditor(ruleRN3, data); err == nil {
		t.Fatal("subject CR/LF accepted")
	}
}

func TestAllTemplatesContainRequiredMessageElements(t *testing.T) {
	t.Run("editor templates", func(t *testing.T) {
		assertEditorTemplatesContainRequiredElements(t)
	})
	t.Run("reviewer templates", func(t *testing.T) {
		assertReviewerTemplatesContainRequiredElements(t)
	})
}

func assertEditorTemplatesContainRequiredElements(t *testing.T) {
	t.Helper()
	for _, rule := range []string{ruleRN1, ruleRN2, ruleRN3, ruleRN4, ruleRN5} {
		rendered, err := (Renderer{}).RenderEditor(rule, editorData())
		if err != nil {
			t.Fatal(err)
		}
		for _, body := range []string{rendered.Text, rendered.HTML} {
			for _, required := range []string{testEditorName, "42", testSubmissionTitle, wantPleaseText, testSubmissionURL} {
				if !strings.Contains(body, required) {
					t.Fatalf("%s missing %q in %q", rule, required, body)
				}
			}
			if (rule == ruleRN1 || rule == ruleRN2 || rule == ruleRN3) && !strings.Contains(body, testReviewerName) {
				t.Fatalf("%s missing reviewer name in %q", rule, body)
			}
			for _, forbidden := range []string{"Fato:", "Ação esperada:", "Prazo:"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("%s contains obsolete label %q in %q", rule, forbidden, body)
				}
			}
			if rule == ruleRN2 && !strings.Contains(body, "The deadline to answer the invite was") {
				t.Fatalf("RN2 missing naturalized deadline in %q", body)
			}
		}
	}
}

func assertReviewerTemplatesContainRequiredElements(t *testing.T) {
	t.Helper()
	for _, rule := range []string{ruleRN6, ruleRN7, ruleRN8} {
		rendered, err := (Renderer{}).RenderReviewer(rule, reviewerData())
		if err != nil {
			t.Fatal(err)
		}
		for _, body := range []string{rendered.Text, rendered.HTML} {
			for _, required := range []string{"Revisora", "42", testSubmissionTitle, fieldDeadline, testDeadlineValue, wantPleaseText, testSubmissionURL} {
				if !strings.Contains(body, required) {
					t.Fatalf("%s missing %q in %q", rule, required, body)
				}
			}
			for _, forbidden := range []string{"Fato:", "Ação esperada:", "Prazo:"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("%s contains obsolete label %q in %q", rule, forbidden, body)
				}
			}
		}
	}
}

func TestRendererCompilesRN6ToRN8(t *testing.T) {
	data := reviewerData()
	for _, rule := range []string{ruleRN6, ruleRN7, ruleRN8} {
		rendered, err := (Renderer{}).RenderReviewer(rule, data)
		if err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
		if !strings.HasPrefix(rendered.Subject, "[JSERD]") || strings.Contains(rendered.Subject, rule) || rendered.Text == "" || rendered.HTML == "" {
			t.Fatalf("%s rendered=%#v", rule, rendered)
		}
	}
	malicious := data
	malicious.Title = "Título\r\nBcc: attacker@example.test"
	if _, err := (Renderer{}).RenderReviewer(ruleRN6, malicious); err == nil {
		t.Fatal("subject CR/LF accepted")
	}
	malicious = data
	malicious.Title = `<script>alert("x")</script>`
	rendered, err := (Renderer{}).RenderReviewer(ruleRN7, malicious)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.HTML, "<script>") {
		t.Fatal("HTML markup was not escaped")
	}
}

func TestReviewerReplyToPolicy(t *testing.T) {
	got, err := ReviewerReplyTo([]EditorContact{{Name: "Editora A", Email: "a@example.test"}, {Name: "Inválida", Email: invalidEmailValue}, {Name: "Editor B", Email: "b@example.test"}}, "fallback@example.test")
	if err != nil || len(got) != 2 || !strings.Contains(got[0], "a@example.test") || !strings.Contains(got[1], "b@example.test") {
		t.Fatalf("reply-to=%v err=%v", got, err)
	}
	fallback, err := ReviewerReplyTo([]EditorContact{{Email: invalidEmailValue}}, "fallback@example.test")
	if err != nil || len(fallback) != 1 {
		t.Fatalf("fallback=%v err=%v", fallback, err)
	}
	omitted, err := ReviewerReplyTo(nil, "")
	if err != nil || len(omitted) != 0 {
		t.Fatalf("omitted=%v err=%v", omitted, err)
	}
	if _, err := ReviewerReplyTo([]EditorContact{{Email: "ok@example.test\r\nBcc:x@example.test"}}, ""); err == nil {
		t.Fatal("CR/LF accepted")
	}
}

func TestReviewerTemplatesRequireDeadline(t *testing.T) {
	data := reviewerData()
	data.Deadline = ""
	for _, rule := range []string{ruleRN6, ruleRN7, ruleRN8} {
		if _, err := (Renderer{}).RenderReviewer(rule, data); err == nil {
			t.Fatalf("%s accepted missing deadline", rule)
		}
	}
}

func TestReviewerTemplatesRequireEveryField(t *testing.T) {
	fields := []struct {
		name  string
		clear func(*ReviewerTemplateData)
	}{
		{name: "recipient", clear: func(data *ReviewerTemplateData) { data.RecipientName = " " }},
		{name: "submission", clear: func(data *ReviewerTemplateData) { data.SubmissionID = " " }},
		{name: fieldTitle, clear: func(data *ReviewerTemplateData) { data.Title = " " }},
		{name: fieldDeadline, clear: func(data *ReviewerTemplateData) { data.Deadline = " " }},
		{name: "url", clear: func(data *ReviewerTemplateData) { data.OJSURL = " " }},
	}
	for _, rule := range []string{ruleRN6, ruleRN7, ruleRN8} {
		for _, field := range fields {
			t.Run(rule+"/"+field.name, func(t *testing.T) {
				data := reviewerData()
				field.clear(&data)
				if _, err := (Renderer{}).RenderReviewer(rule, data); err == nil {
					t.Fatalf("%s accepted missing %s", rule, field.name)
				}
			})
		}
	}
}

func TestWelcomeTemplateRequiredElements(t *testing.T) {
	rendered, err := (Renderer{}).RenderWelcome(WelcomeTemplateData{RecipientName: testEditorName})
	if err != nil || rendered.Version != WelcomeTemplateVersion {
		t.Fatalf("rendered=%#v err=%v", rendered, err)
	}
	for _, body := range []string{rendered.Text, rendered.HTML} {
		if !strings.Contains(body, testEditorName) {
			t.Fatalf("welcome message missing recipient name: %q", body)
		}
	}
	if _, err := (Renderer{}).RenderWelcome(WelcomeTemplateData{RecipientName: " "}); err == nil {
		t.Fatal("accepted blank recipient name")
	}
}

func reviewerData() ReviewerTemplateData {
	return ReviewerTemplateData{RecipientName: "Revisora", SubmissionID: "42", Title: testSubmissionTitle, Deadline: testDeadlineValue, OJSURL: testSubmissionURL}
}

func editorData() EditorTemplateData {
	return EditorTemplateData{RecipientName: testEditorName, SubmissionID: "42", Title: testSubmissionTitle, Deadline: testDeadlineValue, OJSURL: testSubmissionURL, ReviewerName: testReviewerName}
}
