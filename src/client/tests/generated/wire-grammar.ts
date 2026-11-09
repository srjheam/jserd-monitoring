// Generated from src/server/internal/monitoring/search_terms.go and
// src/server/internal/normalizer/slug.go. Do not edit.
// Regenerate with:
//   go test ./internal/monitoring -run TestGeneratedGrammarCorpus -update
//
// The search wire grammar and the name slug need a real parser on each side,
// so the code cannot be generated. The answers can. tests/wire-grammar.test.ts
// runs the client parsers over this corpus and has to match every case.

export type SearchQueryCase = {
  raw: string;
  // The editor chips, as two parallel lists: the raw name the wire form
  // carries, and the slug the server filters on.
  chipNames: string[];
  chipSlugs: string[];
  folder: string;
  free: string;
};

export const searchQueryCases: SearchQueryCase[] = [
  {
    raw: "",
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "",
  },
  {
    raw: "plain free text",
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "plain free text",
  },
  {
    raw: "editor:Ana",
    chipNames: ["Ana"],
    chipSlugs: ["ana"],
    folder: "",
    free: "",
  },
  {
    raw: 'editor:"Ana Maria"',
    chipNames: ["Ana Maria"],
    chipSlugs: ["ana-maria"],
    folder: "",
    free: "",
  },
  {
    raw: 'editor:"Ana Maria" editor:Blake',
    chipNames: ["Ana Maria", "Blake"],
    chipSlugs: ["ana-maria", "blake"],
    folder: "",
    free: "",
  },
  {
    raw: "folder:muted",
    chipNames: [],
    chipSlugs: [],
    folder: "muted",
    free: "",
  },
  {
    raw: "folder:muted folder:archived",
    chipNames: [],
    chipSlugs: [],
    folder: "archived",
    free: "",
  },
  {
    raw: 'folder:work editor:"Ana Maria" code smell',
    chipNames: ["Ana Maria"],
    chipSlugs: ["ana-maria"],
    folder: "work",
    free: "code smell",
  },
  {
    raw: "editor:",
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "editor:",
  },
  {
    raw: "folder:",
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "folder:",
  },
  {
    raw: "one\ttwo\nthree",
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "one two three",
  },
  {
    raw: "spaced    out",
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "spaced out",
  },
  {
    raw: '"quoted words" tail',
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "quoted words tail",
  },
  {
    raw: 'unterminated "quote',
    chipNames: [],
    chipSlugs: [],
    folder: "",
    free: "unterminated quote",
  },
  {
    raw: 'tail words editor:"Ana Maria"',
    chipNames: ["Ana Maria"],
    chipSlugs: ["ana-maria"],
    folder: "",
    free: "tail words",
  },
];

export type NameSlugCase = { raw: string; slug: string };

export const nameSlugCases: NameSlugCase[] = [
  { raw: "", slug: "" },
  { raw: "Ana Maria", slug: "ana-maria" },
  { raw: "ANA MARIA", slug: "ana-maria" },
  { raw: "Ana D'Ávila", slug: "ana-dávila" },
  { raw: "Jean-Luc Picard", slug: "jean-luc-picard" },
  { raw: "  two   spaces  ", slug: "two-spaces" },
  { raw: "---", slug: "" },
  { raw: "José_Silva", slug: "josésilva" },
  { raw: "Reviewer 42", slug: "reviewer-42" },
  { raw: "Ⅷ Roman", slug: "roman" },
  { raw: "½ Half", slug: "half" },
];
