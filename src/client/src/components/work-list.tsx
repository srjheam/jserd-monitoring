"use client";

import { Pencil } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "react-aria-components";
import type {
  EditorFrequency,
  MonitoringMeta,
  SideScreen,
  Submission,
} from "../lib/monitoring";
import type { Query } from "../lib/query";
import { screenHref } from "../lib/side-screens";
import { SearchCombobox } from "./search-combobox";
import { follow, noteHref, plainClick, RowsList } from "./submission/row";
import {
  type OnMarkUpToDate,
  UpToDateSplitButton,
} from "./submission/up-to-date-button";

export function WorkList({
  items,
  meta,
  query,
  editorFrequencies,
  route,
  selectedId,
  total,
  hasMore,
  loadingMore,
  scrollRef,
  sentinelRef,
  onQuery,
  onNavigate,
  onOpen,
  onScreen,
  onLoadMore,
  onMarkUpToDate,
}: {
  items: Submission[];
  meta: MonitoringMeta;
  query: Query;
  editorFrequencies: EditorFrequency[];
  route: SideScreen | undefined;
  selectedId?: string;
  total: number;
  hasMore: boolean;
  loadingMore: boolean;
  scrollRef?: React.Ref<HTMLDivElement>;
  sentinelRef?: React.Ref<HTMLDivElement>;
  onQuery: (query: Query) => void;
  onNavigate: (screen: SideScreen | undefined) => void;
  onOpen: (id: string, note: boolean) => void;
  onScreen: (screen?: SideScreen) => void;
  onLoadMore: () => void;
  onMarkUpToDate: OnMarkUpToDate;
}) {
  const t = useTranslations("workList");
  const filtered =
    query.chips.some((chip) => chip.tag === "editor") ||
    query.text.trim() !== "";
  const allFolders = !query.chips.some((chip) => chip.tag === "folder");
  return (
    <section className="work-list" aria-label={t("section")}>
      <SearchCombobox
        query={query}
        editorFrequencies={editorFrequencies}
        route={route}
        onQuery={onQuery}
        onNavigate={onNavigate}
      />
      <div className="rows-scroll" ref={scrollRef}>
        {items.length === 0 ? (
          <EmptyState filtered={filtered} />
        ) : (
          <RowsList
            items={items}
            selectedId={selectedId}
            showFolderBadge={allFolders}
            searchMode={filtered}
            onOpen={onOpen}
            renderAction={(item) => (
              <RowActions
                submission={item}
                onOpen={onOpen}
                onMarkUpToDate={onMarkUpToDate}
              />
            )}
          />
        )}
        <ListStatus
          hasMore={hasMore}
          loadingMore={loadingMore}
          onLoadMore={onLoadMore}
        />
        <div ref={sentinelRef} className="scroll-sentinel" aria-hidden="true" />
      </div>
      {/* A chief editor reads titles first. A total is a summary, and a
          summary belongs after the thing it summarizes.

          Each count is the link to the screen it names. The href is a real
          destination, so a middle click opens it in a tab; a plain click is
          handled in the page, which keeps `limit` and the open submission. */}
      <footer className="work-list__footer">
        <output aria-live="polite">{t("count", { count: total })}</output>
        <a
          className="side-screen"
          href={screenHref("muted")}
          onClick={plainClick(() => onScreen("muted"))}
        >
          {t("muted", { count: meta.aggregates.triage.muted ?? 0 })}
        </a>
        <a
          className="side-screen"
          href={screenHref("up_to_date")}
          onClick={plainClick(() => onScreen("up_to_date"))}
        >
          {t("upToDate", { count: meta.aggregates.triage.up_to_date ?? 0 })}
        </a>
        <a
          className="side-screen side-screen--archived"
          href={screenHref("archived")}
          onClick={plainClick(() => onScreen("archived"))}
        >
          {t("archived")}
        </a>
      </footer>
    </section>
  );
}

// The note-icon action and the up-to-date split button, the two actions a
// Daily row shows. Shared shape with `noteHref` from `./submission/row`.
function RowActions({
  submission,
  onOpen,
  onMarkUpToDate,
}: {
  submission: Submission;
  onOpen: (id: string, note: boolean) => void;
  onMarkUpToDate: OnMarkUpToDate;
}) {
  const t = useTranslations("workList");
  const id = submission.ojsSubmissionId;
  return (
    <>
      {/* The OJS `-screenReader` habit: the icon is decoration and the
          purpose is spelled out for assistive technology. */}
      <a
        className="row__note"
        href={noteHref(id)}
        onClick={follow(onOpen, id, true)}
      >
        <Pencil size={16} />
        <span className="-screenReader">{t("addNote")}</span>
      </a>
      <UpToDateSplitButton
        submission={submission}
        onMarkUpToDate={onMarkUpToDate}
      />
    </>
  );
}

// The count moved to the work list footer, beside the two side screen links.
// What is left here is the keyboard route to the next batch, which infinite
// scroll alone does not offer.
function ListStatus({
  hasMore,
  loadingMore,
  onLoadMore,
}: {
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
}) {
  const t = useTranslations("listing");
  return (
    <nav className="listing-status" aria-label={t("section")}>
      {hasMore && (
        <Button isDisabled={loadingMore} onPress={onLoadMore}>
          {loadingMore ? t("loadingMore") : t("loadMore")}
        </Button>
      )}
    </nav>
  );
}

// Every submission placed elsewhere is the success state, so the empty list
// must not read as a broken page. A quiet grey mark, no brand colour: a
// bright one would turn a routine morning into an event. A filtered search
// that finds nothing is not a success, so it shows no check mark.
function EmptyState({ filtered }: { filtered: boolean }) {
  const t = useTranslations("workList.empty");
  return (
    <div className="empty-state">
      <svg
        className="empty-state__art"
        viewBox="0 0 64 48"
        width="96"
        height="72"
        aria-hidden="true"
        focusable="false"
      >
        <rect x="1" y="1" width="62" height="46" rx="4" />
        {!filtered && <path d="M24 24 l6 7 l14 -15" />}
      </svg>
      <p className="empty-state__caption">
        {filtered ? t("noMatch") : t("nothing")}
      </p>
    </div>
  );
}
