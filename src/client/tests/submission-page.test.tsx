import { readFileSync } from "node:fs";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ReactNode, useEffect } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import { ClientProviders, useSetLocale } from "../src/app/provider";
import { SubmissionPage } from "../src/components/submission-page";
import type { ActivityItem, Submission } from "../src/lib/monitoring";

const {
  appendJournalEntry,
  loadActivity,
  saveConference,
  saveTracking,
  setMuted,
} = vi.hoisted(() => ({
  appendJournalEntry: vi.fn(),
  loadActivity: vi.fn(),
  saveConference: vi.fn(),
  saveTracking: vi.fn(),
  setMuted: vi.fn(),
}));

vi.mock("../src/lib/api", () => ({
  appendJournalEntry,
  loadActivity,
  saveConference,
  saveTracking,
  setMuted,
}));

const submission: Submission = {
  ojsSubmissionId: "600387",
  title: "Synthetic Review Article",
  status: "review",
  url: "https://ojs.example.test/600387",
  dateSubmitted: "2025-02-14",
  lastModified: "2025-06-16",
  editors: ["Taylor Brooks"],
  decisions: [],
  reviews: [],
  findings: [{ kind: "missing_editor", responsible: "", deadline: "" }],
  findingDigest: "missing_editor",
  effectiveFinalState: "",
  effectiveQueue: "daily",
  effectiveAttention: "missing_editor",
  provenance: {},
};

const conferences = [
  { id: "conf-1", name: "SBQS 2025", description: "" },
  { id: "conf-2", name: "ERGATS", description: "" },
];

const onMarkUpToDate = vi.fn(async () => {});
const onSendBackToWork = vi.fn(async () => {});

const renderPage = (focusNote = false) =>
  render(
    <ClientProviders>
      <SubmissionPage
        submission={submission}
        conferences={conferences}
        focusNote={focusNote}
        onBack={() => {}}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );

function PortugueseLocale({ children }: { children: ReactNode }) {
  const setLocale = useSetLocale();
  useEffect(() => setLocale("pt-BR"), [setLocale]);
  return children;
}

beforeEach(() => {
  vi.clearAllMocks();
  loadActivity.mockResolvedValue([]);
  appendJournalEntry.mockResolvedValue({});
  saveTracking.mockResolvedValue({ id: "t1" });
  setMuted.mockResolvedValue(undefined);
});

it.each([
  ["journal_editor", "Journal editor"],
  ["associate_editor", "Associate editor"],
  ["reviewer", "Reviewer"],
  ["editor", "Assigned editor"],
])(
  "shows the recipient's %s role beside their address",
  async (role, label) => {
    loadActivity.mockResolvedValue([
      {
        kind: "delivery",
        occurredAt: "2025-05-21T09:00:00Z",
        titleKey: "editorial_notification",
        status: "sent",
        recipient: "editor@example.test",
        recipientRole: role as ActivityItem["recipientRole"],
      } satisfies ActivityItem,
    ]);
    renderPage();

    const recipient = await screen.findByText(/editor@example.test/);
    expect(recipient.textContent).toContain(label);
  },
);

it("shows a failed delivery as a mail error and keeps the finding warning", async () => {
  const failed: ActivityItem = {
    kind: "delivery",
    occurredAt: "2025-05-21T09:00:00Z",
    titleKey: "review_invitation_answered",
    status: "failed",
    failureClass: "smtp_permanent_failure",
    recipient: "reviewer@example.test",
  };
  loadActivity.mockResolvedValue([failed]);
  renderPage();

  const finding = await screen.findByText(/No editor/);
  expect(
    finding.closest("li")?.querySelector(".lucide-triangle-alert"),
  ).toBeDefined();

  const delivery = await screen.findByText(
    "The review invitation was answered.",
  );
  expect(
    delivery.closest(".activity-body")?.querySelector(".lucide-mail-x"),
  ).toBeDefined();
});

it("renders the lucide note icon in place of the empty-activity and journal-entry glyphs", async () => {
  renderPage();
  const empty = await screen.findByText("No message and no note yet.");
  expect(empty.querySelector(".lucide-pencil")).toBeDefined();

  loadActivity.mockResolvedValue([
    {
      kind: "journal",
      occurredAt: "2025-06-22T10:00:00Z",
      authorLabel: "morgan@example.test",
      body: "Assigned to Robin.",
    } satisfies ActivityItem,
  ]);
  const user = userEvent.setup();
  const box = await screen.findByRole("textbox", { name: "Write a note…" });
  await user.click(box);
  await user.keyboard("Assigned to Robin.{Control>}{Enter}{/Control}");

  const author = await screen.findByText("morgan@example.test");
  const journalDetails = author.closest(".activity-entry-detail");
  expect(journalDetails?.contains(screen.getByText("Assigned to Robin."))).toBe(
    true,
  );
  expect(journalDetails?.querySelector(".activity-when")?.textContent).toBe(
    new Intl.DateTimeFormat("en", { dateStyle: "medium" }).format(
      new Date("2025-06-22T10:00:00Z"),
    ),
  );
  expect(journalDetails?.querySelector(".activity-author")).toBe(author);
  expect(journalDetails?.querySelector(".activity-recipient")).toBeNull();
  expect(journalDetails?.textContent).not.toContain("To ");
  expect(
    author.closest(".activity-body")?.querySelector(".lucide-pencil"),
  ).toBeDefined();
  const journalMark = screen.getByRole("button", { name: "Journal entry" });
  journalMark.focus();
  expect(
    await screen.findByRole("tooltip", { name: "Journal entry" }),
  ).toBeDefined();
});

it("renders the lucide back-arrow icon in the compact panel's close button", async () => {
  const originalMatchMedia = window.matchMedia;
  window.matchMedia = (query: string) =>
    ({
      matches: query === "(max-width: 767px)",
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList;
  try {
    renderPage();
    expect(
      (await screen.findByRole("button", { name: "Close" })).querySelector(
        ".lucide-arrow-left",
      ),
    ).toBeDefined();
  } finally {
    window.matchMedia = originalMatchMedia;
  }
});

it("shows a localized title and status tooltip without opening a preview", async () => {
  const failed: ActivityItem = {
    kind: "delivery",
    occurredAt: "2025-05-21T09:00:00Z",
    titleKey: "review_invitation_answered",
    recipientRole: "reviewer",
    status: "failed",
    failureClass: "smtp_permanent_failure",
    recipient: "reviewer@example.test",
  };
  loadActivity.mockResolvedValue([failed]);
  renderPage();

  const icon = await screen.findByRole("button", {
    name: "Email status: failed",
  });
  expect(screen.queryByText(/Email body unavailable/)).toBeNull();
  icon.focus();
  expect(
    await screen.findByRole("tooltip", {
      name: /failed · The mail server rejected the message and will not retry\./,
    }),
  ).toBeDefined();
  expect(screen.getByText("The review invitation was answered.")).toBeDefined();
  expect(screen.getByText("To reviewer@example.test · Reviewer")).toBeDefined();
  await userEvent.setup().click(icon);
  expect(screen.queryByRole("dialog")).toBeNull();
});

it.each([undefined, "future_value"])(
  "uses neutral fallbacks for unsupported metadata: %s",
  async (value) => {
    loadActivity.mockResolvedValue([
      {
        kind: "delivery",
        occurredAt: "2025-05-21T09:00:00Z",
        titleKey: value,
        recipientRole: value,
        status: "blocked",
        failureClass: "recipient_blocked",
        recipient: "reviewer@example.test",
      },
    ]);
    renderPage();
    expect(await screen.findByText("Editorial notification.")).toBeDefined();
    expect(screen.getByText("To reviewer@example.test")).toBeDefined();
  },
);

it("has English and Portuguese titles for every activity key", () => {
  const keys = [
    "review_invitation_answered",
    "review_invitation_expired",
    "review_submitted",
    "all_reviews_completed",
    "revised_file_hint",
    "review_invitation_reminder",
    "review_deadline_reminder",
    "review_deadline_expired",
    "missing_copyeditor",
    "welcome",
    "editorial_notification",
  ];
  for (const locale of ["en", "pt-BR"]) {
    const messages = JSON.parse(
      readFileSync(`messages/${locale}.json`, "utf8"),
    );
    expect(Object.keys(messages.submission.activityTitle).sort()).toEqual(
      [...keys].sort(),
    );
  }
});

it("uses the selected locale for date-only activity dates and titles", async () => {
  const occurredAt = "2025-05-21T09:00:00Z";
  loadActivity.mockResolvedValue([
    {
      kind: "delivery",
      occurredAt,
      titleKey: "welcome",
      status: "sent",
      recipientRole: "editor",
      recipient: "reviewer@example.test",
    } satisfies ActivityItem,
  ]);
  render(
    <ClientProviders>
      <PortugueseLocale>
        <SubmissionPage
          submission={submission}
          conferences={conferences}
          focusNote={false}
          onBack={() => {}}
          onSaved={async () => {}}
          onMarkUpToDate={onMarkUpToDate}
          onSendBackToWork={onSendBackToWork}
        />
      </PortugueseLocale>
    </ClientProviders>,
  );
  expect(
    await screen.findByText(
      new Intl.DateTimeFormat("pt-BR", {
        dateStyle: "medium",
      }).format(new Date(occurredAt)),
    ),
  ).toBeDefined();
  expect(screen.getByText("E-mail de boas-vindas.")).toBeDefined();
  expect(
    screen.getByText("Para reviewer@example.test · Editor responsável"),
  ).toBeDefined();
});

it("appends a note with Ctrl+Enter and keeps it read-only", async () => {
  const user = userEvent.setup();
  renderPage();
  const box = await screen.findByRole("textbox", { name: "Write a note…" });

  await user.click(box);
  await user.keyboard("Assigned to Robin.");
  loadActivity.mockResolvedValue([
    {
      kind: "journal",
      occurredAt: "2025-06-22T10:00:00Z",
      authorLabel: "morgan@example.test",
      body: "Assigned to Robin.",
    } satisfies ActivityItem,
  ]);
  await user.keyboard("{Control>}{Enter}{/Control}");

  expect(appendJournalEntry).toHaveBeenCalledWith(
    "600387",
    "Assigned to Robin.",
  );
  const entry = await screen.findByText("Assigned to Robin.");
  // The saved entry is text in the timeline, never a form control, and no
  // control offers to change or drop it.
  expect(entry.tagName).toBe("SPAN");
  expect(screen.queryAllByRole("textbox")).toHaveLength(1);
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("");
  const activitySection = within(entry.closest("section") as HTMLElement);
  for (const name of [/edit/i, /delete/i, /remove/i])
    expect(activitySection.queryByRole("button", { name })).toBeNull();
});

it("renders the mark-up-to-date action left of Post, with tooltips on both icons", async () => {
  const user = userEvent.setup();
  renderPage();
  const activitySection = (
    await screen.findByRole("heading", { name: "Activity" })
  ).closest("section") as HTMLElement;
  const section = within(activitySection);

  const post = section.getByRole("button", { name: "Post" });
  const checkmark = section.getByRole("button", { name: "Mark up to date" });
  const caret = section.getByRole("button", {
    name: "More up to date options",
  });
  expect(
    checkmark.compareDocumentPosition(post) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();

  checkmark.focus();
  expect(
    await screen.findByRole("tooltip", { name: "Mark up to date" }),
  ).toBeDefined();

  caret.focus();
  expect(
    await screen.findByRole("tooltip", { name: "More options" }),
  ).toBeDefined();

  await user.click(checkmark);
  expect(onMarkUpToDate).toHaveBeenCalledWith(
    expect.objectContaining({ ojsSubmissionId: "600387" }),
    "",
    "",
  );
});

it("offers send-back-to-work only for a submission that is up to date", async () => {
  const user = userEvent.setup();
  render(
    <ClientProviders>
      <SubmissionPage
        submission={{ ...submission, effectiveQueue: "up_to_date" }}
        conferences={conferences}
        focusNote={false}
        onBack={() => {}}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );
  const activitySection = (
    await screen.findByRole("heading", { name: "Activity" })
  ).closest("section") as HTMLElement;
  const button = within(activitySection).getByRole("button", {
    name: "Send back to work",
  });
  await user.click(button);
  expect(onSendBackToWork).toHaveBeenCalledWith(
    expect.objectContaining({ ojsSubmissionId: "600387" }),
  );
});

it("hides send-back-to-work for a submission that is not up to date", async () => {
  renderPage();
  await screen.findByRole("heading", { name: "Activity" });
  expect(
    screen.queryByRole("button", { name: "Send back to work" }),
  ).toBeNull();
});

it("shows an overdue-review finding's deadline as a date only, with no time", async () => {
  const withDeadline: Submission = {
    ...submission,
    findings: [
      {
        kind: "overdue_review",
        responsible: "editor",
        deadline: "2026-08-26T23:59:59Z",
      },
    ],
  };
  render(
    <ClientProviders>
      <SubmissionPage
        submission={withDeadline}
        conferences={conferences}
        focusNote={false}
        onBack={() => {}}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );
  const finding = await screen.findByText(/Overdue review/);
  expect(finding.textContent).toContain("2026-08-26");
  expect(finding.textContent).not.toContain("23:59:59");
});

it("focuses the note box when the page is asked to", async () => {
  renderPage(true);
  const box = await screen.findByRole("textbox", { name: "Write a note…" });
  await waitFor(() => expect(document.activeElement).toBe(box));
});

it("keeps every rule code the server knows out of the page", async () => {
  // The list comes from the server's own constants, so a new rule is covered
  // the day it is added instead of the day someone remembers this test.
  const source = readFileSync("../server/internal/events/events.go", "utf8");
  const codes = [...source.matchAll(/\tRule\w+ = "([^"]+)"/g)].map(
    (match) => match[1],
  );
  expect(codes.length).toBeGreaterThan(0);

  loadActivity.mockResolvedValue(
    codes.map((_, index) => ({
      kind: "delivery" as const,
      occurredAt: `2025-06-${String(index + 1).padStart(2, "0")}T09:00:00Z`,
      titleKey: "review_deadline_reminder",
      status: "sent",
      recipient: "reviewer@example.test",
    })),
  );
  renderPage();
  await screen.findAllByText(
    "The deadline to submit the review is approaching.",
  );

  const text = document.body.textContent ?? "";
  for (const code of codes)
    expect({ code, present: text.includes(code) }).toEqual({
      code,
      present: false,
    });
});

it("shows no State or Decision date control", async () => {
  renderPage();
  await screen.findByRole("heading", { name: "Curation" });
  expect(screen.queryByLabelText("State")).toBeNull();
  expect(screen.queryByLabelText("Decision date")).toBeNull();
});

it("offers the mute checkbox only once a conference is chosen from the picker", async () => {
  const user = userEvent.setup();
  renderPage();
  expect(screen.queryByRole("checkbox")).toBeNull();

  await user.click(screen.getByRole("button", { name: "Assign conference" }));
  await user.click(await screen.findByRole("button", { name: "SBQS 2025" }));

  // The curation write carries no `muted`: mute has its own writer.
  await waitFor(() =>
    expect(saveTracking).toHaveBeenCalledWith(undefined, {
      ojsSubmissionId: "600387",
      conference: "conf-1",
    }),
  );
  // Associating a conference proposes the mute, once the curation write that
  // created the tracking row has returned its id.
  await waitFor(() => expect(setMuted).toHaveBeenCalledWith("t1", true));
  const checkbox = await screen.findByRole("checkbox", {
    name: "Mute this submission",
  });
  expect((checkbox as HTMLInputElement).checked).toBe(true);
});

// The checkbox itself is still a legitimate, direct way to change `muted`.
it("still unmutes from the checkbox itself", async () => {
  const user = userEvent.setup();
  const muted: Submission = {
    ...submission,
    tracking: {
      id: "t1",
      conference: "conf-1",
      muted: true,
      mutedDigest: "digest",
      upToDateAt: "",
      upToDateUntil: "",
      created: "",
      updated: "",
    },
  };
  render(
    <ClientProviders>
      <SubmissionPage
        submission={muted}
        conferences={conferences}
        focusNote={false}
        onBack={() => {}}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );

  const checkbox = await screen.findByRole("checkbox", {
    name: "Mute this submission",
  });
  await user.click(checkbox);

  await waitFor(() => expect(setMuted).toHaveBeenCalledWith("t1", false));
  expect(saveTracking).not.toHaveBeenCalled();
});

it("shows all conferences when changing an assignment, filters typed text, and clears only the search", async () => {
  const user = userEvent.setup();
  const withConference: Submission = {
    ...submission,
    tracking: {
      id: "t1",
      conference: "conf-1",
      muted: false,
      mutedDigest: "",
      upToDateAt: "",
      upToDateUntil: "",
      created: "",
      updated: "",
    },
  };
  render(
    <ClientProviders>
      <SubmissionPage
        submission={withConference}
        conferences={conferences}
        focusNote={false}
        onBack={() => {}}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );

  await user.click(screen.getByRole("button", { name: "Change" }));
  const search = (await screen.findByRole("searchbox", {
    name: "Search conferences",
  })) as HTMLInputElement;
  expect(search.value).toBe("");
  expect(screen.getByRole("button", { name: "ERGATS" })).toBeDefined();
  await user.type(search, "SBQS");
  expect(screen.getByRole("button", { name: "SBQS 2025" })).toBeDefined();
  expect(screen.queryByRole("button", { name: "ERGATS" })).toBeNull();

  await user.click(screen.getByRole("button", { name: "Clear search" }));
  expect(search.value).toBe("");
  expect(screen.getByRole("button", { name: "ERGATS" })).toBeDefined();
  // Clearing the search box never detaches the submission's assignment.
  expect(saveTracking).not.toHaveBeenCalled();
  expect(setMuted).not.toHaveBeenCalled();
});

it("creates a new conference from the picker and assigns it immediately", async () => {
  const user = userEvent.setup();
  saveConference.mockResolvedValue({
    id: "conf-3",
    name: "New Conference",
    description: "",
  });
  renderPage();

  await user.click(screen.getByRole("button", { name: "Assign conference" }));
  await user.click(
    await screen.findByRole("button", {
      name: "Create a new conference",
    }),
  );
  await user.type(
    screen.getByRole("textbox", { name: "Name" }),
    "New Conference",
  );
  await user.click(screen.getByRole("button", { name: "Create conference" }));

  await waitFor(() =>
    expect(saveConference).toHaveBeenCalledWith(
      undefined,
      "New Conference",
      "",
    ),
  );
  await waitFor(() =>
    expect(saveTracking).toHaveBeenCalledWith(undefined, {
      ojsSubmissionId: "600387",
      conference: "conf-3",
    }),
  );
  await waitFor(() => expect(setMuted).toHaveBeenCalledWith("t1", true));
});

it("opens the conference tooltip from the label hover and from the info-icon tap, with the old marker pill gone", async () => {
  const user = userEvent.setup();
  renderPage();
  await screen.findByRole("heading", { name: "Curation" });

  expect(screen.queryByRole("button", { name: "JSERD" })).toBeNull();

  // The label and the ⓘ icon are two separate triggers for the same
  // tooltip, in that DOM order.
  const [label, info] = screen.getAllByRole("button", {
    name: "About the conference field",
  });
  const tooltipText =
    "Conference associations are managed in JSERD Monitoring separately from OJS.";

  // Hovering the underlined label opens it.
  await user.hover(label);
  expect(
    await screen.findByRole("tooltip", { name: tooltipText }),
  ).toBeDefined();
  await user.unhover(label);

  // Tapping the info icon opens it too.
  await user.click(info);
  expect(
    await screen.findByRole("tooltip", { name: tooltipText }),
  ).toBeDefined();
});

it("shows Unassign only once a conference is assigned, and it clears the association without confirmation", async () => {
  const user = userEvent.setup();
  const confirmSpy = vi.spyOn(window, "confirm");
  const withConference: Submission = {
    ...submission,
    tracking: {
      id: "t1",
      conference: "conf-1",
      muted: true,
      mutedDigest: "digest",
      upToDateAt: "",
      upToDateUntil: "",
      created: "",
      updated: "",
    },
  };
  render(
    <ClientProviders>
      <SubmissionPage
        submission={withConference}
        conferences={conferences}
        focusNote={false}
        onBack={() => {}}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );

  expect(screen.queryByRole("button", { name: "Unassign" })).toBeNull();

  await user.click(screen.getByRole("button", { name: "Change" }));
  const unassign = await screen.findByRole("button", { name: "Unassign" });
  await user.click(unassign);

  await waitFor(() =>
    expect(saveTracking).toHaveBeenCalledWith("t1", {
      ojsSubmissionId: "600387",
      conference: "",
    }),
  );
  await waitFor(() => expect(setMuted).toHaveBeenCalledWith("t1", false));
  expect(confirmSpy).not.toHaveBeenCalled();
  confirmSpy.mockRestore();
});

it("does not show Unassign in the picker when no conference is assigned", async () => {
  const user = userEvent.setup();
  renderPage();

  await user.click(screen.getByRole("button", { name: "Assign conference" }));
  await screen.findByRole("heading", { name: "Conference" });
  expect(screen.queryByRole("button", { name: "Unassign" })).toBeNull();
});

it("asks for confirmation before Escape discards an unsaved note, and respects the answer", async () => {
  const user = userEvent.setup();
  const onBack = vi.fn();
  const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
  render(
    <ClientProviders>
      <SubmissionPage
        submission={submission}
        conferences={conferences}
        focusNote={false}
        onBack={onBack}
        onSaved={async () => {}}
        onMarkUpToDate={onMarkUpToDate}
        onSendBackToWork={onSendBackToWork}
      />
    </ClientProviders>,
  );
  const box = await screen.findByRole("textbox", { name: "Write a note…" });
  await user.click(box);
  await user.keyboard("Draft in progress");

  await user.keyboard("{Escape}");
  expect(confirmSpy).toHaveBeenCalledWith("The note is not saved. Discard it?");
  expect(onBack).not.toHaveBeenCalled();

  confirmSpy.mockReturnValue(true);
  await user.keyboard("{Escape}");
  expect(onBack).toHaveBeenCalledOnce();

  confirmSpy.mockRestore();
});
