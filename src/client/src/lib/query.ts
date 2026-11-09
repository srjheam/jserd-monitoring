import type { EditorFrequency, SideScreen } from "./monitoring-types";
import { type FolderSlug, folderSlugs } from "./wire";

// The search combobox's domain model (spec: .scratch/_archive/search-combobox
// once archived, see git history for .scratch/search-combobox/spec.md).
// Only `editor` and `folder` ship; a future tag is a registry addition.
export type TagKey = "editor" | "folder";

export interface Suggestion {
  tag: TagKey;
  value: string;
  label: string;
}

// `label` is frozen at commit time (the suggestion clicked/selected, or the
// raw wire text on a URL load) and never recomputed from `labelFor` on
// render: a locale change or a later frequency re-rank must not reword a
// chip the chief editor already placed.
export interface Chip {
  tag: TagKey;
  value: string;
  label: string;
}

export interface Query {
  chips: Chip[];
  text: string;
}

export const emptyQuery: Query = { chips: [], text: "" };

// `route` types as `SideScreen | undefined`, not spec.md's bare `SideScreen`:
// the rest of this app already models the Work route as `undefined` (see
// `pathForScreen`/`screenFromPath` in ./monitoring), and a combobox on the
// Work page needs to tell suggestions() that too.
export interface SuggestionContext {
  query: Query;
  typed: string;
  route: SideScreen | undefined;
  // Fetched by the combobox component (network, scoped by folder + committed
  // free text), not by the registry: `suggestions` must stay synchronous.
  editorFrequencies: EditorFrequency[];
}

// Returned by a committed chip's `onCommit`. `folder` is the only tag that
// carries one today: navigating routes is not something `suggestions` or
// `slugify` can express.
export type CommitEffect = { navigate: SideScreen | undefined } | null;

export interface TagTypeConfig {
  key: TagKey;
  exclusive: boolean;
  slugify: (raw: string) => string;
  labelFor: (value: string) => string;
  suggestions: (ctx: SuggestionContext) => Suggestion[];
  onCommit?: (value: string, query: Query) => CommitEffect;
}

export type ComboboxState =
  | { mode: "closed" }
  | { mode: "idle-open" }
  | { mode: "tag-open"; tag: TagKey; typed: string };

export type CursorPlacement = { kind: "input" } | { kind: "tag"; tag: TagKey };

export { folderSlugs };
export type { FolderSlug };

// Plain English, not translated: spec.md's `labelFor` signature carries no
// locale, so a chip's frozen label is domain text, the same way an OJS id
// is. The combobox's surrounding chrome (group headers, the free-text note)
// is translated separately, in the component.
const folderLabels: Record<FolderSlug, string> = {
  work: "Work",
  up_to_date: "Up to date",
  muted: "Muted",
  archived: "Archived",
};

export function folderLabel(slug: string): string {
  return folderLabels[slug as FolderSlug] ?? slug;
}

function isFolderSlug(value: string): value is FolderSlug {
  return (folderSlugs as readonly string[]).includes(value);
}

// editorSlug mirrors NameSlug in src/server/internal/normalizer/slug.go:
// lowercase, keep letters and decimal digits, treat space and hyphen as a
// separator, drop every other symbol, and collapse to single hyphens. Two raw
// names must produce the same slug on both sides (ADR-0006) or a chip built
// from an `editor:` suggestion would filter differently than one round-tripped
// through `?q`.
//
// `\p{Nd}`, not `\p{N}`: Go's unicode.IsDigit answers for the decimal-digit
// category alone, so a Roman numeral or a fraction is a symbol the server
// drops. tests/wire-grammar.test.ts holds the server's answer for both.
export function editorSlug(name: string): string {
  const lower = name.toLowerCase();
  let cleaned = "";
  for (const symbol of lower) {
    if (/[\p{L}\p{Nd}]/u.test(symbol)) cleaned += symbol;
    else if (symbol === " " || symbol === "-") cleaned += " ";
  }
  return cleaned.split(/\s+/).filter(Boolean).join("-");
}

// splitWireTokens mirrors splitSearchTokens: splits on whitespace, except
// inside double quotes, and drops the quotes it consumed.
function splitWireTokens(raw: string): string[] {
  const tokens: string[] = [];
  let current = "";
  let quoted = false;
  const flush = () => {
    if (current.length > 0) {
      tokens.push(current);
      current = "";
    }
  };
  for (const symbol of raw) {
    if (symbol === '"') quoted = !quoted;
    else if (!quoted && (symbol === " " || symbol === "\t" || symbol === "\n"))
      flush();
    else current += symbol;
  }
  flush();
  return tokens;
}

// toWireQuery writes the form the server reads: `editor:"<name>"` (the
// backend slug-merges on read, so the wire form still carries the raw
// display name, not the slug) and `folder:<slug>` (already a bare slug, no
// spaces, never quoted), followed by the free text.
export function toWireQuery(query: Query): string {
  const tokens = query.chips.map((chip) =>
    chip.tag === "editor" ? `editor:"${chip.label}"` : `folder:${chip.value}`,
  );
  return [...tokens, query.text.trim()].filter(Boolean).join(" ");
}

// parseWireQuery reads a `?q` value back into a Query. An unrecognized
// `folder:` slug is dropped rather than rejected: this runs on page load,
// not as a form submission, and a bad bookmark must not crash the page.
export function parseWireQuery(raw: string): Query {
  const chips: Chip[] = [];
  const words: string[] = [];
  for (const token of splitWireTokens(raw)) {
    const editorName = readPrefixed(token, "editor:");
    if (editorName !== null) {
      chips.push({
        tag: "editor",
        value: editorSlug(editorName),
        label: editorName,
      });
      continue;
    }
    const folderSlug = readPrefixed(token, "folder:");
    if (folderSlug !== null) {
      if (isFolderSlug(folderSlug)) keepLastFolder(chips, folderSlug);
      continue;
    }
    words.push(token);
  }
  return { chips, text: words.join(" ") };
}

// The last `folder:` token wins, and it keeps the place that token held. This
// is how searchFolder reads the same wire form on the server. Two folder tokens
// in one `?q` is a hand-edited URL: the combobox marks the tag exclusive, so it
// can never place a second one.
function keepLastFolder(chips: Chip[], slug: FolderSlug): void {
  const earlier = chips.findIndex((chip) => chip.tag === "folder");
  if (earlier >= 0) chips.splice(earlier, 1);
  chips.push({ tag: "folder", value: slug, label: folderLabel(slug) });
}

function readPrefixed(token: string, prefix: string): string | null {
  if (!token.startsWith(prefix) || token.length === prefix.length) return null;
  return token.slice(prefix.length);
}

// reconcileQuery implements spec.md's load-time rule. No `?q` prefills
// `folder:<route folder>` (the route folder is `work` on the Work route,
// where `screen` is undefined). `?q` present is trusted as-is, including a
// missing `folder:` token, which is genuine all-folder mode: nothing is
// forced in.
export function reconcileQuery(
  rawQ: string | null,
  screen: SideScreen | undefined,
): Query {
  if (!rawQ) {
    const slug: FolderSlug = screen ?? "work";
    return {
      chips: [{ tag: "folder", value: slug, label: folderLabel(slug) }],
      text: "",
    };
  }
  return parseWireQuery(rawQ);
}

// folderChipScreen reads the route a Query's folder chip implies, or
// undefined for the Work route (no chip, or an explicit `folder:work`).
// The caller compares this against the currently loaded route to decide
// whether a `folder:` token disagrees with it and must navigate (part B).
export function folderChipScreen(query: Query): SideScreen | undefined {
  const chip = query.chips.find((candidate) => candidate.tag === "folder");
  return chip && chip.value !== "work" ? (chip.value as SideScreen) : undefined;
}

// The frequency list arrives already ranked worst-to-best, but carries no
// server-side cap (it also feeds non-suggestion callers), so the dropdown
// caps it itself — the old SearchField capped at 5, kept a little roomier
// here since both the editor and folder groups now share the one list.
const maxEditorSuggestions = 8;

function editorSuggestions(ctx: SuggestionContext): Suggestion[] {
  const needle = ctx.typed.trim().toLowerCase();
  const existing = new Set(
    ctx.query.chips
      .filter((chip) => chip.tag === "editor")
      .map((chip) => chip.value),
  );
  return ctx.editorFrequencies
    .filter((editor) => !existing.has(editor.slug))
    .filter(
      (editor) => needle === "" || editor.label.toLowerCase().includes(needle),
    )
    .slice(0, maxEditorSuggestions)
    .map((editor) => ({
      tag: "editor" as const,
      value: editor.slug,
      label: editor.label,
    }));
}

function folderSuggestions(ctx: SuggestionContext): Suggestion[] {
  const needle = ctx.typed.trim().toLowerCase();
  const existing = new Set(
    ctx.query.chips
      .filter((chip) => chip.tag === "folder")
      .map((chip) => chip.value),
  );
  return folderSlugs
    .filter((slug) => !existing.has(slug))
    .filter(
      (slug) =>
        needle === "" || folderLabel(slug).toLowerCase().includes(needle),
    )
    .map((slug) => ({
      tag: "folder" as const,
      value: slug,
      label: folderLabel(slug),
    }));
}

export const tagRegistry: Record<TagKey, TagTypeConfig> = {
  editor: {
    key: "editor",
    exclusive: false,
    slugify: editorSlug,
    labelFor: (value) => value,
    suggestions: editorSuggestions,
  },
  folder: {
    key: "folder",
    exclusive: true,
    slugify: (raw) => raw,
    labelFor: folderLabel,
    suggestions: folderSuggestions,
    onCommit: (value) => ({
      navigate: value === "work" ? undefined : (value as SideScreen),
    }),
  },
};
