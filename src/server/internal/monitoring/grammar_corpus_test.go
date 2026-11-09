package monitoring

import (
	"fmt"
	"os"
	"path/filepath"
	"server/internal/normalizer"
	"strings"
	"testing"
)

// generatedCorpusPath is the client fixture this test owns. The search wire
// grammar and the name slug cannot be generated into TypeScript, because each
// side needs a real parser in its own language. What can be generated is the
// answer: the client runs its parser over this corpus and has to agree with the
// server on every case.
const generatedCorpusPath = "../../../client/tests/generated/wire-grammar.ts"

// grammarCorpus is the one list of inputs both parsers must agree on. Add a
// case here and the client test picks it up on the next run.
//
// Every folder slug used here is one the client also accepts. The server takes
// a wider set (`all`, `active`, `worklist`), which the client drops instead;
// that divergence is outside what this corpus tests.
var grammarCorpus = []string{
	"",
	"plain free text",
	"editor:Ana",
	`editor:"Ana Maria"`,
	`editor:"Ana Maria" editor:Blake`,
	"folder:muted",
	"folder:muted folder:archived",
	`folder:work editor:"Ana Maria" code smell`,
	"editor:",
	"folder:",
	"one\ttwo\nthree",
	"spaced    out",
	`"quoted words" tail`,
	`unterminated "quote`,
	`tail words editor:"Ana Maria"`,
}

// slugCorpus is the one list of names both sluggers must agree on. The last
// entries carry Unicode numbers that are not decimal digits, which is the exact
// point where a Go `unicode.IsDigit` test and a TypeScript `\p{N}` test part.
var slugCorpus = []string{
	"",
	"Ana Maria",
	"ANA MARIA",
	"Ana D'Ávila",
	"Jean-Luc Picard",
	"  two   spaces  ",
	"---",
	"José_Silva",
	"Reviewer 42",
	"Ⅷ Roman",
	"½ Half",
}

// TestGeneratedGrammarCorpusIsCurrent writes the corpus answers out as
// TypeScript and fails when the result differs from the file that is checked
// in. Rewrite the file with:
//
//	go test ./internal/monitoring -run TestGeneratedGrammarCorpus -update
func TestGeneratedGrammarCorpusIsCurrent(t *testing.T) {
	want := renderGrammarCorpusTS(t)
	if *updateWireFlag {
		if err := os.MkdirAll(filepath.Dir(generatedCorpusPath), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(generatedCorpusPath), err)
		}
		if err := os.WriteFile(generatedCorpusPath, []byte(want), 0o644); err != nil {
			t.Fatalf("write %s: %v", generatedCorpusPath, err)
		}
		t.Logf("wrote %s", generatedCorpusPath)
		return
	}
	got, err := os.ReadFile(generatedCorpusPath)
	if err != nil {
		t.Fatalf("read %s: %v (run this test with -update to create it)", generatedCorpusPath, err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of date with the server parsers.\nRun: go test ./internal/monitoring -run TestGeneratedGrammarCorpus -update", filepath.Clean(generatedCorpusPath))
	}
}

func renderGrammarCorpusTS(t *testing.T) string {
	t.Helper()
	var out strings.Builder
	out.WriteString("// Generated from src/server/internal/monitoring/search_terms.go and\n")
	out.WriteString("// src/server/internal/normalizer/slug.go. Do not edit.\n")
	out.WriteString("// Regenerate with:\n")
	out.WriteString("//   go test ./internal/monitoring -run TestGeneratedGrammarCorpus -update\n")
	out.WriteString("//\n")
	out.WriteString("// The search wire grammar and the name slug need a real parser on each side,\n")
	out.WriteString("// so the code cannot be generated. The answers can. tests/wire-grammar.test.ts\n")
	out.WriteString("// runs the client parsers over this corpus and has to match every case.\n\n")

	out.WriteString("export type SearchQueryCase = {\n")
	out.WriteString("  raw: string;\n")
	out.WriteString("  // The editor chips, as two parallel lists: the raw name the wire form\n")
	out.WriteString("  // carries, and the slug the server filters on.\n")
	out.WriteString("  chipNames: string[];\n")
	out.WriteString("  chipSlugs: string[];\n")
	out.WriteString("  folder: string;\n")
	out.WriteString("  free: string;\n")
	out.WriteString("};\n\n")
	out.WriteString("export const searchQueryCases: SearchQueryCase[] = [\n")
	for _, raw := range grammarCorpus {
		out.WriteString(renderSearchQueryCase(t, raw))
	}
	out.WriteString("];\n\n")

	out.WriteString("export type NameSlugCase = { raw: string; slug: string };\n\n")
	out.WriteString("export const nameSlugCases: NameSlugCase[] = [\n")
	for _, raw := range slugCorpus {
		fmt.Fprintf(&out, "  { raw: %s, slug: %s },\n", tsString(raw), tsString(normalizer.NameSlug(raw)))
	}
	out.WriteString("];\n")
	return out.String()
}

func renderSearchQueryCase(t *testing.T, raw string) string {
	t.Helper()
	chips, folder, free, err := parseSearchQuery(raw)
	if err != nil {
		t.Fatalf("corpus case %q is rejected by the server: %v", raw, err)
	}
	names := make([]string, 0, len(chips))
	slugs := make([]string, 0, len(chips))
	for _, name := range chips {
		names = append(names, tsString(name))
		slugs = append(slugs, tsString(normalizer.NameSlug(name)))
	}
	var out strings.Builder
	out.WriteString("  {\n")
	fmt.Fprintf(&out, "    raw: %s,\n", tsString(raw))
	fmt.Fprintf(&out, "    chipNames: [%s],\n", strings.Join(names, ", "))
	fmt.Fprintf(&out, "    chipSlugs: [%s],\n", strings.Join(slugs, ", "))
	fmt.Fprintf(&out, "    folder: %s,\n", tsString(folder))
	fmt.Fprintf(&out, "    free: %s,\n", tsString(free))
	out.WriteString("  },\n")
	return out.String()
}

// tsString quotes a value the way biome formats a string literal: double
// quotes, unless the value carries a double quote and no single quote, where
// biome switches rather than escape.
func tsString(value string) string {
	quote := `"`
	if strings.Contains(value, `"`) && !strings.Contains(value, "'") {
		quote = "'"
	}
	escaped := strings.NewReplacer(
		`\`, `\\`,
		"\t", `\t`,
		"\n", `\n`,
		quote, `\`+quote,
	).Replace(value)
	return quote + escaped + quote
}
