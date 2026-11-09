import type { Submission } from "./monitoring";

// isoDate writes a plain calendar date from the LOCAL fields of `date`.
// `toISOString` would convert to UTC first, so a chief editor west of UTC
// pressing a preset in the evening would store a date one day further out
// than the one the menu named.
export function isoDate(date: Date): string {
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

export const today = () => isoDate(new Date());

// addMonth clamps to the end of the target month, because `setMonth` rolls
// over instead: 31 January plus one month would otherwise be 3 March.
export function addMonth(date: Date): Date {
  const day = date.getDate();
  const shifted = new Date(date);
  shifted.setDate(1);
  shifted.setMonth(shifted.getMonth() + 1);
  const lastDay = new Date(
    shifted.getFullYear(),
    shifted.getMonth() + 1,
    0,
  ).getDate();
  shifted.setDate(Math.min(day, lastDay));
  return shifted;
}

export type Elapsed = { kind: "overdue" | "waiting"; days: number };

// elapsed reads the line that carries the reason: an overdue deadline when
// the worst finding has one, and the time since the submission's most
// recent known editorial activity otherwise.
export function elapsed(submission: Submission, now: number): Elapsed | null {
  // The server ships a submission's findings worst first, so the first one is
  // the finding that ranks the submission and owns the deadline shown here.
  const deadline = submission.findings[0]?.deadline;
  const overdue = daysSince(deadline, now);
  if (overdue !== null && overdue > 0)
    return { kind: "overdue", days: overdue };
  const waiting = daysSince(mostRecentKnownActivity(submission), now);
  return waiting === null
    ? null
    : { kind: "waiting", days: Math.max(waiting, 0) };
}

// mostRecentKnownActivity picks the latest of dateLastActivity,
// dateSubmitted, lastModified, and every decision date, so the "waiting" line
// does not anchor on the original submission date alone once later activity is
// known. dateSubmitted never moves, so a submission with months of review
// activity since submission would otherwise report a "waiting" count far
// longer than the true time since anyone last acted on it. dateLastActivity is
// OJS's own timestamp and is the only candidate that moves when a reviewer
// completes a review.
function mostRecentKnownActivity(submission: Submission): string | undefined {
  const candidates = [
    submission.dateSubmitted,
    submission.lastModified,
    submission.dateLastActivity,
    ...submission.decisions.map((decision) => decision.date),
  ].filter((value): value is string => Boolean(value));
  return candidates.reduce<string | undefined>((latest, value) => {
    if (!latest) return value;
    return Date.parse(value) > Date.parse(latest) ? value : latest;
  }, undefined);
}

function daysSince(value: string | undefined, now: number): number | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) return null;
  return Math.floor((now - parsed) / 86_400_000);
}
