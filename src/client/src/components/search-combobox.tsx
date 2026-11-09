"use client";

import { Search, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useId, useRef, useState } from "react";
import { Button } from "react-aria-components";
import type { EditorFrequency, SideScreen } from "../lib/monitoring";
import {
  type Chip,
  type ComboboxState,
  type CursorPlacement,
  type Query,
  type Suggestion,
  type SuggestionContext,
  type TagKey,
  tagRegistry,
} from "../lib/query";

// react-aria-components ships a `ComboBox`, but it binds one flat option
// list to one text value and has no notion of a chip a chief editor can
// arrow into and re-edit. That interaction needs the raw ARIA combobox
// pattern (a single real `<input>`, `aria-activedescendant`, `role="option"`
// divs that never take focus themselves) instead, the same pattern the
// `SearchField` this replaces already used. `Button` is still the RAC one.
export function SearchCombobox({
  query,
  editorFrequencies,
  route,
  onQuery,
  onNavigate,
}: {
  query: Query;
  editorFrequencies: EditorFrequency[];
  route: SideScreen | undefined;
  onQuery: (query: Query) => void;
  onNavigate: (screen: SideScreen | undefined) => void;
}) {
  const t = useTranslations("workList");
  const listID = useId();
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const combo = useComboboxController({
    query,
    editorFrequencies,
    route,
    onQuery,
    onNavigate,
  });
  const { cursor } = combo;
  const activeZone = cursor.kind === "tag" ? cursor.tag : null;
  // The editor zone never edits a particular existing chip — a pick there
  // always adds a new one — so the input only splices in place of the one
  // chip being replaced for the folder zone, and otherwise trails every chip.
  const editedChipIndex =
    cursor.kind === "tag" && cursor.tag === "folder"
      ? chipIndexForTag(query.chips, cursor.tag)
      : -1;
  const beforeChips =
    editedChipIndex === -1
      ? query.chips
      : query.chips.slice(0, editedChipIndex);
  const afterChips =
    editedChipIndex === -1 ? [] : query.chips.slice(editedChipIndex + 1);
  const indexOf = (offset: number) =>
    editedChipIndex === -1 ? offset : editedChipIndex + offset;

  return (
    <div className="search-row">
      {/* biome-ignore lint/a11y/noStaticElementInteractions: relatedTarget check below is what closes the popover; there is no click behaviour here. */}
      <div
        className={
          activeZone === null ? "search" : "search search--zone-active"
        }
        ref={containerRef}
        onBlur={(event) => {
          if (containerRef.current?.contains(event.relatedTarget as Node))
            return;
          combo.close();
        }}
      >
        <div className="search__chips">
          {beforeChips.map((chip, index) => (
            <TagToken
              key={`${chip.tag}-${chip.value}-${index}`}
              chip={chip}
              t={t}
              active={chip.tag === activeZone}
              onEdit={
                chip.tag === "editor"
                  ? undefined
                  : () => {
                      combo.enterEdit(chip.tag);
                      inputRef.current?.focus();
                    }
              }
              onRemove={() => combo.deleteChip(index)}
            />
          ))}
          <label className="-screenReader" htmlFor={`${listID}-input`}>
            {t("searchLabel")}
          </label>
          <input
            id={`${listID}-input`}
            ref={inputRef}
            type="text"
            role="combobox"
            aria-expanded={combo.mode.mode !== "closed"}
            aria-controls={listID}
            aria-autocomplete="list"
            aria-activedescendant={
              combo.activeIndex !== null
                ? `${listID}-${combo.activeIndex}`
                : undefined
            }
            placeholder={t("searchPlaceholder")}
            value={combo.draft}
            onFocus={combo.focus}
            onChange={(event) => combo.setDraft(event.target.value)}
            onKeyDown={combo.handleKeyDown}
          />
          {afterChips.map((chip, index) => (
            <TagToken
              key={`${chip.tag}-${chip.value}-after-${index}`}
              chip={chip}
              t={t}
              active={chip.tag === activeZone}
              onEdit={
                chip.tag === "editor"
                  ? undefined
                  : () => {
                      combo.enterEdit(chip.tag);
                      inputRef.current?.focus();
                    }
              }
              onRemove={() => combo.deleteChip(indexOf(1 + index))}
            />
          ))}
        </div>
        <Button
          className="search__clear"
          aria-label={t("clear")}
          // Clear drops free text and every `editor:` chip but keeps the
          // `folder:` chip: it is the route's own identity, and clearing it
          // would silently flip the page into all-folder mode.
          onPress={() =>
            onQuery({
              chips: query.chips.filter((chip) => chip.tag === "folder"),
              text: "",
            })
          }
        >
          <X aria-hidden="true" size={16} />
        </Button>
        {combo.mode.mode !== "closed" && (
          <SuggestionList
            listID={listID}
            mode={combo.mode}
            flat={combo.flat}
            activeIndex={combo.activeIndex}
            t={t}
            onPick={(suggestion) => {
              combo.commitSuggestion(suggestion);
              inputRef.current?.focus();
            }}
          />
        )}
      </div>
      <Button
        className="search__submit"
        aria-label={t("runSearch")}
        onPress={() => {
          combo.commitOnEnter();
          inputRef.current?.blur();
        }}
      >
        <Search aria-hidden="true" size={16} />
      </Button>
    </div>
  );
}

function TagToken({
  chip,
  t,
  active,
  onEdit,
  onRemove,
}: {
  chip: Chip;
  t: ReturnType<typeof useTranslations>;
  active: boolean;
  onEdit: (() => void) | undefined;
  onRemove: () => void;
}) {
  const label = `${chip.tag}:${chip.label}`;
  return (
    <span className={active ? "tag-token tag-token--active" : "tag-token"}>
      {onEdit ? (
        <Button
          aria-label={t("editChip", { name: chip.label })}
          onPress={onEdit}
        >
          {label}
        </Button>
      ) : (
        <span>{label}</span>
      )}
      <Button
        aria-label={t("removeChip", { name: chip.label })}
        onPress={onRemove}
      >
        <X aria-hidden="true" size={14} />
      </Button>
    </span>
  );
}

function groupLabel(
  tag: TagKey,
  t: ReturnType<typeof useTranslations>,
): string {
  return tag === "editor" ? t("editorGroup") : t("folderGroup");
}

function SuggestionList({
  listID,
  mode,
  flat,
  activeIndex,
  t,
  onPick,
}: {
  listID: string;
  mode: ComboboxState;
  flat: Suggestion[];
  activeIndex: number | null;
  t: ReturnType<typeof useTranslations>;
  onPick: (suggestion: Suggestion) => void;
}) {
  const dimmed = mode.mode === "idle-open";
  return (
    <div
      className="search__suggestions"
      id={listID}
      role="listbox"
      aria-label={t("searchLabel")}
    >
      {flat.map((item, index) => (
        <div key={`${item.tag}-${item.value}`}>
          {dimmed && (index === 0 || flat[index - 1].tag !== item.tag) && (
            <div className="search__group-label" aria-hidden="true">
              {groupLabel(item.tag, t)}
            </div>
          )}
          {/* biome-ignore lint/a11y/useKeyWithClickEvents: the combobox input owns the keyboard, per the ARIA combobox pattern. */}
          <div
            id={`${listID}-${index}`}
            role="option"
            tabIndex={-1}
            aria-selected={index === activeIndex}
            className={dimmed ? "dimmed" : undefined}
            onClick={() => onPick(item)}
          >
            {item.label}
          </div>
        </div>
      ))}
      {dimmed && (
        <div className="search__note" aria-hidden="true">
          {t("freeTextNote")}
        </div>
      )}
    </div>
  );
}

function suggestionContextFor(
  query: Query,
  cursor: CursorPlacement,
  route: SideScreen | undefined,
  editorFrequencies: EditorFrequency[],
  typed: string,
): SuggestionContext {
  // The folder zone strips its own chip out first, so the current folder
  // reappears alongside the others instead of hiding itself as "already
  // selected" — that is what lets picking the same value re-confirm it. The
  // editor zone keeps every chip in: an editor already picked must stay
  // excluded, since picking there always adds a new one rather than
  // replacing what is already chosen.
  const activeQuery: Query =
    cursor.kind === "tag" && cursor.tag === "folder"
      ? {
          ...query,
          chips: query.chips.filter((chip) => chip.tag !== cursor.tag),
        }
      : query;
  return { query: activeQuery, typed, route, editorFrequencies };
}

function flatSuggestions(
  mode: ComboboxState,
  query: Query,
  cursor: CursorPlacement,
  route: SideScreen | undefined,
  editorFrequencies: EditorFrequency[],
): Suggestion[] {
  const ctx = (typed: string) =>
    suggestionContextFor(query, cursor, route, editorFrequencies, typed);
  if (mode.mode === "tag-open")
    return tagRegistry[mode.tag].suggestions(ctx(mode.typed)).slice(0, 4);
  if (mode.mode === "idle-open")
    return [
      ...tagRegistry.editor.suggestions(ctx("")),
      ...tagRegistry.folder.suggestions(ctx("")),
    ];
  return [];
}

// The three focus zones a chief editor can be in (spec.md's keyboard model,
// pressure-tested on branch prototype/search-combobox-keyboard-nav commit
// 8c6e9c6): the free-text field, the one editor tag, and the one folder tag.
// Up/Down cycle these in a full circle when no zone's suggestion list is
// interactive.
const zoneOrder: readonly (TagKey | "input")[] = ["input", "editor", "folder"];

function zoneOf(cursor: CursorPlacement): TagKey | "input" {
  return cursor.kind === "input" ? "input" : cursor.tag;
}

function cursorForZone(zone: TagKey | "input"): CursorPlacement {
  return zone === "input" ? { kind: "input" } : { kind: "tag", tag: zone };
}

function stepZone(zone: TagKey | "input", delta: 1 | -1): TagKey | "input" {
  const index = zoneOrder.indexOf(zone);
  return zoneOrder[(index + delta + zoneOrder.length) % zoneOrder.length];
}

// Query.chips carries no index for "the chip of this tag", so every lookup
// site shares this instead of re-deriving it. The folder tag has at most one
// chip, so "last" and "first" agree there; the editor tag can carry several,
// and the last one is the most recently added, which is what a fresh
// backspace in that zone should remove.
function chipIndexForTag(chips: Chip[], tag: TagKey): number {
  for (let i = chips.length - 1; i >= 0; i--) {
    if (chips[i].tag === tag) return i;
  }
  return -1;
}

// useComboboxController owns the whole interaction state machine described
// in spec.md: one committed `query` (the caller's), one uncommitted `draft`
// (the trailing free-text buffer, or the label of a chip mid-edit — never
// both at once), a `cursor` naming which of those the input currently is,
// and `activeIndex` over the flattened, currently visible suggestion list.
// `activeIndex !== null` while `cursor.kind === "tag"` is what the keyboard
// model calls a zone's list being "interactive"; the two never drift apart,
// so no separate boolean is kept for it.
function useComboboxController({
  query,
  editorFrequencies,
  route,
  onQuery,
  onNavigate,
}: {
  query: Query;
  editorFrequencies: EditorFrequency[];
  route: SideScreen | undefined;
  onQuery: (query: Query) => void;
  onNavigate: (screen: SideScreen | undefined) => void;
}) {
  const [focused, setFocused] = useState(false);
  const [cursor, setCursor] = useState<CursorPlacement>({ kind: "input" });
  const [draft, setDraft] = useState(query.text);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  // Set the instant Enter or the magnifier commits, so the suggestion list
  // hides even though the box stays focused. Any further typing or edit
  // reopens it (see setDraftAndReopen/enterEdit below).
  const [dismissed, setDismissed] = useState(false);

  // Resyncs the free-text buffer when the committed text changes from the
  // outside (mount, a URL load, another commit) — never while a chip is
  // mid-edit, and never while the chief editor is still typing (query.text
  // only changes on commit, so this cannot clobber a live keystroke).
  useEffect(() => {
    if (cursor.kind === "input") setDraft(query.text);
  }, [query.text, cursor.kind]);

  function setDraftAndReopen(value: string) {
    setDismissed(false);
    setDraft(value);
  }

  const mode: ComboboxState =
    !focused || dismissed
      ? { mode: "closed" }
      : cursor.kind === "tag"
        ? { mode: "tag-open", tag: cursor.tag, typed: draft }
        : { mode: "idle-open" };
  const flat = flatSuggestions(mode, query, cursor, route, editorFrequencies);

  // enterZone lands on a zone with no suggestion highlighted (spec.md: a
  // fresh Up/Down/Tab/Right press always starts a zone here, never straight
  // into an interactive list). The free-text zone's draft is the committed
  // text; an editor/folder zone reached this way starts with nothing typed,
  // since the keyboard model never types into it.
  function enterZone(zone: TagKey | "input") {
    setActiveIndex(null);
    setCursor(cursorForZone(zone));
    setDraft(zone === "input" ? query.text : "");
  }

  // enterEdit is the mouse path (clicking an existing tag token): it opens
  // that zone pre-filled with the chip's label, same non-interactive
  // starting point Tab/Right expects before making the list interactive.
  function enterEdit(tag: TagKey) {
    const chip = query.chips.find((candidate) => candidate.tag === tag);
    if (!chip) return;
    setDismissed(false);
    setCursor({ kind: "tag", tag });
    setDraft(chip.label);
    setActiveIndex(null);
  }

  function deleteChip(index: number) {
    const removed = query.chips[index];
    onQuery({ ...query, chips: query.chips.filter((_, i) => i !== index) });
    // Deleting the `folder:` chip is entering all-folder mode (spec.md),
    // which only renders on the Work route: a side screen must navigate
    // there, the same as committing a changed `folder:` chip does.
    if (removed?.tag === "folder" && route !== undefined) onNavigate(undefined);
    enterZone("input");
  }

  function commitChips(chips: Chip[], committed: Chip) {
    const nextQuery = { ...query, chips };
    onQuery(nextQuery);
    if (committed.tag !== "folder") return;
    const effect = tagRegistry.folder.onCommit?.(committed.value, nextQuery);
    if (effect && effect.navigate !== route) onNavigate(effect.navigate);
  }

  // commitSuggestion resolves an editor pick by appending, since a chief
  // editor can name several editors (query.ts already filters an editor
  // already picked out of the suggestion list, so no duplicate can land
  // here). The folder tag keeps spec.md's one-chip-per-tag rule: picking a
  // suggestion replaces its existing chip (or adds it), then returns focus
  // to the free-text field either way. The keyboard model (spec.md) commits
  // unconditionally the moment a zone's list is interactive, even if the
  // highlighted value repeats the existing folder chip — that can only
  // happen with `cursor.kind === "tag"`, so the "just move the cursor in"
  // shortcut below is scoped to the input zone's idle-open preview list (a
  // mouse-only path), where it never conflicts.
  function commitSuggestion(suggestion: Suggestion) {
    const chip: Chip = {
      tag: suggestion.tag,
      value: suggestion.value,
      label: suggestion.label,
    };
    if (suggestion.tag === "editor") {
      commitChips([...query.chips, chip], chip);
      enterZone("input");
      return;
    }
    const existingIndex = chipIndexForTag(query.chips, suggestion.tag);
    // A pick that only repeats the existing chip's value moves the cursor
    // into it instead of committing a no-op change.
    if (
      cursor.kind === "input" &&
      existingIndex !== -1 &&
      query.chips[existingIndex].value === suggestion.value
    ) {
      enterEdit(suggestion.tag);
      return;
    }
    const chips =
      existingIndex === -1
        ? [...query.chips, chip]
        : query.chips.map((existing, i) =>
            i === existingIndex ? chip : existing,
          );
    commitChips(chips, chip);
    enterZone("input");
  }

  function commitFreeText() {
    onQuery({ ...query, text: draft.trim() });
  }

  // Shared by Enter and the magnifier button (spec.md): commit whichever of
  // "the highlighted suggestion" or "the typed free text" applies, then
  // always dismiss the suggestion list — spec.md requires both to close any
  // open list, even mid chip-edit with nothing highlighted to commit.
  function commitOnEnter() {
    if (activeIndex !== null && flat[activeIndex]) {
      commitSuggestion(flat[activeIndex]);
    } else if (cursor.kind === "input") {
      commitFreeText();
    }
    setDismissed(true);
  }

  const handlers = keyHandlers({
    query,
    cursor,
    draft,
    flat,
    activeIndex,
    setActiveIndex,
    enterZone,
    deleteChip,
    commitSuggestion,
    commitOnEnter,
  });

  function handleKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    const input = event.currentTarget;
    const atStart = input.selectionStart === 0 && input.selectionEnd === 0;
    handlers(event, atStart);
  }

  function close() {
    if (cursor.kind === "tag") enterZone("input");
    setFocused(false);
  }

  return {
    mode,
    cursor,
    draft,
    setDraft: setDraftAndReopen,
    activeIndex,
    flat,
    focus: () => {
      setDismissed(false);
      setFocused(true);
    },
    close,
    enterEdit,
    deleteChip,
    commitSuggestion,
    commitOnEnter,
    handleKeyDown,
  };
}

// keyHandlers builds one dispatcher out of small, single-purpose handlers so
// the combined switch stays flat: every branch below is at most two `if`s
// deep, which is what keeps this under the cognitive-complexity cap instead
// of one long `handleKeyDown`.
function keyHandlers(deps: {
  query: Query;
  cursor: CursorPlacement;
  draft: string;
  flat: Suggestion[];
  activeIndex: number | null;
  setActiveIndex: (update: (current: number | null) => number | null) => void;
  enterZone: (zone: TagKey | "input") => void;
  deleteChip: (index: number) => void;
  commitSuggestion: (suggestion: Suggestion) => void;
  commitOnEnter: () => void;
}) {
  const { query, cursor, draft, flat, activeIndex } = deps;
  const interactive = cursor.kind === "tag" && activeIndex !== null;

  // Down always wraps within an interactive list and never exits it. Up
  // moves back and, once pressed past the first item, exits to zone-level
  // navigation. With no list interactive, both cycle the three zones.
  function vertical(event: React.KeyboardEvent, delta: 1 | -1) {
    event.preventDefault();
    if (!interactive) {
      deps.enterZone(stepZone(zoneOf(cursor), delta));
      return;
    }
    if (delta === 1) {
      deps.setActiveIndex((current) =>
        current === null || flat.length === 0
          ? current
          : (current + 1) % flat.length,
      );
    } else {
      deps.setActiveIndex((current) =>
        current !== null && current > 0 ? current - 1 : null,
      );
    }
  }

  // Left arrow and Shift+Tab share this: exit an interactive list instantly,
  // from any highlighted position. Neither does anything otherwise.
  function exitList(event: React.KeyboardEvent) {
    if (!interactive) return;
    event.preventDefault();
    deps.setActiveIndex(() => null);
  }

  // Right arrow and Tab share this too: on the free-text zone there is
  // nothing to expand, so Right is a no-op and Tab is left uncaptured
  // entirely, handing off to the page's own tab order. On editor/folder, a
  // non-interactive list becomes interactive; an interactive one commits
  // its highlighted suggestion and returns focus to the free-text field.
  function expandOrCommit(event: React.KeyboardEvent) {
    if (cursor.kind === "input") return;
    event.preventDefault();
    if (!interactive) {
      deps.setActiveIndex(() => 0);
      return;
    }
    const suggestion = activeIndex !== null ? flat[activeIndex] : undefined;
    if (suggestion) deps.commitSuggestion(suggestion);
  }

  function backspace(event: React.KeyboardEvent, atStart: boolean) {
    if (!atStart || draft !== "") return;
    if (cursor.kind === "input" && query.chips.length > 0) {
      event.preventDefault();
      deps.deleteChip(query.chips.length - 1);
      return;
    }
    if (cursor.kind === "tag") {
      const index = chipIndexForTag(query.chips, cursor.tag);
      if (index === -1) return;
      event.preventDefault();
      deps.deleteChip(index);
    }
  }

  return (event: React.KeyboardEvent<HTMLInputElement>, atStart: boolean) => {
    switch (event.key) {
      case "ArrowDown":
        return vertical(event, 1);
      case "ArrowUp":
        return vertical(event, -1);
      case "ArrowLeft":
        return exitList(event);
      case "ArrowRight":
        return expandOrCommit(event);
      case "Backspace":
        return backspace(event, atStart);
      case "Escape":
        if (cursor.kind === "tag") {
          event.preventDefault();
          deps.enterZone("input");
        }
        return;
      case "Tab":
        return event.shiftKey ? exitList(event) : expandOrCommit(event);
      case "Enter": {
        const commits =
          (activeIndex !== null && !!flat[activeIndex]) ||
          cursor.kind === "input";
        if (commits) event.preventDefault();
        deps.commitOnEnter();
        return;
      }
      default:
        return;
    }
  };
}
