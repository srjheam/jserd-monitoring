package editorial

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGoldenFlag = flag.Bool("update", false, "rewrite the generated finding-kinds file instead of checking it")

func updateGolden() bool { return *updateGoldenFlag }

// generatedKindsPath is the client constant this test owns. The client must not
// keep its own copy of the finding kinds or of their order: it used to, as
// findingRank in work-list.ts, and the two lists had nothing keeping them equal.
const generatedKindsPath = "../../../client/src/lib/finding-kinds.ts"

// TestGeneratedFindingKindsFileIsCurrent writes the catalog out as TypeScript
// and fails when the result differs from the file that is checked in. It is a
// test rather than a go:generate step so that it runs on every commit, with no
// tool to remember and no route to call.
//
// When it fails, run it with -update to rewrite the file:
//
//	go test ./internal/editorial -run TestGeneratedFindingKindsFile -update
func TestGeneratedFindingKindsFileIsCurrent(t *testing.T) {
	want := renderFindingKindsTS()
	if updateGolden() {
		writeGolden(t, want)
		return
	}
	got, err := os.ReadFile(generatedKindsPath)
	if err != nil {
		t.Fatalf("read %s: %v (run this test with -update to create it)", generatedKindsPath, err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of date with the catalog.\nRun: go test ./internal/editorial -run TestGeneratedFindingKindsFile -update", filepath.Clean(generatedKindsPath))
	}
}

func renderFindingKindsTS() string {
	var out strings.Builder
	out.WriteString("// Generated from src/server/internal/editorial/catalog.go. Do not edit.\n")
	out.WriteString("// Regenerate with:\n")
	out.WriteString("//   go test ./internal/editorial -run TestGeneratedFindingKindsFile -update\n")
	out.WriteString("//\n")
	out.WriteString("// The order is the rank, worst first. The client does not sort by it: the\n")
	out.WriteString("// server already ships each submission's findings in this order. The list\n")
	out.WriteString("// exists so a test can assert that every kind has a translated label.\n")
	out.WriteString("export const findingKinds = [\n")
	for _, entry := range catalog {
		out.WriteString("  \"" + string(entry.Kind) + "\",\n")
	}
	out.WriteString("] as const;\n\n")
	out.WriteString("export type FindingKind = (typeof findingKinds)[number];\n")
	return out.String()
}

func writeGolden(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(generatedKindsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", generatedKindsPath, err)
	}
	t.Logf("wrote %s", generatedKindsPath)
}
