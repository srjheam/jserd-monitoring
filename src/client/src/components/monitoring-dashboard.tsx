"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Button,
  FieldError,
  Form,
  Heading,
  Input,
  Label,
  TextField,
} from "react-aria-components";
import {
  loadEditorFrequencies,
  loadMonitoringMeta,
  loadSideScreen,
  loadSubmission,
  loadSubmissionPage,
  markUpToDate,
  pb,
  removeConference,
  restoreSession,
  saveConference,
  sendBackToWork,
  setMuted,
} from "../lib/api";
import {
  type Conference,
  defaultPageLimit,
  type EditorFrequency,
  type MonitoringMeta,
  maxPageLimit,
  type PageLocation,
  pathForScreen,
  readPageLocation,
  type SideScreen,
  type Submission,
  writePageLocation,
} from "../lib/monitoring";
import {
  emptyQuery,
  folderChipScreen,
  folderLabel,
  type Query,
  toWireQuery,
} from "../lib/query";
import { type LoadPage, useSubmissionList } from "../lib/use-submission-list";
import { ConferenceFields, useConferenceDraft } from "./conference-fields";
import { AppModal } from "./modal";
import type { SaveState } from "./save-status";
import { ArchivedScreen, MutedScreen, UpToDateScreen } from "./side-screens";
import type { OnSendBackToWork } from "./submission/send-back-to-work-button";
import type { OnMarkUpToDate } from "./submission/up-to-date-button";
import { SubmissionPage as SubmissionPageView } from "./submission-page";
import { TopBar, useCompact } from "./top-bar";
import { WorkList } from "./work-list";

type AuthState = "checking" | "signedOut" | "signedIn" | "expired";
type DialogState = { kind: "conferences" } | null;
export function MonitoringDashboard() {
  const [auth, setAuth] = useState<AuthState>("checking");
  const [query, setQuery] = useState<Query>(emptyQuery);
  const [editorFrequencies, setEditorFrequencies] = useState<EditorFrequency[]>(
    [],
  );
  const [dialog, setDialog] = useState<DialogState>(null);
  const [selectedID, setSelectedID] = useState<string>();
  const [selectedFallback, setSelectedFallback] = useState<Submission>();
  const [pageLocation, setPageLocation] = useState<PageLocation>({
    limit: defaultPageLimit,
    query: emptyQuery,
  });
  const [urlReady, setURLReady] = useState(false);
  const [message, setMessage] = useState("");
  // Which submission, if any, should have its note box focused. Set by a
  // cold load that carries `note=1` and by every `✎`/title click since:
  // `pageLocation` is written only on mount and on `popstate`, so it cannot
  // carry this by itself without also going stale the moment a plain click
  // reopens the same row.
  const [noteFocusID, setNoteFocusID] = useState<string>();
  const t = useTranslations("dashboard");
  const sentinelRef = useRef<HTMLDivElement | null>(null);
  // The work list's own scrolling region: infinite scroll watches the
  // sentinel against this element, not the viewport, now that the rows list
  // scrolls inside a fixed-height column instead of the whole page.
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const compact = useCompact();

  useEffect(() => {
    const readLocation = () => {
      const location = readPageLocation(
        new URLSearchParams(window.location.search),
        window.location.pathname,
      );
      // A `folder:` token inside `?q` can disagree with the path (a
      // bookmark or a typed edit); it wins, and the write effect below
      // corrects the path to match on its next run (spec.md's load-time
      // reconciliation rule).
      setPageLocation({
        ...location,
        screen: folderChipScreen(location.query),
      });
      setQuery(location.query);
      setSelectedID(location.article);
      setNoteFocusID(location.note ? location.article : undefined);
      setURLReady(true);
    };
    readLocation();
    window.addEventListener("popstate", readLocation);
    return () => window.removeEventListener("popstate", readLocation);
  }, []);

  const handleFailure = useCallback(
    (error: unknown) => {
      // A request the SDK auto-cancelled was replaced by a newer one, not
      // refused: two quick unmutes overlap two reloads, and the first would
      // otherwise raise "data unavailable" over a screen that succeeded.
      if ((error as { isAbort?: boolean }).isAbort) return;
      if ((error as { status?: number }).status === 401) {
        pb.authStore.clear();
        setAuth("expired");
      } else setMessage(t("dataUnavailable"));
    },
    [t],
  );

  const screen = pageLocation.screen;
  const wire = toWireQuery(query);

  // One module loads both list shapes. It owns the stale-batch guard, the
  // batch count and the replay, so none of that is restated here.
  const loadPage = useCallback<LoadPage>(
    (request) =>
      screen
        ? loadSideScreen(request.query, screen)
        : loadSubmissionPage(request),
    [screen],
  );
  const list = useSubmissionList({
    mode: screen ? "batch" : "paged",
    query: wire,
    limit: screen ? maxPageLimit : pageLocation.limit,
    enabled: auth === "signedIn",
    loadPage,
    loadMeta: loadMonitoringMeta,
    onFailure: handleFailure,
  });
  const { meta, items, total, replay } = list;

  // A load that succeeded clears a stale failure line.
  useEffect(() => {
    if (meta) setMessage("");
  }, [meta]);

  // Ranks the combobox's `editor:` suggestions (spec.md's three-scope
  // fallback order): scoped by the committed `folder:` chip and free text,
  // never by what is merely typed inside a chip being edited.
  const folderChipValue = query.chips.find(
    (chip) => chip.tag === "folder",
  )?.value;
  useEffect(() => {
    if (auth !== "signedIn") return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      loadEditorFrequencies({ folder: folderChipValue, text: query.text })
        .then((editors) => {
          if (!cancelled) setEditorFrequencies(editors);
        })
        .catch(() => {});
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [auth, folderChipValue, query.text]);

  useEffect(() => {
    restoreSession().then(setAuth);
  }, []);

  useEffect(() => {
    const node = sentinelRef.current;
    if (!node || auth !== "signedIn" || !meta) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) void list.loadMore();
      },
      // On desktop the rows list scrolls inside its own column, so the
      // sentinel is watched against that column rather than the viewport.
      { root: compact ? null : scrollRef.current, rootMargin: "240px 0px" },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [auth, compact, list, meta]);

  // Always read the selected submission from the single-submission route.
  // The list response drops `provenance` to keep a page cheap, so a list item
  // is not a complete submission and the detail panel cannot render one.
  useEffect(() => {
    if (!meta || !selectedID) {
      setSelectedFallback(undefined);
      return;
    }
    let cancelled = false;
    loadSubmission(selectedID)
      .then((submission) => {
        if (!cancelled) setSelectedFallback(submission);
      })
      .catch(() => {
        if (!cancelled) setSelectedFallback(undefined);
      });
    return () => {
      cancelled = true;
    };
  }, [meta, selectedID]);

  useEffect(() => {
    if (!urlReady || auth !== "signedIn" || !meta) return;
    const url = new URL(window.location.href);
    url.pathname = pathForScreen(screen);
    writePageLocation(url.searchParams, {
      limit: pageLocation.limit,
      article: selectedID,
      query,
    });
    window.history.replaceState(null, "", url);
  }, [auth, meta, pageLocation, query, screen, selectedID, urlReady]);

  // A curation write can move a submission off whichever screen is open, so
  // both shapes refresh the same way: replay the batches on screen. Chaining
  // fresh cursors means a removed row leaves no gap and duplicates no row.
  const refreshed = replay;

  const selected = selectedFallback;
  // `✎` opens the submission page with the note box focused. `note=1` is the
  // signal the submission page reads; it is a navigation, never a popup.
  const open = (id: string, note: boolean) => {
    setSelectedID(id);
    setNoteFocusID(note ? id : undefined);
    const url = new URL(window.location.href);
    url.searchParams.set("article", id);
    if (note) url.searchParams.set("note", "1");
    else url.searchParams.delete("note");
    // On mobile the detail panel is a full-screen modal, so opening it pushes
    // a history entry: the browser's own back action then closes it, the
    // same as every other mobile page.
    if (compact) window.history.pushState(null, "", url);
    else window.history.replaceState(null, "", url);
  };
  // A side screen is a different list, so the keyset window of the work list
  // does not survive the trip. `limit` and the open submission do survive,
  // and the effect above writes them both back into the URL.
  // The list module restarts itself when `screen` changes, because the mode,
  // the loader and the limit all change with it.
  const navigateScreen = (next?: SideScreen) => {
    setPageLocation((current) => ({ ...current, screen: next }));
  };
  // The footer links and the Archived menu item swap only the route: any
  // `editor:`/text filter already typed survives the trip, the same as
  // before this ticket, and only the `folder:` chip is replaced to match.
  const openScreen = (next?: SideScreen) => {
    navigateScreen(next);
    const slug = next ?? "work";
    setQuery((current) => ({
      ...current,
      chips: [
        { tag: "folder" as const, value: slug, label: folderLabel(slug) },
        ...current.chips.filter((chip) => chip.tag !== "folder"),
      ],
    }));
  };
  // Shared by the work list row and the detail panel's relocated action: the
  // row that marks a submission up to date leaves the work list either way.
  const onMarkUpToDate: OnMarkUpToDate = async (submission, until, reason) => {
    try {
      await markUpToDate(submission, until, reason);
      // The row left the work list. A replay chains fresh cursors, so the
      // rows that slide up arrive once each: there is nothing to drop by hand.
      await replay();
    } catch (error) {
      handleFailure(error);
    }
  };
  // Shared by the Up to date row and the detail panel's relocated action,
  // the same way onMarkUpToDate is: whichever one is showing, the submission
  // leaves Up to date either way. It replays rather than filtering the list in
  // place, because the panel action can fire while a different screen than Up
  // to date is the one on screen.
  const onSendBackToWork: OnSendBackToWork = async (submission) => {
    try {
      await sendBackToWork(submission);
      await refreshed();
    } catch (error) {
      handleFailure(error);
    }
  };

  if (auth !== "signedIn")
    return (
      <SignIn
        state={auth}
        onSignedIn={() => {
          setAuth("signedIn");
        }}
      />
    );
  return (
    <main className="monitoring-shell">
      <TopBar
        meta={meta}
        user={pb.authStore.record?.email ?? ""}
        onManageConferences={() => setDialog({ kind: "conferences" })}
        onSignOut={() => {
          pb.authStore.clear();
          // The list module is disabled while signed out, so the next chief
          // editor's first batch starts the list over.
          setSelectedID(undefined);
          setSelectedFallback(undefined);
          setAuth("signedOut");
        }}
      />
      {!meta ? (
        message ? (
          <p className="notice failure" role="alert">
            {message}
          </p>
        ) : (
          <output className="notice">{t("loading")}</output>
        )
      ) : (
        <>
          {message && (
            <p className="notice failure" role="alert">
              {message}
            </p>
          )}
          {meta.availability === "empty" ||
          meta.availability === "unavailable" ? (
            <p className="notice">{t("emptyProjection")}</p>
          ) : (
            <section className="monitoring-workspace">
              <ScreenSwitch
                screen={screen}
                context={{
                  meta,
                  query,
                  editorFrequencies,
                  selectedId: selectedID,
                  onQuery: setQuery,
                  onNavigate: navigateScreen,
                  onOpen: open,
                  onScreen: openScreen,
                }}
                list={{
                  items,
                  loaded: list.loaded,
                  total,
                  hasMore: list.hasMore,
                  loadingMore: list.loadingMore,
                  scrollRef,
                  sentinelRef,
                  onLoadMore: () => void list.loadMore(),
                }}
                onUnmute={async (submission) => {
                  try {
                    await setMuted(submission.tracking?.id, false);
                    await replay();
                  } catch (error) {
                    handleFailure(error);
                  }
                }}
                onMarkUpToDate={onMarkUpToDate}
                onSendBackToWork={onSendBackToWork}
              />
              <SubmissionPageView
                submission={selected}
                conferences={meta.conferences}
                focusNote={
                  selectedID !== undefined && selectedID === noteFocusID
                }
                onBack={() => setSelectedID(undefined)}
                onSaved={refreshed}
                onMarkUpToDate={onMarkUpToDate}
                onSendBackToWork={onSendBackToWork}
              />
            </section>
          )}
          <ModalDialog
            state={dialog}
            close={() => setDialog(null)}
            meta={meta}
            refreshed={refreshed}
          />
        </>
      )}
    </main>
  );
}

// ScreenContext is what every screen needs regardless of which one it is: the
// projection to show, the search state, and the four callbacks that move the
// route or the selection. A prop that every screen needs is one edit here, not
// one edit per screen in a pass-through list.
type ScreenContext = {
  meta: MonitoringMeta;
  query: Query;
  editorFrequencies: EditorFrequency[];
  selectedId: string | undefined;
  onQuery: (query: Query) => void;
  onNavigate: (screen: SideScreen | undefined) => void;
  onOpen: (id: string, note: boolean) => void;
  onScreen: (screen?: SideScreen) => void;
};

// ListWindow is the batch of submissions loaded so far, and how to ask for the
// next one. The side screens read the first two members; the work list reads
// all of them, because it is the only screen that pages.
type ListWindow = {
  items: Submission[];
  loaded: boolean;
  total: number;
  hasMore: boolean;
  loadingMore: boolean;
  scrollRef: React.Ref<HTMLDivElement>;
  sentinelRef: React.Ref<HTMLDivElement>;
  onLoadMore: () => void;
};

// Which of the four screens renders is decided by the route alone. The three
// actions below stay separate props because each belongs to one screen: a
// screen that cannot unmute has no reason to be handed `onUnmute`.
function ScreenSwitch({
  screen,
  context,
  list,
  onUnmute,
  onMarkUpToDate,
  onSendBackToWork,
}: {
  screen: SideScreen | undefined;
  context: ScreenContext;
  list: ListWindow;
  onUnmute: (submission: Submission) => Promise<void>;
  onMarkUpToDate: OnMarkUpToDate;
  onSendBackToWork: OnSendBackToWork;
}) {
  if (screen === "muted")
    return (
      <MutedScreen
        {...context}
        items={list.items}
        loaded={list.loaded}
        route="muted"
        onUnmute={onUnmute}
      />
    );
  if (screen === "up_to_date")
    return (
      <UpToDateScreen
        {...context}
        items={list.items}
        loaded={list.loaded}
        route="up_to_date"
        onSendBackToWork={onSendBackToWork}
      />
    );
  if (screen === "archived")
    return (
      <ArchivedScreen
        {...context}
        items={list.items}
        loaded={list.loaded}
        route="archived"
      />
    );
  return (
    <WorkList
      {...context}
      {...list}
      route={screen}
      onMarkUpToDate={onMarkUpToDate}
    />
  );
}

function SignIn({
  state,
  onSignedIn,
}: {
  state: AuthState;
  onSignedIn: () => void;
}) {
  const t = useTranslations("signIn");
  const tCommon = useTranslations("common");
  const [mode, setMode] = useState<"credentials" | "reset">("credentials");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [status, setStatus] = useState<SaveState>("idle");
  if (mode === "reset")
    return <RequestPasswordReset onBack={() => setMode("credentials")} />;
  return (
    <main className="sign-in">
      <Form
        validationBehavior="aria"
        onSubmit={async (event) => {
          event.preventDefault();
          setStatus("loading");
          try {
            await pb
              .collection("chief_editors")
              .authWithPassword(email, password);
            onSignedIn();
          } catch {
            setError(t("failed"));
            setStatus("error");
          }
        }}
      >
        <p className="eyebrow">{t("eyebrow")}</p>
        <Heading level={1}>{t("title")}</Heading>
        <p>{t("subtitle")}</p>
        {state === "checking" ? (
          <output>{t("restoring")}</output>
        ) : state === "expired" ? (
          <p className="failure" role="alert">
            {t("expired")}
          </p>
        ) : null}
        <TextField value={email} onChange={setEmail} isRequired>
          <Label>{t("email")}</Label>
          <Input type="email" />
          <FieldError />
        </TextField>
        <TextField value={password} onChange={setPassword} isRequired>
          <Label>{t("password")}</Label>
          <Input type="password" />
          <FieldError />
        </TextField>
        {error && (
          <p className="failure" role="alert">
            {error}
          </p>
        )}
        <Button type="submit" isDisabled={status === "loading"}>
          {status === "loading" ? tCommon("saving") : t("submit")}
        </Button>
        <Button className="link-button" onPress={() => setMode("reset")}>
          {t("forgotPassword")}
        </Button>
      </Form>
    </main>
  );
}

// PocketBase's own reset-request endpoint always resolves, whether or not the
// address has an account, so this form has only a success or a network-error
// outcome, never a "no such account" one.
function RequestPasswordReset({ onBack }: { onBack: () => void }) {
  const t = useTranslations("passwordReset");
  const tSignIn = useTranslations("signIn");
  const tCommon = useTranslations("common");
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [status, setStatus] = useState<SaveState>("idle");
  return (
    <main className="sign-in">
      <Form
        validationBehavior="aria"
        onSubmit={async (event) => {
          event.preventDefault();
          setStatus("loading");
          try {
            await pb.collection("chief_editors").requestPasswordReset(email);
            setStatus("success");
          } catch {
            setError(t("requestFailed"));
            setStatus("error");
          }
        }}
      >
        <Heading level={1}>{t("requestTitle")}</Heading>
        {status === "success" ? (
          <output>{t("requestSuccess")}</output>
        ) : (
          <>
            <p>{t("requestSubtitle")}</p>
            <TextField value={email} onChange={setEmail} isRequired>
              <Label>{tSignIn("email")}</Label>
              <Input type="email" />
              <FieldError />
            </TextField>
            {error && (
              <p className="failure" role="alert">
                {error}
              </p>
            )}
            <Button type="submit" isDisabled={status === "loading"}>
              {status === "loading" ? tCommon("saving") : t("requestSubmit")}
            </Button>
          </>
        )}
        <Button className="link-button" onPress={onBack}>
          {t("backToSignIn")}
        </Button>
      </Form>
    </main>
  );
}

// The one modal the dashboard opens. It holds the conference dialog and
// nothing else, so DialogState has one member today; it is a state rather than
// a boolean because a second dialog would join it here.
function ModalDialog({
  state,
  close,
  meta,
  refreshed,
}: {
  state: DialogState;
  close: () => void;
  meta: MonitoringMeta;
  refreshed: () => Promise<void>;
}) {
  const tCommon = useTranslations("common");
  if (!state) return null;
  return (
    <AppModal isOpen onOpenChange={(open) => !open && close()}>
      {({ close: closeDialog }) => (
        <>
          <ConferencesForm
            conferences={meta.conferences}
            done={async () => {
              await refreshed();
              closeDialog();
            }}
          />
          <Button slot="close">{tCommon("close")}</Button>
        </>
      )}
    </AppModal>
  );
}

function ConferencesForm({
  conferences,
  done,
}: {
  conferences: Conference[];
  done: () => Promise<void>;
}) {
  const t = useTranslations("conferencesForm");
  const tCommon = useTranslations("common");
  const draft = useConferenceDraft();
  const [editingID, setEditingID] = useState<string>();
  return (
    <section>
      <Heading slot="title">{t("title")}</Heading>
      <Form
        validationBehavior="aria"
        onSubmit={async (event) => {
          event.preventDefault();
          draft.start();
          try {
            await saveConference(editingID, draft.name, draft.description);
            draft.clear();
            setEditingID(undefined);
            await done();
          } catch {
            draft.fail(t("saveFailed"));
          }
        }}
      >
        <ConferenceFields draft={draft} />
        <Button type="submit" isDisabled={draft.status === "loading"}>
          {draft.status === "loading"
            ? tCommon("saving")
            : editingID
              ? t("update")
              : t("create")}
        </Button>
      </Form>
      <ul className="conference-list">
        {conferences.map((conference) => (
          <li key={conference.id}>
            <span>
              {conference.name}
              {conference.description ? `: ${conference.description}` : ""}
            </span>
            <Button
              onPress={() => {
                setEditingID(conference.id);
                draft.setName(conference.name);
                draft.setDescription(conference.description);
              }}
            >
              {tCommon("edit")}
            </Button>
            <Button
              onPress={async () => {
                if (
                  window.confirm(t("deleteConfirm", { name: conference.name }))
                ) {
                  try {
                    await removeConference(conference.id);
                    await done();
                  } catch {
                    draft.setError(t("deleteFailed"));
                  }
                }
              }}
            >
              {tCommon("delete")}
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
