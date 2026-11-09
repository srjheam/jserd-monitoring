import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ClientProviders } from "../src/app/provider";
import { MonitoringDashboard } from "../src/components/monitoring-dashboard";
import type {
  MonitoringMeta,
  Submission,
  SubmissionPage,
} from "../src/lib/monitoring";

// jsdom ships no IntersectionObserver, and the dashboard's scroll sentinel
// constructs one on every render once signed in. The stub never fires on its
// own; "Load more" is used instead to drive `loadMore` in these tests.
class IntersectionObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("IntersectionObserver", IntersectionObserverStub);

const {
  restoreSession,
  loadMonitoringMeta,
  loadSubmissionPage,
  loadSubmission,
  loadSideScreen,
  loadEditorFrequencies,
  markUpToDate,
  setMuted,
  saveConference,
  removeConference,
  appendJournalEntry,
  loadActivity,
  saveTracking,
} = vi.hoisted(() => ({
  restoreSession: vi.fn(),
  loadMonitoringMeta: vi.fn(),
  loadSubmissionPage: vi.fn(),
  loadSubmission: vi.fn(),
  loadSideScreen: vi.fn(),
  loadEditorFrequencies: vi.fn(),
  markUpToDate: vi.fn(),
  setMuted: vi.fn(),
  saveConference: vi.fn(),
  removeConference: vi.fn(),
  appendJournalEntry: vi.fn(),
  loadActivity: vi.fn(),
  saveTracking: vi.fn(),
}));

vi.mock("../src/lib/api", () => ({
  pb: {
    authStore: { record: { email: "chief@example.test" }, clear: vi.fn() },
  },
  restoreSession,
  loadMonitoringMeta,
  loadSubmissionPage,
  loadSubmission,
  loadSideScreen,
  loadEditorFrequencies,
  markUpToDate,
  setMuted,
  saveConference,
  removeConference,
  appendJournalEntry,
  loadActivity,
  saveTracking,
}));

const row = (
  id: string,
  title: string,
  effectiveQueue: Submission["effectiveQueue"] = "daily",
): Submission => ({
  ojsSubmissionId: id,
  title,
  status: "Queued",
  url: "",
  dateSubmitted: "2026-01-01",
  lastModified: "",
  editors: [],
  decisions: [],
  reviews: [],
  findings: [],
  effectiveFinalState: "",
  effectiveQueue,
  effectiveAttention: "",
  findingDigest: "digest",
  provenance: {},
});

const meta = (): MonitoringMeta => ({
  generatedAt: "",
  availability: "available",
  conferences: [{ id: "conf-1", name: "Conference One", description: "" }],
  aggregates: {
    metrics: {},
    triage: {
      daily: 0,
      archived: 0,
      muted: 0,
      up_to_date: 0,
    },
    dailyRequiringAttention: 0,
    statuses: [],
    editors: [],
    total: 0,
  },
});

const page = (items: Submission[], nextCursor = ""): SubmissionPage => ({
  items,
  nextCursor,
  total: items.length,
});

beforeEach(() => {
  vi.clearAllMocks();
  restoreSession.mockResolvedValue("signedIn");
  loadMonitoringMeta.mockResolvedValue(meta());
  loadSubmission.mockImplementation(async (id: string) =>
    row(id, id === "1" ? "Row One" : "Row Two"),
  );
  loadActivity.mockResolvedValue([]);
  saveTracking.mockResolvedValue({ id: "t1" });
  loadEditorFrequencies.mockResolvedValue([]);
});

afterEach(() => {
  window.history.replaceState(null, "", "/");
  restoreMatchMedia?.();
  restoreMatchMedia = undefined;
});

function renderDashboard() {
  return render(
    <ClientProviders>
      <MonitoringDashboard />
    </ClientProviders>,
  );
}

// Real matchMedia is stubbed desktop-only in tests/setup.ts; the mobile test
// below narrows the query itself and restores it afterward.
let restoreMatchMedia: (() => void) | undefined;

const setCompact = (matches: boolean) => {
  const original = window.matchMedia;
  window.matchMedia = (query: string) =>
    ({
      matches: query === "(max-width: 767px)" ? matches : false,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList;
  restoreMatchMedia = () => {
    window.matchMedia = original;
  };
};

it("focuses the note box when the note control is clicked in the same session", async () => {
  const user = userEvent.setup();
  loadSubmissionPage.mockResolvedValue(page([row("1", "Row One")]));
  renderDashboard();

  const note = await screen.findByRole("link", { name: "Add a note" });
  await user.click(note);

  const box = await screen.findByRole("textbox", { name: "Write a note…" });
  await waitFor(() => expect(document.activeElement).toBe(box));
});

it("does not steal focus into the note box on a plain click after a cold note load", async () => {
  const user = userEvent.setup();
  window.history.replaceState(null, "", "/?article=1&note=1");
  loadSubmissionPage.mockResolvedValue(page([row("1", "Row One")]));
  renderDashboard();

  // The cold-load path still works: opening with `note=1` focuses the box.
  const box = await screen.findByRole("textbox", { name: "Write a note…" });
  await waitFor(() => expect(document.activeElement).toBe(box));

  // Close the panel, then reopen the same row with a plain click, which asks
  // for no focus. The stale `note=1` read at mount must not resurrect it.
  await user.keyboard("{Escape}");
  await waitFor(() =>
    expect(screen.queryByRole("textbox", { name: "Write a note…" })).toBeNull(),
  );
  await user.click(screen.getByRole("link", { name: /Row One/ }));

  const reopened = await screen.findByRole("textbox", {
    name: "Write a note…",
  });
  await waitFor(() => expect(document.activeElement).not.toBe(reopened));
});

it("discards a load-more batch that resolves after a curation save has refreshed the list", async () => {
  const user = userEvent.setup();
  let callCount = 0;
  let resolveLoadMore!: (value: SubmissionPage) => void;
  let resolveRefresh!: (value: SubmissionPage) => void;

  loadSubmissionPage.mockImplementation(async () => {
    callCount += 1;
    if (callCount === 1) return page([row("1", "Row One")], "curA");
    if (callCount === 2)
      return new Promise<SubmissionPage>((resolve) => {
        resolveLoadMore = resolve;
      });
    if (callCount === 3)
      return new Promise<SubmissionPage>((resolve) => {
        resolveRefresh = resolve;
      });
    throw new Error(`unexpected loadSubmissionPage call ${callCount}`);
  });
  const { container } = renderDashboard();

  // Select the submission so the curation panel is on screen.
  await user.click(await screen.findByRole("link", { name: /Row One/ }));
  await screen.findByRole("heading", { name: "Curation" });

  // Start "load more": its fetch is now in flight.
  await user.click(screen.getByRole("button", { name: "Load more" }));
  await waitFor(() => expect(loadSubmissionPage).toHaveBeenCalledTimes(2));

  // A curation save starts a refresh while that fetch is still pending.
  await user.click(screen.getByRole("button", { name: "Assign conference" }));
  await user.click(
    await screen.findByRole("button", { name: "Conference One" }),
  );
  await waitFor(() => expect(loadSubmissionPage).toHaveBeenCalledTimes(3));

  // The refresh's own fetch resolves first...
  resolveRefresh(page([row("1", "Row One")], "curA-refreshed"));
  await waitFor(() => expect(saveTracking).toHaveBeenCalled());

  // ...then the superseded load-more batch resolves.
  resolveLoadMore(page([row("2", "Row Two")], "curB"));
  await waitFor(() => screen.getByRole("button", { name: "Load more" }));

  // The stale batch must not have been appended: only row 1 is on screen.
  const rowIDs = [...container.querySelectorAll(".row__id")].map((node) =>
    (node.textContent ?? "").replace("ID ", ""),
  );
  expect(rowIDs).toEqual(["1"]);
});

// The mirror of the test above, and the order it did not cover. A load-more
// that starts *while* a refresh is in flight carries a cursor encoded before
// the refresh's own rows moved, so its batch repeats the row that slid up.
// The ticket cannot catch it, because the refresh already took the newer
// ticket before this load-more read one, preventing duplicate rows.
it("refuses a load-more that starts while a curation refresh is in flight", async () => {
  const user = userEvent.setup();
  let callCount = 0;
  let resolveRefresh!: (value: SubmissionPage) => void;

  loadSubmissionPage.mockImplementation(async () => {
    callCount += 1;
    if (callCount === 1) return page([row("1", "Row One")], "curA");
    if (callCount === 2)
      return new Promise<SubmissionPage>((resolve) => {
        resolveRefresh = resolve;
      });
    throw new Error(`unexpected loadSubmissionPage call ${callCount}`);
  });
  const { container } = renderDashboard();

  await user.click(await screen.findByRole("link", { name: /Row One/ }));
  await screen.findByRole("heading", { name: "Curation" });

  // A curation save starts the refresh. Its fetch is now in flight.
  await user.click(screen.getByRole("button", { name: "Assign conference" }));
  await user.click(
    await screen.findByRole("button", { name: "Conference One" }),
  );
  await waitFor(() => expect(loadSubmissionPage).toHaveBeenCalledTimes(2));

  // The reader reaches the bottom while the refresh is still running. This
  // must ask for nothing: the mock throws on a third call.
  await user.click(screen.getByRole("button", { name: "Load more" }));
  expect(loadSubmissionPage).toHaveBeenCalledTimes(2);

  resolveRefresh(page([row("1", "Row One")], "curA-refreshed"));
  await waitFor(() => expect(saveTracking).toHaveBeenCalled());

  const rowIDs = [...container.querySelectorAll(".row__id")].map((node) =>
    (node.textContent ?? "").replace("ID ", ""),
  );
  expect(rowIDs).toEqual(["1"]);
});

it("clears the open submission when Escape is pressed at desktop width", async () => {
  const user = userEvent.setup();
  loadSubmissionPage.mockResolvedValue(page([row("1", "Row One")]));
  renderDashboard();

  await user.click(await screen.findByRole("link", { name: /Row One/ }));
  await screen.findByRole("heading", { name: "Curation" });

  await user.keyboard("{Escape}");

  await waitFor(() =>
    expect(screen.queryByRole("heading", { name: "Curation" })).toBeNull(),
  );
});

it("presents an open submission as a full-screen modal at mobile width, closable by the browser's back action", async () => {
  setCompact(true);
  const user = userEvent.setup();
  loadSubmissionPage.mockResolvedValue(page([row("1", "Row One")]));
  renderDashboard();

  await user.click(await screen.findByRole("link", { name: /Row One/ }));
  await screen.findByRole("heading", { name: "Curation" });
  expect(screen.getByRole("button", { name: "Close" })).toBeDefined();

  window.history.back();

  await waitFor(() =>
    expect(screen.queryByRole("heading", { name: "Curation" })).toBeNull(),
  );
});

it("prefills folder:muted from the route when /muted carries no ?q", async () => {
  window.history.replaceState(null, "", "/muted");
  loadSideScreen.mockResolvedValue(page([]));
  renderDashboard();

  expect(
    await screen.findByRole("button", { name: "Edit Muted" }),
  ).toBeTruthy();
});

it("navigates to /archived when ?q names a folder: that disagrees with /muted", async () => {
  window.history.replaceState(null, "", "/muted?q=folder%3Aarchived");
  loadSideScreen.mockResolvedValue(page([]));
  renderDashboard();

  await screen.findByRole("button", { name: "Edit Archived" });
  await waitFor(() => expect(window.location.pathname).toBe("/archived"));
});

it("enters all-folder search from a ?q with no folder: token, staying on /, badging every row", async () => {
  window.history.replaceState(null, "", "/?q=editor%3Aanderson-s");
  loadSubmissionPage.mockResolvedValue(
    page([row("1", "Row One", "up_to_date")]),
  );
  const { container } = renderDashboard();

  await screen.findByRole("link", { name: /Row One/ });
  // No folder: chip at all: only the editor chip renders, with the raw
  // (unquoted, uncapitalized) wire text as its label — `?q` only carries a
  // proper display name when the chip was quoted when written. An editor
  // chip has no edit affordance: picking another editor always adds a
  // second chip rather than replacing this one, so there is nothing to edit.
  expect(
    screen.queryByRole("button", {
      name: /^Edit (Work|Muted|Up to date|Archived)$/,
    }),
  ).toBeNull();
  expect(screen.queryByRole("button", { name: "Edit anderson-s" })).toBeNull();
  expect(screen.getByText("editor:anderson-s")).toBeTruthy();
  expect(container.querySelector(".row__folder-badge")).toBeTruthy();
  // The all-folder request carries no queue restriction, only the query text.
  expect(loadSubmissionPage).toHaveBeenCalledWith(
    expect.objectContaining({ query: 'editor:"anderson-s"' }),
  );
});
