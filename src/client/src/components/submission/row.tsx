"use client";

import { TriangleAlert } from "lucide-react";
import { useTranslations } from "next-intl";
import { useMemo } from "react";
import type { Submission } from "../../lib/monitoring";
import { elapsed } from "../../lib/work-list";

// The row href. A real anchor, never a row-level click handler: a handler
// breaks middle-click, focus and screen readers.
export const articleHref = (id: string) => `?article=${encodeURIComponent(id)}`;
// `✎` is not a popup. It opens the submission page with the note box focused,
// and `note=1` is the signal the submission page reads to focus it.
export const noteHref = (id: string) => `${articleHref(id)}&note=1`;

// plainClick keeps an anchor a real anchor. A plain left click is handled in
// place; a middle click, a modified click and "open in new tab" never reach
// the handler, so they keep the browser behaviour the href promises.
export function plainClick(handler: () => void) {
  return (event: React.MouseEvent) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)
      return;
    event.preventDefault();
    handler();
  };
}

export const follow = (
  onOpen: (id: string, note: boolean) => void,
  id: string,
  note: boolean,
) => plainClick(() => onOpen(id, note));

// Every finding is warnable, so severity is carried by the order of the list
// and by the elapsed line, not by five hues. Colour never carries meaning
// alone: each badge pairs it with an uppercase word and an icon.
//
// The badge label is deliberately shorter than the full attention sentence:
// a badge is a pill on a row that must stay readable at 375px, so it reads
// "NO EDITOR" where the panel reads "No editor assigned".
export function Badge({ kind, worst }: { kind: string; worst: boolean }) {
  const t = useTranslations("workList.badge");
  return (
    <span className={`badge${worst ? " badge--solid" : ""}`}>
      <TriangleAlert size={16} /> {t.has(kind) ? t(kind) : kind}
    </span>
  );
}

// The one row anatomy every screen renders: the OJS `pkpListPanelItem` shape
// of id/author/title/activity inside one anchor, with the action cluster
// outside it. `action` and `children` are the only per-screen variation: a
// Daily row's note-and-up-to-date actions, a Muted row's unmute button, an
// Up to date row's send-back-to-work button, and the "new since muted"/reason
// text a row's activity line carries. The open row's own actions are shown
// once, in the detail panel next to "Post", so an active row hides them
// instead of repeating them.
export function Row({
  submission,
  active,
  showFolderBadge = false,
  searchMode = false,
  onOpen,
  action,
  children,
}: {
  submission: Submission;
  active: boolean;
  showFolderBadge?: boolean;
  searchMode?: boolean;
  onOpen: (id: string, note: boolean) => void;
  action?: React.ReactNode;
  children?: React.ReactNode;
}) {
  const t = useTranslations("workList");
  const tCommon = useTranslations("common");
  const tQueue = useTranslations("enums.queue");
  const id = submission.ojsSubmissionId;
  const age = useMemo(() => elapsed(submission, Date.now()), [submission]);
  // Badges read worst first, the same order the list itself uses, and the
  // worst one wears the solid badge while the rest stay outlined. The server
  // ships them in that order, so there is nothing to sort here.
  const findings = submission.findings;
  const isCopyediting =
    submission.status.trim().toLowerCase() === "copyediting";
  return (
    <article className={`row${active ? " row--active" : ""}`}>
      <a
        className="row__link"
        href={articleHref(id)}
        onClick={follow(onOpen, id, false)}
      >
        <span className="row__id">
          <span className="-screenReader">{t("idLabel")} </span>
          {id}
        </span>
        <span className="row__author">
          {submission.editors.join(", ") || t("noEditor")}
        </span>
        {/* The title is weight 700 and the author 400. This inverts OJS,
            where the author is bold, on purpose: our unit is the submission,
            not the author. */}
        <span className="row__title">
          {submission.title || tCommon("unavailable")}
          <span aria-hidden="true" className="row__chevron">
            {" ›"}
          </span>
        </span>
        <span className="row__activity">
          {/* All-folder search mixes rows from every queue, so each one
              needs its own folder label: on a single-folder page the
              route already says it, and the badge would only repeat it. */}
          {showFolderBadge && (
            <span
              className={`badge row__folder-badge${searchMode ? " badge--search" : ""}`}
            >
              {tQueue(submission.effectiveQueue)}
            </span>
          )}
          {findings.map((finding, index) => (
            <Badge
              key={`${finding.kind}-${index}`}
              kind={finding.kind}
              worst={index === 0}
            />
          ))}
          {!isCopyediting &&
            (submission.effectiveFinalState === "accepted" ||
              submission.effectiveFinalState === "rejected" ||
              submission.effectiveFinalState === "withdrawn") && (
              <span className="badge badge--final">
                {t(`badge.${submission.effectiveFinalState}`)}
              </span>
            )}
          {isCopyediting && (
            <span className="badge badge--final">{t("badge.copyediting")}</span>
          )}
          {age && (
            <span className="row__elapsed">
              {t(age.kind === "overdue" ? "overdueDays" : "waitingDays", {
                days: age.days,
              })}
            </span>
          )}
          {children}
        </span>
      </a>
      {!active && action && <div className="row__actions">{action}</div>}
    </article>
  );
}

// RowsList renders one screen's flat list of rows: Work uses it directly,
// the side screens' shared wrapper uses it for Up to date and Archived, and
// Muted uses it once per cluster. The empty state stays with each caller,
// because Work's and the side screens' empty states read differently.
export function RowsList({
  items,
  selectedId,
  showFolderBadge,
  searchMode,
  onOpen,
  renderAction,
  renderExtra,
}: {
  items: Submission[];
  selectedId?: string;
  showFolderBadge?: boolean;
  searchMode?: boolean;
  onOpen: (id: string, note: boolean) => void;
  renderAction?: (item: Submission) => React.ReactNode;
  renderExtra?: (item: Submission) => React.ReactNode;
}) {
  return (
    <div className="rows">
      {items.map((item) => (
        <Row
          key={item.ojsSubmissionId}
          submission={item}
          active={item.ojsSubmissionId === selectedId}
          showFolderBadge={showFolderBadge}
          searchMode={searchMode}
          onOpen={onOpen}
          action={renderAction?.(item)}
        >
          {renderExtra?.(item)}
        </Row>
      ))}
    </div>
  );
}
