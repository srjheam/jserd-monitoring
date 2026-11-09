export type { EditorFrequency, SideScreen } from "./monitoring-types";

import type { EditorFrequency, SideScreen } from "./monitoring-types";
import { type Query, reconcileQuery, toWireQuery } from "./query";
import { defaultPageLimit, maxPageLimit, type Queue } from "./wire";

export type { Queue };

export type Conference = { id: string; name: string; description: string };
export type Tracking = {
  id: string;
  conference: string;
  muted: boolean;
  // The finding digest stamped when the mute was set. The Muted screen
  // compares it against the live findingDigest to mark a finding that
  // appeared afterwards.
  mutedDigest: string;
  upToDateAt: string;
  upToDateUntil: string;
  created: string;
  updated: string;
};
// One row of the submission activity timeline, as served by
// GET /api/jserd/monitoring/submissions/{id}/activity: either a delivery
// attempt or an append-only journal entry, newest first.
export type ActivityItem = {
  kind: "delivery" | "journal";
  occurredAt: string;
  titleKey?: string;
  status?: string;
  failureClass?: string;
  recipient?: string;
  recipientRole?:
    | "journal_editor"
    | "associate_editor"
    | "editor"
    | "reviewer"
    | "";
  authorLabel?: string;
  body?: string;
};
export type Submission = {
  ojsSubmissionId: string;
  title: string;
  status: string;
  url: string;
  dateSubmitted: string;
  lastModified: string;
  // OJS's own last-activity timestamp. It moves when a reviewer completes a
  // review; lastModified does not.
  dateLastActivity?: string;
  editors: string[];
  decisions: { decision: string; date: string }[];
  reviews: {
    round: number;
    status: string;
    responseDeadline: string;
    reviewDeadline: string;
  }[];
  findings: { kind: string; responsible: string; deadline: string }[];
  tracking?: Tracking;
  effectiveFinalState: string;
  effectiveQueue: Queue;
  effectiveAttention: string;
  // Stable hash of the current finding kinds. Marking a submission up to date
  // stores it, so the marking ends by itself once the findings change.
  findingDigest: string;
  provenance: Record<string, string>;
};
export type MonitoringAggregates = {
  metrics: Record<string, number>;
  // One count per queue, keyed by queue name. The server fills it from its own
  // queue list, so a queue added there arrives here with no type change. A key
  // the server has not sent yet reads as undefined, so every caller defaults.
  triage: Partial<Record<Queue, number>>;
  // Not a queue, so it sits beside triage rather than inside it.
  dailyRequiringAttention: number;
  statuses: string[];
  editors: EditorFrequency[];
  total: number;
};
export type MonitoringMeta = {
  generatedAt: string;
  availability: "available" | "stale" | "empty" | "unavailable";
  latestAttempt?: {
    id: string;
    source: string;
    status: string;
    finishedAt: string;
    failure?: string;
  };
  latestSuccess?: {
    id: string;
    source: string;
    status: string;
    finishedAt: string;
  };
  conferences: Conference[];
  aggregates: MonitoringAggregates;
};
export type SubmissionPage = {
  items: Submission[];
  nextCursor: string;
  total: number;
};

// `limit` can be read from an incoming URL (a deep link or a test fixture),
// but is never written back: it stays out of the address bar the user sees.
// A reload always restarts the keyset window from the first batch. The two
// numbers come from the server's listquery.go, through the generated ./wire.
export { defaultPageLimit, maxPageLimit };

// Each side screen is its own route rather than a query param, so its URL is
// a real destination a chief editor can bookmark or open in a new tab.
const screenSlugs: Record<SideScreen, string> = {
  muted: "muted",
  up_to_date: "up-to-date",
  archived: "archived",
};

export function pathForScreen(screen?: SideScreen): string {
  return screen ? `/${screenSlugs[screen]}` : "/";
}

export function screenFromPath(pathname: string): SideScreen | undefined {
  const slug = pathname.replace(/^\/+|\/+$/g, "");
  return (Object.keys(screenSlugs) as SideScreen[]).find(
    (screen) => screenSlugs[screen] === slug,
  );
}

export type PageLocation = {
  limit: number;
  article?: string;
  // `note=1` asks the submission page to open with the note box focused. It is
  // consumed once: writePageLocation drops it so a later reload never steals
  // the focus again.
  note?: boolean;
  // Absent means the work list, which is the application.
  screen?: SideScreen;
  // The search combobox's whole state (chips + free text), reconciled from
  // `?q` per spec.md's load-time rule. `screen` above stays path-only (a
  // `folder:` token that disagrees with it is the caller's job to act on,
  // via `folderChipScreen`), so this reconciliation cannot recurse.
  query: Query;
};

export function clampPageLimit(value: number): number {
  if (!Number.isSafeInteger(value) || value < 1) return defaultPageLimit;
  return Math.min(value, maxPageLimit);
}

export function readPageLocation(
  params: URLSearchParams,
  pathname: string,
): PageLocation {
  const screen = screenFromPath(pathname);
  return {
    limit: clampPageLimit(Number(params.get("limit"))),
    article: params.get("article") || undefined,
    note: params.get("note") === "1" || undefined,
    screen,
    query: reconcileQuery(params.get("q"), screen),
  };
}

export function writePageLocation(
  params: URLSearchParams,
  location: Pick<PageLocation, "limit" | "article" | "query">,
): void {
  params.delete("limit");
  if (location.article) params.set("article", location.article);
  else params.delete("article");
  params.delete("note");
  const wire = toWireQuery(location.query);
  if (wire) params.set("q", wire);
  else params.delete("q");
}
