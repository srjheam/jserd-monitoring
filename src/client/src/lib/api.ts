import PocketBase from "pocketbase";
import type {
  ActivityItem,
  EditorFrequency,
  MonitoringMeta,
  SideScreen,
  Submission,
  SubmissionPage,
  Tracking,
} from "./monitoring";
import { maxPageLimit } from "./monitoring";
import { today } from "./work-list";

const baseURL =
  process.env.NEXT_PUBLIC_POCKETBASE_URL ||
  (typeof window === "undefined" ? "" : window.location.origin);
export const pb = new PocketBase(baseURL);

export async function restoreSession() {
  if (!pb.authStore.token) return "signedOut" as const;
  if (!pb.authStore.isValid) {
    pb.authStore.clear();
    return "expired" as const;
  }
  try {
    await pb.collection("chief_editors").authRefresh();
    return "signedIn" as const;
  } catch {
    pb.authStore.clear();
    return "expired" as const;
  }
}
export async function loadMonitoringMeta() {
  return pb.send<MonitoringMeta>("/api/jserd/monitoring", { method: "GET" });
}
export type PageRequest = {
  limit: number;
  cursor?: string;
  // The wire form written by toWireQuery: `folder:<slug> editor:"<name>"
  // free words`. A `folder:` token wins over the request's own `queue`,
  // and its absence means an all-folder search (server, ticket 01 part B/C)
  // — so the queue scope always travels inside this string, never as a
  // separate request param, and there is one place that decides it.
  query: string;
};
// True when `query` carries no `folder:` token: genuine all-folder mode
// (spec.md, folder-as-route interplay). The list endpoint defaults its own
// `queue` param to `"daily"` when nothing says otherwise (parseListQuery in
// query.go), so this case must say `queue=all` explicitly, or an all-folder
// search would silently narrow to the daily queue instead.
export function allFolderParams(query: string): Record<string, string> {
  return /(^|\s)folder:/.test(query) ? {} : { queue: "all" };
}
// One list, worst first. There is no queue tab and no sort control: the
// combobox's `folder:` chip, carried inside `query`, decides the scope,
// including the all-folder union when that chip is absent.
export async function loadSubmissionPage(request: PageRequest) {
  const query: Record<string, string> = {
    limit: String(request.limit),
    sort: "priority",
    ...allFolderParams(request.query),
  };
  if (request.cursor) query.cursor = request.cursor;
  if (request.query) query.query = request.query;
  return pb.send<SubmissionPage>("/api/jserd/monitoring/submissions", {
    method: "GET",
    query,
    requestKey: JSON.stringify(query),
  });
}
// A side screen is one batch, not a paginated list: Muted groups its whole
// set before it can render a cluster, and a cursor would hand it a fragment.
// The batch is therefore capped at the page limit, and a journal with more
// than `maxPageLimit` submissions on one side screen would show the first
// hundred and under-report every cluster header. JSERD holds tens, so the
// cap is not reached; a journal that outgrows it needs paging here.
export async function loadSideScreen(search: string, screen: SideScreen) {
  const query: Record<string, string> = {
    limit: String(maxPageLimit),
    ...allFolderParams(search),
  };
  if (search) query.query = search;
  // Archived carries no actionable finding rank, so it orders by OJS's own
  // last-activity timestamp instead, newest first. Muted and Up to date keep
  // the server's plain id order.
  if (screen === "archived") {
    query.sort = "activity";
    query.dir = "desc";
  }
  return pb.send<SubmissionPage>("/api/jserd/monitoring/submissions", {
    method: "GET",
    query,
    requestKey: JSON.stringify(query),
  });
}
// Ranks the search combobox's `editor:` suggestions (spec.md's three-scope
// fallback order). `folder` is the client's own vocabulary (never
// `worklist`); the server applies that one alias. The server also falls
// back to the unrestricted ranking when a scope names no editor, so this
// call never needs a second request to cover that case.
export async function loadEditorFrequencies(scope: {
  folder?: string;
  text?: string;
}) {
  const query: Record<string, string> = {};
  if (scope.folder) query.folder = scope.folder;
  if (scope.text) query.text = scope.text;
  return pb.send<EditorFrequency[]>(
    "/api/jserd/monitoring/aggregates/editors",
    { method: "GET", query, requestKey: JSON.stringify(query) },
  );
}
// The one writer of `muted`. `🔔` on the Muted screen and the mute checkbox
// in curation both flow through here, never through `saveTracking`: mute is
// a placement decision and curation writes editorial metadata, and letting
// the two share a payload lets a stale curation save silently undo a mute
// change made out of band. The conference association stays when unmuting,
// because it is a label and not a claim about the work: a submission the
// same chief editor had also set aside keeps that marking and lands on the
// Up to date screen, because muted merely outranked it. Clearing the stamped
// muted_digest is the server's job.
export async function setMuted(id: string | undefined, muted: boolean) {
  if (!id) return;
  await pb.collection("submission_tracking").update(id, { muted });
}
export async function loadSubmission(id: string) {
  return pb.send<Submission>(
    `/api/jserd/monitoring/submissions/${encodeURIComponent(id)}`,
    { method: "GET" },
  );
}
// `requestKey` names the caller, not the request. The SDK auto-cancels two
// in-flight requests that share a key, and it derives the default key from
// the method and the URL alone, so the Up to date row and the submission
// panel reading the same submission would abort each other: whichever lost
// would either strand the panel on its error line or drop the reason from
// the row.
export async function loadActivity(id: string, requestKey?: string) {
  return pb.send<ActivityItem[]>(
    `/api/jserd/monitoring/submissions/${encodeURIComponent(id)}/activity`,
    { method: "GET", requestKey },
  );
}
// The journal is append-only, so the only write it accepts is this one.
export async function appendJournalEntry(id: string, body: string) {
  return pb.send(
    `/api/jserd/monitoring/submissions/${encodeURIComponent(id)}/journal`,
    { method: "POST", body: { body } },
  );
}
export async function saveTracking(
  id: string | undefined,
  // Only the curated fields. The digests and the up to date dates are
  // written by "Mark up to date" and by the server, never by this form.
  // `muted` is not here either: it has its own writer, `setMuted`, so a
  // curation save never carries a value for a field the user did not touch.
  values: Pick<Tracking, "conference"> & {
    ojsSubmissionId: string;
  },
) {
  const body = {
    ojs_submission_id: values.ojsSubmissionId,
    conference: values.conference || "",
  };
  return id
    ? pb.collection("submission_tracking").update(id, body)
    : pb.collection("submission_tracking").create(body);
}
// markUpToDate and sendBackToWork each write the tracking record and the
// journal entry that explains it as one atomic server-side transaction: a
// write split across two requests could leave a tracking change stamped
// without the journal entry the Up to date screen reads to explain it, or
// the other way around. `today` travels from the browser rather than being
// stamped by the server, so a chief editor west of UTC marking a submission
// up to date in the evening gets the date their own calendar shows.
export async function markUpToDate(
  submission: Submission,
  until: string,
  reason: string,
) {
  return pb.send(
    `/api/jserd/monitoring/submissions/${encodeURIComponent(submission.ojsSubmissionId)}/up-to-date`,
    { method: "POST", body: { today: today(), until, reason } },
  );
}
// sendBackToWork reverses markUpToDate: it clears the tracking record's up
// to date fields and writes a fixed journal entry, atomically, moving the
// submission off the Up to date screen and back onto the work list.
export async function sendBackToWork(submission: Submission) {
  return pb.send(
    `/api/jserd/monitoring/submissions/${encodeURIComponent(submission.ojsSubmissionId)}/send-back-to-work`,
    { method: "POST" },
  );
}
export async function saveConference(
  id: string | undefined,
  name: string,
  description: string,
) {
  return id
    ? pb.collection("conferences").update(id, { name, description })
    : pb.collection("conferences").create({ name, description });
}
export async function removeConference(id: string) {
  return pb.collection("conferences").delete(id);
}
