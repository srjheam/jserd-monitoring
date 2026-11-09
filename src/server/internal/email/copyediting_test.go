package email

import (
	"strings"
	"testing"
)

func TestRN18RequestsACopyeditorAssignment(t *testing.T) {
	rendered, err := (Renderer{}).RenderEditor("RN18", editorData())
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{rendered.Text, rendered.HTML} {
		for _, required := range []string{testEditorName, testSubmissionTitle, testSubmissionURL, "has no copyeditor assigned", "Please assign a copyeditor"} {
			if !strings.Contains(body, required) {
				t.Fatalf("missing %q in %q", required, body)
			}
		}
	}
	if rendered.Version != "rn18:v1" || strings.Contains(rendered.Subject, "RN18") {
		t.Fatalf("rendered=%#v", rendered)
	}
}
