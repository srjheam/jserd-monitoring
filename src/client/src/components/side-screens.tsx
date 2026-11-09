"use client";

import { Bell, Inbox, Info } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useState } from "react";
import { Button, Tooltip, TooltipTrigger } from "react-aria-components";
import { loadActivity } from "../lib/api";
import { day } from "../lib/dates";
import type {
  ActivityItem,
  EditorFrequency,
  MonitoringMeta,
  SideScreen,
  Submission,
} from "../lib/monitoring";
import type { Query } from "../lib/query";
import {
  clusterByConference,
  isNewSinceMuted,
  reasonOf,
  screenHref,
  settingAsideEntry,
} from "../lib/side-screens";
import { SearchCombobox } from "./search-combobox";
import { plainClick, RowsList } from "./submission/row";
import {
  type OnSendBackToWork,
  SendBackToWorkButton,
} from "./submission/send-back-to-work-button";

type ScreenProps = {
  items: Submission[];
  // False until the first batch has answered. An empty screen is a claim
  // about the work, so it waits for the answer instead of guessing it from a
  // list that has not been filled yet.
  loaded: boolean;
  meta: MonitoringMeta;
  query: Query;
  editorFrequencies: EditorFrequency[];
  route: SideScreen;
  selectedId?: string;
  onQuery: (query: Query) => void;
  onNavigate: (screen: SideScreen | undefined) => void;
  onOpen: (id: string, note: boolean) => void;
  onScreen: (screen?: SideScreen) => void;
};

// Muted groups by conference. Clusters sort alphanumerically by conference
// name, and rows inside a cluster sort by oldest activity first.
export function MutedScreen({
  items,
  loaded,
  meta,
  query,
  editorFrequencies,
  route,
  selectedId,
  onQuery,
  onNavigate,
  onOpen,
  onScreen,
  onUnmute,
}: ScreenProps & { onUnmute: (submission: Submission) => Promise<void> }) {
  const t = useTranslations("sideScreens");
  const tSubmission = useTranslations("submission");
  const clusters = useMemo(
    () => clusterByConference(items, meta.conferences, t("noConference")),
    [items, meta.conferences, t],
  );
  return (
    <Shell
      label={t("muted")}
      query={query}
      editorFrequencies={editorFrequencies}
      route={route}
      onQuery={onQuery}
      onNavigate={onNavigate}
      onScreen={onScreen}
    >
      {items.length === 0
        ? loaded && <Empty text={t("mutedEmpty")} />
        : // The heading names the cluster. The section is deliberately not a
          // landmark: fourteen muted submissions would otherwise put a dozen
          // regions in the landmark list of a screen reader.
          clusters.map((cluster) => (
            <section key={cluster.name} className="cluster">
              <header className="cluster__header">
                <h3 className="conference-chip">
                  {tSubmission("conferenceChip", { name: cluster.name })}
                </h3>
                <span className="cluster__count">
                  {t("clusterCount", { count: cluster.items.length })}
                </span>
                {/* Conference associations are managed in JSERD Monitoring
                  separately from OJS, so the header says where they come from. */}
                <TooltipTrigger delay={0}>
                  <Button className="jserd-marker">
                    {tSubmission("jserdMarker")} <Info size={16} />
                  </Button>
                  <Tooltip className="tooltip">
                    {tSubmission("conferenceTooltip")}
                  </Tooltip>
                </TooltipTrigger>
              </header>
              <RowsList
                items={cluster.items}
                selectedId={selectedId}
                onOpen={onOpen}
                renderAction={(item) => (
                  <Button
                    className="row__unmute"
                    aria-label={t("unmute")}
                    onPress={() => void onUnmute(item)}
                  >
                    <Bell size={16} />
                  </Button>
                )}
                /* A finding that arrived after the mute marks the row where
                   it stands. It never moves it: the chief editor decides
                   whether to unmute. */
                renderExtra={(item) =>
                  isNewSinceMuted(item) && (
                    <span className="row__since-muted">
                      {" · "}
                      {t("newSinceMuted")}
                    </span>
                  )
                }
              />
            </section>
          ))}
    </Shell>
  );
}

// Up to date is a flat list. There is nothing to group by: the rows share
// only the fact that they need nothing right now.
export function UpToDateScreen({
  items,
  loaded,
  query,
  editorFrequencies,
  route,
  selectedId,
  onQuery,
  onNavigate,
  onOpen,
  onScreen,
  onSendBackToWork,
}: ScreenProps & { onSendBackToWork: OnSendBackToWork }) {
  const t = useTranslations("sideScreens");
  return (
    <Shell
      label={t("upToDate")}
      query={query}
      editorFrequencies={editorFrequencies}
      route={route}
      onQuery={onQuery}
      onNavigate={onNavigate}
      onScreen={onScreen}
    >
      {items.length === 0 ? (
        loaded && <Empty text={t("upToDateEmpty")} />
      ) : (
        <RowsList
          items={items}
          selectedId={selectedId}
          onOpen={onOpen}
          renderAction={(item) => (
            <SendBackToWorkButton
              submission={item}
              onSendBackToWork={onSendBackToWork}
            />
          )}
          renderExtra={(item) => <Reason submission={item} />}
        />
      )}
    </Shell>
  );
}

// Archived lists final submissions and Copyediting submissions with a
// confirmed Copyeditor assignment. It has no per-row curation actions.
export function ArchivedScreen({
  items,
  loaded,
  query,
  editorFrequencies,
  route,
  selectedId,
  onQuery,
  onNavigate,
  onOpen,
  onScreen,
}: ScreenProps) {
  const t = useTranslations("sideScreens");
  return (
    <Shell
      label={t("archived")}
      query={query}
      editorFrequencies={editorFrequencies}
      route={route}
      onQuery={onQuery}
      onNavigate={onNavigate}
      onScreen={onScreen}
    >
      {items.length === 0 ? (
        loaded && <Empty text={t("archivedEmpty")} />
      ) : (
        <RowsList items={items} selectedId={selectedId} onOpen={onOpen} />
      )}
    </Shell>
  );
}

function Shell({
  label,
  query,
  editorFrequencies,
  route,
  onQuery,
  onNavigate,
  onScreen,
  children,
}: {
  label: string;
  query: Query;
  editorFrequencies: EditorFrequency[];
  route: SideScreen;
  onQuery: (query: Query) => void;
  onNavigate: (screen: SideScreen | undefined) => void;
  onScreen: (screen?: SideScreen) => void;
  children: React.ReactNode;
}) {
  const t = useTranslations("sideScreens");
  return (
    <section className="work-list side-screen-view" aria-label={label}>
      <header className="side-screen-view__header">
        <h2>{label}</h2>
        <a
          className="side-screen-view__back"
          href={screenHref()}
          onClick={plainClick(() => onScreen(undefined))}
        >
          {t("backToWorkList")}
        </a>
      </header>
      {/* The same combobox as the work list, with the same chips: one set
          of rules to learn, not three. */}
      <SearchCombobox
        query={query}
        editorFrequencies={editorFrequencies}
        route={route}
        onQuery={onQuery}
        onNavigate={onNavigate}
      />
      <div className="rows-scroll">{children}</div>
    </section>
  );
}

// Every Up to date row says why it is there. A submission with no finding is
// there by itself; one a chief editor set aside carries their name, the day
// they set it aside, the reason they gave and the return date they chose.
function Reason({ submission }: { submission: Submission }) {
  const t = useTranslations("sideScreens");
  const setAside = submission.tracking?.upToDateAt ?? "";
  const until = submission.tracking?.upToDateUntil ?? "";
  // A submission with no finding is here by itself, so it has no marking to
  // read and asks for nothing.
  const entry = useSettingAside(
    submission.ojsSubmissionId,
    setAside !== "" && submission.findings.length > 0,
  );
  if (submission.findings.length === 0) return null;
  const reason = reasonOf(entry?.body ?? "");
  return (
    <span className="row__why">
      {entry?.authorLabel
        ? t("setAsideBy", { date: day(setAside), author: entry.authorLabel })
        : t("setAsideOn", { date: day(setAside) })}
      {until && ` · ${t("returnsOn", { date: day(until) })}`}
      {reason && ` · ${reason}`}
    </span>
  );
}

// The chief editor who set a submission aside and the reason they gave live
// in the journal entry that "Mark up to date" writes; the tracking record
// holds only the two dates. So each set-aside row reads its own entry rather
// than the list response growing a second shape for one line of text. That
// is one request per row a chief editor set aside by hand, bounded by the
// batch, and none for a row that is here because it has no finding. A row
// whose entry cannot be read keeps the date.
function useSettingAside(id: string, enabled: boolean) {
  const [entry, setEntry] = useState<ActivityItem>();
  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    // Its own auto-cancel lane, so it never races the submission panel
    // reading the same activity route for the same submission.
    loadActivity(id, `side-screen-activity-${id}`)
      .then((items) => {
        if (!cancelled) setEntry(settingAsideEntry(items));
      })
      .catch(() => {
        if (!cancelled) setEntry(undefined);
      });
    return () => {
      cancelled = true;
    };
  }, [id, enabled]);
  return entry;
}

// An empty side screen is not a success and not a failure. It is the same
// quiet grey line the work list uses, with no check mark.
function Empty({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <Inbox className="empty-state__art" size={48} />
      <p className="empty-state__caption">{text}</p>
    </div>
  );
}
