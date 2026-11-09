"use client";

import {
  ArrowLeft,
  Info,
  Mail,
  MailX,
  Pencil,
  TriangleAlert,
  Unlink,
} from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Button,
  CheckboxButton,
  CheckboxField,
  Form,
  Heading,
  Input,
  Label,
  SearchField,
  TextArea,
  TextField,
  Tooltip,
  TooltipTrigger,
} from "react-aria-components";
import {
  appendJournalEntry,
  loadActivity,
  saveConference,
  saveTracking,
  setMuted,
} from "../lib/api";
import { day } from "../lib/dates";
import type { ActivityItem, Conference, Submission } from "../lib/monitoring";
import { ConferenceFields, useConferenceDraft } from "./conference-fields";
import { AppModal } from "./modal";
import { SaveStatus, useSaveStatus } from "./save-status";
import {
  type OnSendBackToWork,
  SendBackToWorkButton,
} from "./submission/send-back-to-work-button";
import {
  type OnMarkUpToDate,
  UpToDateSplitButton,
} from "./submission/up-to-date-button";
import { useCompact } from "./top-bar";

// Every finding kind, delivery status and failure class the server can send is
// translated; unknown values use the supplied fallback or their original value.
function useWords(namespace: string, fallback?: string) {
  const t = useTranslations(namespace);
  return (value: string) =>
    t.has(value) ? t(value) : fallback ? t(fallback) : value;
}

export function SubmissionPage({
  submission,
  conferences,
  focusNote,
  onBack,
  onSaved,
  onMarkUpToDate,
  onSendBackToWork,
}: {
  submission?: Submission;
  conferences: Conference[];
  focusNote: boolean;
  onBack: () => void;
  onSaved: () => Promise<void>;
  onMarkUpToDate: OnMarkUpToDate;
  onSendBackToWork: OnSendBackToWork;
}) {
  const t = useTranslations("submission");
  const compact = useCompact();
  // Set by Activity while the note textarea holds unsaved text, so dismissing
  // the panel (Escape, the mobile "←", closing the modal) can warn before it
  // discards a draft the chief editor never posted.
  const [hasDraft, setHasDraft] = useState(false);
  const confirmedBack = useCallback(() => {
    if (hasDraft && !window.confirm(t("discardNoteConfirm"))) return;
    onBack();
  }, [hasDraft, onBack, t]);

  useEffect(() => {
    if (compact || !submission) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") confirmedBack();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [compact, submission, confirmedBack]);

  const content = submission ? (
    <>
      <Header
        submission={submission}
        compact={compact}
        onBack={confirmedBack}
      />
      <Findings submission={submission} />
      <Activity
        key={submission.ojsSubmissionId}
        submission={submission}
        focusNote={focusNote}
        onDraftChange={setHasDraft}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
      <Curation
        key={`curation-${submission.ojsSubmissionId}`}
        submission={submission}
        conferences={conferences}
        onSaved={onSaved}
      />
      <Evidence submission={submission} />
    </>
  ) : null;

  if (compact)
    return (
      <AppModal
        isOpen={submission !== undefined}
        onOpenChange={(open) => !open && confirmedBack()}
        fullScreen
        dialogProps={{ "aria-label": t("section") }}
      >
        <div className="detail-panel">{content}</div>
      </AppModal>
    );

  return (
    <aside className="detail-panel" aria-label={t("section")}>
      {content}
    </aside>
  );
}

function Header({
  submission,
  compact,
  onBack,
}: {
  submission: Submission;
  compact: boolean;
  onBack: () => void;
}) {
  const t = useTranslations("submission");
  const tCommon = useTranslations("common");
  return (
    <header className="detail-header">
      {compact && (
        <Button
          aria-label={tCommon("close")}
          className="detail-panel__close"
          onPress={onBack}
        >
          <ArrowLeft size={16} />
        </Button>
      )}
      <div>
        <Heading level={2}>
          {submission.url ? (
            <a
              className="ojs-link"
              href={submission.url}
              target="_blank"
              rel="noopener noreferrer"
            >
              {submission.ojsSubmissionId}
              {/* The dash is a separator that slides in with the label. It is
                  decoration, so it stays out of the accessible name: a screen
                  reader must hear the submission ID and label without the dash. */}
              <span className="ojs-link__label">
                <span aria-hidden="true">-</span>
                {t("openInOJS")}
              </span>
              <svg
                className="ojs-link__icon"
                aria-hidden="true"
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                <polyline points="15 3 21 3 21 9" />
                <line x1="10" y1="14" x2="21" y2="3" />
              </svg>
            </a>
          ) : (
            submission.ojsSubmissionId
          )}
        </Heading>
        <p className="submission-title">{submission.title}</p>
      </div>
    </header>
  );
}

function Findings({ submission }: { submission: Submission }) {
  const t = useTranslations("submission");
  const findingWords = useWords("enums.attention");
  return (
    <section className="detail-section" aria-label={t("findingsSection")}>
      {submission.findings.length === 0 ? (
        <p className="readonly">{t("noFindings")}</p>
      ) : (
        <ul className="finding-badges">
          {submission.findings.map((finding, index) => (
            <li
              className="finding-badge"
              key={`${finding.kind}-${finding.deadline}-${index}`}
            >
              <TriangleAlert size={16} />{" "}
              {finding.deadline
                ? t("findingWithDeadline", {
                    finding: findingWords(finding.kind),
                    deadline: day(finding.deadline),
                  })
                : findingWords(finding.kind)}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function Activity({
  submission,
  focusNote,
  onDraftChange,
  onMarkUpToDate,
  onSendBackToWork,
}: {
  submission: Submission;
  focusNote: boolean;
  onDraftChange: (hasDraft: boolean) => void;
  onMarkUpToDate: OnMarkUpToDate;
  onSendBackToWork: OnSendBackToWork;
}) {
  const t = useTranslations("submission");
  const tCommon = useTranslations("common");
  const locale = useLocale();
  const statusWords = useWords("submission.deliveryStatus");
  const failureWords = useWords("submission.failureClass");
  const titleWords = useWords(
    "submission.activityTitle",
    "editorial_notification",
  );
  const recipientRoleWords = useWords("submission.activityRecipientRole");
  const ojsSubmissionId = submission.ojsSubmissionId;
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const [status, setStatus] = useSaveStatus();
  const noteRef = useRef<HTMLTextAreaElement>(null);

  const refresh = useCallback(async () => {
    try {
      setItems(await loadActivity(ojsSubmissionId));
    } catch {
      setError(t("activityFailed"));
    }
  }, [ojsSubmissionId, t]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (focusNote) noteRef.current?.focus();
  }, [focusNote]);

  useEffect(() => {
    onDraftChange(draft.trim() !== "");
  }, [draft, onDraftChange]);
  // A remounted Activity (a new submission opened) must not leave the parent
  // thinking the previous submission's draft is still pending.
  useEffect(() => () => onDraftChange(false), [onDraftChange]);

  const append = async () => {
    const body = draft.trim();
    if (!body) return;
    setStatus("loading");
    try {
      await appendJournalEntry(ojsSubmissionId, body);
      setDraft("");
      setError("");
      await refresh();
      setStatus("success");
    } catch {
      setError(t("noteFailed"));
      setStatus("error");
    }
  };

  return (
    <section className="detail-section" aria-label={t("activitySection")}>
      <Heading level={3}>{t("activitySection")}</Heading>
      {items.length === 0 ? (
        <p className="readonly activity-empty">
          <Pencil size={16} aria-hidden="true" /> {t("activityEmpty")}
        </p>
      ) : (
        <ol className="activity">
          {items.map((item, index) => (
            <ActivityEntry
              key={`${item.occurredAt}-${item.kind}-${index}`}
              item={item}
              locale={locale}
              t={t}
              statusWords={statusWords}
              failureWords={failureWords}
              titleWords={titleWords}
              recipientRoleWords={recipientRoleWords}
            />
          ))}
        </ol>
      )}
      <div className="note-box">
        <TextField value={draft} onChange={setDraft}>
          <Label className="-screenReader">{t("noteLabel")}</Label>
          <TextArea
            ref={noteRef}
            placeholder={t("noteLabel")}
            rows={3}
            onKeyDown={(event) => {
              if (event.key === "Enter" && event.ctrlKey) {
                event.preventDefault();
                void append();
              }
            }}
          />
        </TextField>
        <div className="note-actions">
          <div className="note-actions__left">
            <SaveStatus
              status={status}
              savingLabel={tCommon("saving")}
              savedLabel={tCommon("saved")}
            />
            {submission.effectiveQueue === "up_to_date" ? (
              <SendBackToWorkButton
                submission={submission}
                onSendBackToWork={onSendBackToWork}
              />
            ) : (
              <UpToDateSplitButton
                submission={submission}
                onMarkUpToDate={onMarkUpToDate}
              />
            )}
          </div>
          <TooltipTrigger delay={0}>
            <Button
              onPress={() => void append()}
              isDisabled={status === "loading"}
            >
              {t("noteAdd")}
            </Button>
            <Tooltip className="tooltip">{t("noteImmutable")}</Tooltip>
          </TooltipTrigger>
        </div>
      </div>
      {error && (
        <p className="failure" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}

function ActivityEntry({
  item,
  locale,
  t,
  statusWords,
  failureWords,
  titleWords,
  recipientRoleWords,
}: {
  item: ActivityItem;
  locale: string;
  t: ReturnType<typeof useTranslations<"submission">>;
  statusWords: (value: string) => string;
  failureWords: (value: string) => string;
  titleWords: (value: string) => string;
  recipientRoleWords: (value: string) => string;
}) {
  const isJournal = item.kind === "journal";
  const failed = item.status !== "sent";
  const Icon = isJournal ? Pencil : failed ? MailX : Mail;
  const date = new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
  }).format(new Date(item.occurredAt));
  const status = statusWords(item.status ?? "");
  const detail = failed
    ? `${status} · ${failureWords(item.failureClass || "unknown")}`
    : status;
  return (
    <li className={`activity-entry${isJournal ? "" : " activity-entry--mail"}`}>
      <span className="activity-body">
        <TooltipTrigger delay={0}>
          <Button
            className="activity-mark"
            data-kind={item.kind}
            data-failed={!isJournal && failed}
            aria-label={
              isJournal
                ? t("activityJournalType")
                : t("activityStatusFor", { status })
            }
          >
            <Icon size={18} aria-hidden="true" />
          </Button>
          <Tooltip className="tooltip">
            {isJournal ? t("activityJournalType") : detail}
          </Tooltip>
        </TooltipTrigger>
        <span className="activity-entry-detail">
          <span>
            {isJournal
              ? item.body
              : titleWords(item.titleKey ?? "editorial_notification")}
          </span>
          <span className="activity-when">{date}</span>
          {isJournal ? (
            <span className="activity-author">{item.authorLabel}</span>
          ) : (
            item.recipient && (
              <span className="activity-recipient">
                {t("activityTo")} {item.recipient}
                {item.recipientRole &&
                  t.has(`activityRecipientRole.${item.recipientRole}`) &&
                  ` · ${recipientRoleWords(item.recipientRole)}`}
              </span>
            )
          )}
        </span>
      </span>
    </li>
  );
}

function Curation({
  submission,
  conferences,
  onSaved,
}: {
  submission: Submission;
  conferences: Conference[];
  onSaved: () => Promise<void>;
}) {
  const t = useTranslations("submission");
  const tCommon = useTranslations("common");
  const [values, setValues] = useState({
    conference: submission.tracking?.conference ?? "",
  });
  // Mute is not part of the curation payload: the panel and the Muted screen
  // sit side by side, so `muted` can change under an open form, and a
  // curation save must never resend a value the user did not touch here.
  // It keeps its own local state, seeded the same way, purely to drive the
  // checkbox.
  const [muted, setMutedState] = useState(submission.tracking?.muted ?? false);
  const [error, setError] = useState("");
  const [status, setStatus] = useSaveStatus();
  const [pickerOpen, setPickerOpen] = useState(false);
  // A tap does not focus a button as keyboard-visibly as a Tab press does, so
  // the tooltip's own hover/focus triggers miss it. Pressing the info
  // control opens it directly, on top of the same hover/focus triggers.
  const [infoOpen, setInfoOpen] = useState(false);
  // A submission has at most one tracking row, so two changes made before the
  // first save returns must not both create one. The saves run in a queue, and
  // the id of the created row is carried to the next one.
  const trackingID = useRef(submission.tracking?.id);
  const queue = useRef<Promise<unknown>>(Promise.resolve());
  const writeTracking = (next: typeof values) =>
    saveTracking(trackingID.current ?? submission.tracking?.id, {
      ojsSubmissionId: submission.ojsSubmissionId,
      ...next,
    }).then((saved) => {
      trackingID.current = saved.id;
      return saved.id;
    });
  // The checkbox is a direct, legitimate way to set `muted`, on this page as
  // well as on the Muted screen: it goes straight to `setMuted`, never
  // through the curation payload.
  const mute = (next: boolean) => {
    setMutedState(next);
    setStatus("loading");
    queue.current = queue.current
      .then(async () => {
        await setMuted(trackingID.current ?? submission.tracking?.id, next);
        setError("");
        setStatus("success");
        await onSaved();
      })
      .catch(() => {
        setError(t("curationFailed"));
        setStatus("error");
      });
  };
  const assign = (conference: string) => {
    // Muting is the decision the conference belongs to, so associating one
    // proposes the mute and detaching one withdraws it. That is two writes
    // now that mute has its own writer: the curation save runs first, so a
    // failed mute write leaves the submission visible on the work list with
    // an error shown, rather than muting it without the conference association
    // that explains why it vanished.
    const next = { ...values, conference };
    setMutedState(conference !== "");
    setValues(next);
    setStatus("loading");
    queue.current = queue.current
      .then(async () => {
        const id = await writeTracking(next);
        await setMuted(id, conference !== "");
        setError("");
        setStatus("success");
        await onSaved();
      })
      .catch(() => {
        setError(t("curationFailed"));
        setStatus("error");
      });
    setPickerOpen(false);
  };
  const createAndAssign = async (name: string, description: string) => {
    const created = await saveConference(undefined, name, description);
    assign(created.id);
  };
  const conference = conferences.find((item) => item.id === values.conference);
  return (
    <section className="detail-section" aria-label={t("curationSection")}>
      <Heading level={3}>{t("curationSection")}</Heading>
      <div className="conference-field">
        <div className="conference-field__label">
          <TooltipTrigger delay={0}>
            <Button
              className="conference-field__label-text"
              aria-label={t("conferenceInfo")}
            >
              {t("conference")}
            </Button>
            <Tooltip className="tooltip">{t("conferenceTooltip")}</Tooltip>
          </TooltipTrigger>
          <TooltipTrigger
            delay={0}
            isOpen={infoOpen}
            onOpenChange={setInfoOpen}
          >
            <Button
              className="conference-field__info"
              aria-label={t("conferenceInfo")}
              onPress={() => setInfoOpen(true)}
            >
              <Info size={16} aria-hidden="true" />
            </Button>
            <Tooltip className="tooltip">{t("conferenceTooltip")}</Tooltip>
          </TooltipTrigger>
        </div>
        <div className="conference-field__association">
          <strong className="conference-field__name">
            {conference ? conference.name : t("noConference")}
          </strong>
          {submission.effectiveQueue !== "archived" && (
            <Button
              className="conference-field__trigger"
              onPress={() => setPickerOpen(true)}
            >
              {t(conference ? "changeConference" : "assignConference")}
            </Button>
          )}
        </div>
        {/* Clearing the box is the unmute control, on this page as well as on
            the Muted screen. */}
        {(values.conference || muted) && (
          <CheckboxField
            className="mute-field"
            isSelected={muted}
            onChange={mute}
          >
            <CheckboxButton>{t("mute")}</CheckboxButton>
          </CheckboxField>
        )}
        {muted && <p className="readonly">{t("mutedNote")}</p>}
      </div>
      <SaveStatus
        status={status}
        savingLabel={tCommon("saving")}
        savedLabel={tCommon("saved")}
      />
      {error && (
        <p className="failure" role="alert">
          {error}
        </p>
      )}
      <ConferencePicker
        isOpen={pickerOpen}
        onOpenChange={setPickerOpen}
        conferences={conferences}
        currentConferenceName={conference?.name ?? ""}
        onAssign={assign}
        onCreate={createAndAssign}
      />
    </section>
  );
}

const conferencePageSize = 20;

function ConferencePicker({
  isOpen,
  onOpenChange,
  conferences,
  currentConferenceName,
  onAssign,
  onCreate,
}: {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  conferences: Conference[];
  currentConferenceName: string;
  onAssign: (id: string) => void;
  onCreate: (name: string, description: string) => Promise<void>;
}) {
  const t = useTranslations("submission");
  const [step, setStep] = useState<"list" | "create">("list");
  const [search, setSearch] = useState("");
  const [visible, setVisible] = useState(conferencePageSize);

  useEffect(() => {
    if (!isOpen) return;
    setStep("list");
    setSearch("");
    setVisible(conferencePageSize);
  }, [isOpen]);

  const needle = search.trim().toLowerCase();
  const filtered = conferences.filter((conference) =>
    conference.name.toLowerCase().includes(needle),
  );
  const page = filtered.slice(0, visible);

  return (
    <AppModal
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      dialogProps={{ "aria-label": t("conference") }}
    >
      <p>{t("conferenceAssignmentEffect")}</p>
      {step === "list" ? (
        <>
          <Heading slot="title">{t("conference")}</Heading>
          {currentConferenceName && (
            <Button className="unassign-button" onPress={() => onAssign("")}>
              <Unlink size={16} aria-hidden="true" />
              {t("unassign")}
            </Button>
          )}
          <SearchField
            className="search"
            value={search}
            onChange={(value) => {
              setSearch(value);
              setVisible(conferencePageSize);
            }}
          >
            <Label className="-screenReader">
              {t("conferenceSearchLabel")}
            </Label>
            <Input placeholder={t("conferenceSearchPlaceholder")} />
            <Button>
              <span aria-hidden="true">✕</span>
            </Button>
          </SearchField>
          <ul className="conference-list">
            {page.map((conference) => (
              <li key={conference.id}>
                <Button onPress={() => onAssign(conference.id)}>
                  {conference.name}
                </Button>
              </li>
            ))}
          </ul>
          {filtered.length > page.length && (
            <Button
              onPress={() => setVisible((count) => count + conferencePageSize)}
            >
              {t("loadMoreConferences")}
            </Button>
          )}
          <Button onPress={() => setStep("create")}>
            {t("createConference")}
          </Button>
        </>
      ) : (
        <ConferenceCreateForm
          onBack={() => setStep("list")}
          onCreate={onCreate}
        />
      )}
    </AppModal>
  );
}

function ConferenceCreateForm({
  onBack,
  onCreate,
}: {
  onBack: () => void;
  onCreate: (name: string, description: string) => Promise<void>;
}) {
  const t = useTranslations("conferencesForm");
  const tCommon = useTranslations("common");
  const draft = useConferenceDraft();
  return (
    <Form
      validationBehavior="aria"
      onSubmit={async (event) => {
        event.preventDefault();
        draft.start();
        try {
          await onCreate(draft.name, draft.description);
        } catch {
          draft.fail(t("saveFailed"));
        }
      }}
    >
      <Heading slot="title">{t("title")}</Heading>
      <ConferenceFields draft={draft} />
      <Button type="submit" isDisabled={draft.status === "loading"}>
        {draft.status === "loading" ? tCommon("saving") : t("create")}
      </Button>
      <Button onPress={onBack}>{tCommon("back")}</Button>
    </Form>
  );
}

function Evidence({ submission }: { submission: Submission }) {
  const t = useTranslations("submission");
  const tCommon = useTranslations("common");
  return (
    <section className="detail-section" aria-label={t("evidenceSection")}>
      <Heading level={3}>{t("evidenceSection")}</Heading>
      <dl>
        <dt>{t("status")}</dt>
        <dd>{submission.status || tCommon("unknown")}</dd>
        <dt>{t("editors")}</dt>
        <dd>{submission.editors.join(", ") || tCommon("unavailable")}</dd>
        <dt>{t("submitted")}</dt>
        <dd>{submission.dateSubmitted || tCommon("unavailable")}</dd>
        <dt>{t("lastModified")}</dt>
        <dd>{submission.lastModified || tCommon("unavailable")}</dd>
        <dt>{t("reviews")}</dt>
        <dd>
          <ul className="evidence-list">
            {submission.reviews.map((review, index) => (
              <li key={`${review.round}-${review.reviewDeadline}-${index}`}>
                {t("reviewLine", {
                  round: review.round,
                  status: review.status || tCommon("unavailable"),
                  response: review.responseDeadline
                    ? day(review.responseDeadline)
                    : tCommon("unavailable"),
                  review: review.reviewDeadline
                    ? day(review.reviewDeadline)
                    : tCommon("unavailable"),
                })}
              </li>
            ))}
          </ul>
        </dd>
        <dt>{t("decisions")}</dt>
        <dd>
          <ul className="evidence-list">
            {submission.decisions.map((decision, index) => (
              <li key={`${decision.decision}-${decision.date}-${index}`}>
                {t("decisionLine", {
                  decision: decision.decision || tCommon("unavailable"),
                  date: decision.date || tCommon("unavailable"),
                })}
              </li>
            ))}
          </ul>
        </dd>
      </dl>
    </section>
  );
}
