import {
  type ActivityItem,
  type Conference,
  pathForScreen,
  type Submission,
} from "./monitoring";
import { setAsidePrefix } from "./wire";

// The screen href. A real anchor, so a side screen opens in a new tab like
// any other destination; the root path is the work list, which is the
// application.
export const screenHref = pathForScreen;

// isNewSinceMuted answers whether a finding appeared after the mute. The
// stamp is a hash of the finding kinds, so it can only say that the set
// changed, never which member changed: that is why ADR 0002 puts the mark on
// the row and leaves the row where it stands. An empty stamp is a submission
// that was never muted, because even a submission with no findings hashes to
// a non-empty digest.
export function isNewSinceMuted(submission: Submission): boolean {
  const stamped = submission.tracking?.mutedDigest ?? "";
  return stamped !== "" && stamped !== submission.findingDigest;
}

// The activity a cluster orders by is the last time the submission moved in
// OJS. A submission nothing has touched since it arrived falls back to the
// day it arrived.
export const lastActivity = (submission: Submission) =>
  submission.lastModified || submission.dateSubmitted || "";

export type Cluster = { name: string; items: Submission[] };

// clusterByConference groups the Muted screen by conference. Clusters sort
// alphanumerically by conference name, and rows inside a cluster sort by oldest
// activity first. `unnamed` names the cluster of submissions a chief editor
// muted without a conference; it sorts among the rest rather than
// taking a place of its own, because it is not more or less important.
export function clusterByConference(
  items: Submission[],
  conferences: Conference[],
  unnamed: string,
): Cluster[] {
  const names = new Map(
    conferences.map((conference) => [conference.id, conference.name]),
  );
  const clusters = new Map<string, Submission[]>();
  for (const item of items) {
    const name = names.get(item.tracking?.conference ?? "") ?? unnamed;
    const bucket = clusters.get(name);
    if (bucket) bucket.push(item);
    else clusters.set(name, [item]);
  }
  return [...clusters]
    .map(([name, bucket]) => ({
      name,
      items: [...bucket].sort((left, right) =>
        lastActivity(left).localeCompare(lastActivity(right)),
      ),
    }))
    .sort((left, right) =>
      left.name.localeCompare(right.name, undefined, { numeric: true }),
    );
}

// The opening of the one journal entry that markUpToDate writes. Nothing else
// writes it, so it identifies the marking among the notes a chief editor may
// have added to the same submission since. Re-exported from the generated wire
// module, so the server owns the words.
export { setAsidePrefix };

// The exact sentence markUpToDate composes, and nothing looser. The journal
// is free text and the timeline arrives newest first, so a plain "starts
// with Set aside" would let a chief editor's own note beginning with those
// words pass itself off as the marking, and carry its author with it.
const setAsideSentence = new RegExp(
  `^${setAsidePrefix}( until \\d{4}-\\d{2}-\\d{2})?\\.`,
);

// settingAsideEntry finds the journal entry that set a submission aside. The
// tracking record holds the two dates and nothing else, so the chief editor
// who set it aside and the reason they gave are only readable here.
export const settingAsideEntry = (items: ActivityItem[]) =>
  items.find(
    (item) => item.kind === "journal" && setAsideSentence.test(item.body ?? ""),
  );

// reasonOf drops that opening sentence and keeps the words the chief editor
// added. The row already shows the day and the return date from the tracking
// record, so repeating the sentence would print both of them twice.
export const reasonOf = (body: string) =>
  body.replace(setAsideSentence, "").trim();
