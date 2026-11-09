// EditorFrequency and SideScreen live here, not in monitoring.ts, because
// query.ts needs both and monitoring.ts imports query.ts. A module that
// imports neither breaks that cycle without changing either type's shape.

// One assigned-editor slug's submission count (ADR-0006 merges raw names
// that collide on the same slug). Ranks the search combobox's `editor:`
// suggestions.
export type EditorFrequency = { slug: string; label: string; count: number };

// The side screens hold what a chief editor chose not to watch, plus the
// submissions the workflow closed on its own. Muted is delegation, Up to
// date is a statement about the present state of the work, and Archived is a
// closed submission kept only for the record, so they never share a screen
// or a name. The members come from the server's queue list (./wire).
export type { SideScreen } from "./wire";
