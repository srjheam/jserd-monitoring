import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ClientProviders } from "../src/app/provider";
import { WorkList } from "../src/components/work-list";
import type {
  EditorFrequency,
  MonitoringMeta,
  SideScreen,
  Submission,
} from "../src/lib/monitoring";
import type { Query } from "../src/lib/query";
import { addMonth, elapsed, isoDate } from "../src/lib/work-list";

const submission = (overrides: Partial<Submission> = {}): Submission => ({
  ojsSubmissionId: "600440",
  title: "Synthetic Job Ads Article",
  status: "Queued",
  url: "",
  dateSubmitted: "2026-01-01",
  lastModified: "",
  editors: ["Casey Lee"],
  decisions: [],
  reviews: [],
  findings: [],
  effectiveFinalState: "",
  effectiveQueue: "daily",
  effectiveAttention: "missing_editor",
  findingDigest: "digest",
  provenance: {},
  ...overrides,
});

const meta = (): MonitoringMeta => ({
  generatedAt: "",
  availability: "available",
  conferences: [],
  aggregates: {
    metrics: {},
    triage: {
      daily: 0,
      archived: 0,
      muted: 14,
      up_to_date: 8,
    },
    dailyRequiringAttention: 0,
    statuses: [],
    editors: [],
    total: 0,
  },
});

const workFolderQuery: Query = {
  chips: [{ tag: "folder", value: "work", label: "Work" }],
  text: "",
};

function renderList(
  items: Submission[],
  options: {
    editorFrequencies?: EditorFrequency[];
    onQuery?: (query: Query) => void;
    query?: Query;
    route?: SideScreen;
    selectedId?: string;
    onNavigate?: (screen: SideScreen | undefined) => void;
    onOpen?: (id: string, note: boolean) => void;
    onMarkUpToDate?: (
      item: Submission,
      until: string,
      reason: string,
    ) => Promise<void>;
  } = {},
) {
  return render(
    <ClientProviders>
      <WorkList
        items={items}
        meta={meta()}
        query={options.query ?? workFolderQuery}
        editorFrequencies={options.editorFrequencies ?? []}
        route={options.route}
        selectedId={options.selectedId}
        total={items.length}
        hasMore={false}
        loadingMore={false}
        onQuery={options.onQuery ?? (() => {})}
        onNavigate={options.onNavigate ?? (() => {})}
        onOpen={options.onOpen ?? (() => {})}
        onScreen={() => {}}
        onLoadMore={() => {}}
        onMarkUpToDate={options.onMarkUpToDate ?? (async () => {})}
      />
    </ClientProviders>,
  );
}

// The duplication this ticket removes: with tabs, 36 submissions produced 47
// rows and 11 titles appeared twice. With one list, one submission is one row
// and every finding it carries is a badge on that row.
it("renders a submission with two findings as one row with two badges", () => {
  renderList([
    submission({
      findings: [
        { kind: "missing_editor", responsible: "editor", deadline: "" },
        { kind: "missing_reviewer", responsible: "editor", deadline: "" },
      ],
    }),
  ]);
  const rows = screen.getAllByRole("article");
  expect(rows).toHaveLength(1);
  const row = within(rows[0]);
  expect(row.getByText("No editor")).toBeDefined();
  expect(row.getByText("No reviewer")).toBeDefined();
  // One anchor carries the whole accessible name; the actions sit outside it.
  const link = row.getByRole("link", { name: /Synthetic Job Ads Article/ });
  expect(link.getAttribute("href")).toBe("?article=600440");
  expect(link.textContent).toContain("600440");
  expect(link.textContent).toContain("Casey Lee");
});

it("gives the note control its own link to the focused note box", () => {
  renderList([submission()]);
  const note = screen.getByRole("link", { name: "Add a note" });
  expect(note.getAttribute("href")).toBe("?article=600440&note=1");
});

describe("the row open in the right panel", () => {
  it("highlights that row and hides its quick actions, leaving every other row unchanged", () => {
    renderList([submission(), submission({ ojsSubmissionId: "600387" })], {
      selectedId: "600440",
    });
    const rows = screen.getAllByRole("article");
    const openRow = rows.find((row) => row.textContent?.includes("600440"));
    const otherRow = rows.find((row) => row.textContent?.includes("600387"));
    expect(openRow?.className).toContain("row--active");
    expect(otherRow?.className).not.toContain("row--active");
    expect(
      within(openRow as HTMLElement).queryByRole("link", {
        name: "Add a note",
      }),
    ).toBeNull();
    expect(
      within(openRow as HTMLElement).queryByRole("button", {
        name: "Mark up to date",
      }),
    ).toBeNull();
    expect(
      within(otherRow as HTMLElement).getByRole("link", {
        name: "Add a note",
      }),
    ).toBeDefined();
    expect(
      within(otherRow as HTMLElement).getByRole("button", {
        name: "Mark up to date",
      }),
    ).toBeDefined();
  });
});

describe("the queue badge's search-mode colour", () => {
  it("gains the blue modifier only while a search or filter is active", () => {
    const { container, rerender } = render(
      <ClientProviders>
        <WorkList
          items={[submission()]}
          meta={meta()}
          query={{ chips: [], text: "" }}
          editorFrequencies={[]}
          route={undefined}
          total={1}
          hasMore={false}
          loadingMore={false}
          onQuery={() => {}}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onLoadMore={() => {}}
          onMarkUpToDate={async () => {}}
        />
      </ClientProviders>,
    );
    expect(
      container.querySelector(".row__folder-badge")?.className,
    ).not.toContain("badge--search");
    rerender(
      <ClientProviders>
        <WorkList
          items={[submission()]}
          meta={meta()}
          query={{ chips: [], text: "job ads" }}
          editorFrequencies={[]}
          route={undefined}
          total={1}
          hasMore={false}
          loadingMore={false}
          onQuery={() => {}}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onLoadMore={() => {}}
          onMarkUpToDate={async () => {}}
        />
      </ClientProviders>,
    );
    expect(container.querySelector(".row__folder-badge")?.className).toContain(
      "badge--search",
    );
  });
});

it("opens the submission in place on a plain click and defers to the browser on a modified one", async () => {
  const user = userEvent.setup();
  const onOpen = vi.fn();
  renderList([submission()], { onOpen });
  await user.click(
    screen.getByRole("link", { name: /Synthetic Job Ads Article/ }),
  );
  expect(onOpen).toHaveBeenCalledWith("600440", false);
  await user.keyboard("{Control>}");
  await user.click(
    screen.getByRole("link", { name: /Synthetic Job Ads Article/ }),
  );
  await user.keyboard("{/Control}");
  expect(onOpen).toHaveBeenCalledTimes(1);
});

it("puts the count at the bottom beside all three side screen links", () => {
  const { container } = renderList([
    submission(),
    submission({ ojsSubmissionId: "600387" }),
  ]);
  const footer = container.querySelector("footer") as HTMLElement;
  expect(footer.textContent).toContain("2 submissions need you");
  // Each side screen is a direct link; only two screens show counts.
  expect(footer.textContent).toContain("Muted 14");
  expect(footer.textContent).toContain("Up to date 8");
  expect(within(footer).getAllByRole("link")).toHaveLength(3);
  // The summary belongs after the thing it summarizes.
  expect(
    footer.compareDocumentPosition(screen.getAllByRole("article")[0]) &
      Node.DOCUMENT_POSITION_PRECEDING,
  ).toBeTruthy();
});

it("navigates directly to Archived on a plain click and preserves modified clicks", async () => {
  const user = userEvent.setup();
  const onScreen = vi.fn();
  render(
    <ClientProviders>
      <WorkList
        items={[submission()]}
        meta={meta()}
        query={workFolderQuery}
        editorFrequencies={[]}
        route={undefined}
        total={1}
        hasMore={false}
        loadingMore={false}
        onQuery={() => {}}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={onScreen}
        onLoadMore={() => {}}
        onMarkUpToDate={async () => {}}
      />
    </ClientProviders>,
  );
  const archived = screen.getByRole("link", { name: "Archived" });
  expect(archived.getAttribute("href")).toBe("/archived");
  await user.click(archived);
  expect(onScreen).toHaveBeenCalledWith("archived");
  await user.keyboard("{Control>}");
  await user.click(archived);
  await user.keyboard("{/Control}");
  expect(onScreen).toHaveBeenCalledTimes(1);
});

it("shows copyediting and actual final outcome badges in all-folder search", () => {
  renderList(
    [
      submission({
        ojsSubmissionId: "accepted",
        effectiveQueue: "archived",
        effectiveFinalState: "accepted",
      }),
      submission({
        ojsSubmissionId: "rejected",
        effectiveQueue: "archived",
        effectiveFinalState: "rejected",
      }),
      submission({
        ojsSubmissionId: "active",
        status: "Copyediting",
        effectiveQueue: "daily",
        effectiveFinalState: "accepted",
        decisions: [{ decision: "Accepted", date: "2026-01-01" }],
        findings: [
          { kind: "missing_copyeditor", responsible: "editor", deadline: "" },
        ],
      }),
      submission({
        ojsSubmissionId: "published",
        status: "Published",
        effectiveQueue: "daily",
        effectiveFinalState: "accepted",
      }),
      submission({
        ojsSubmissionId: "archived-copyediting",
        status: "Copyediting",
        effectiveQueue: "archived",
        effectiveFinalState: "accepted",
      }),
      submission({
        ojsSubmissionId: "withdrawn",
        effectiveQueue: "archived",
        effectiveFinalState: "withdrawn",
      }),
    ],
    { query: { chips: [], text: "" } },
  );
  const rows = screen.getAllByRole("article");
  expect(within(rows[0]).getByText("Accepted")).toBeDefined();
  expect(within(rows[1]).getByText("Rejected")).toBeDefined();
  expect(within(rows[2]).getByText("Copyediting")).toBeDefined();
  expect(within(rows[2]).getByText("No copyeditor")).toBeDefined();
  expect(within(rows[2]).queryByText("Accepted")).toBeNull();
  expect(within(rows[3]).getByText("Accepted")).toBeDefined();
  expect(within(rows[4]).getByText("Copyediting")).toBeDefined();
  expect(within(rows[4]).queryByText("Accepted")).toBeNull();
  expect(within(rows[5]).getByText("Withdrawn")).toBeDefined();
});

describe("the search combobox", () => {
  it("turns a clicked editor suggestion into a chip, committing at once", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderList([submission()], {
      editorFrequencies: [
        { slug: "casey-lee", label: "Casey Lee", count: 2 },
        { slug: "jordan-reed", label: "Jordan Reed", count: 1 },
      ],
      onQuery,
    });
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "Casey Lee" }));
    expect(onQuery).toHaveBeenCalledWith({
      chips: [
        { tag: "folder", value: "work", label: "Work" },
        {
          tag: "editor",
          value: "casey-lee",
          label: "Casey Lee",
        },
      ],
      text: "",
    });
  });

  it("removes a chip from its own button", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderList([submission()], {
      query: {
        chips: [
          { tag: "folder", value: "work", label: "Work" },
          {
            tag: "editor",
            value: "casey-lee",
            label: "Casey Lee",
          },
        ],
        text: "",
      },
      onQuery,
    });
    await user.click(screen.getByRole("button", { name: "Remove Casey Lee" }));
    expect(onQuery).toHaveBeenCalledWith({
      chips: [{ tag: "folder", value: "work", label: "Work" }],
      text: "",
    });
  });

  it("commits free text on Enter, not on every keystroke", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderList([submission()], { onQuery });
    await user.type(screen.getByRole("combobox"), "job ads");
    expect(onQuery).not.toHaveBeenCalled();
    await user.keyboard("{Enter}");
    expect(onQuery).toHaveBeenCalledWith({
      chips: [{ tag: "folder", value: "work", label: "Work" }],
      text: "job ads",
    });
  });

  it("reaches the folder field on ArrowUp and reverts on Escape", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderList([submission()], { onQuery });
    const input = screen.getByRole("combobox") as HTMLInputElement;
    await user.click(input);
    // Up from the free-text field wraps straight to the folder field.
    await user.keyboard("{ArrowUp}");
    expect(screen.getByRole("option", { name: "Work" })).toBeDefined();
    await user.keyboard("{Escape}");
    expect(onQuery).not.toHaveBeenCalled();
    expect(input.value).toBe("");
  });

  it("deletes the trailing chip with Backspace on an empty input", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    renderList([submission()], {
      query: {
        chips: [
          { tag: "folder", value: "work", label: "Work" },
          { tag: "editor", value: "jordan-reed", label: "Jordan Reed" },
        ],
        text: "",
      },
      onQuery,
    });
    await user.click(screen.getByRole("combobox"));
    await user.keyboard("{Backspace}");
    expect(onQuery).toHaveBeenCalledWith({
      chips: [{ tag: "folder", value: "work", label: "Work" }],
      text: "",
    });
  });

  it("replaces the folder chip and navigates when a different folder suggestion is clicked", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    const onNavigate = vi.fn();
    renderList([submission()], { onQuery, onNavigate });
    await user.click(screen.getByRole("combobox"));
    await user.click(screen.getByRole("option", { name: "Muted" }));
    expect(onQuery).toHaveBeenCalledWith({
      chips: [{ tag: "folder", value: "muted", label: "Muted" }],
      text: "",
    });
    expect(onNavigate).toHaveBeenCalledWith("muted");
  });

  it("leaves the already-committed folder out of its own suggestions", async () => {
    const user = userEvent.setup();
    renderList([submission()]);
    await user.click(screen.getByRole("combobox"));
    expect(screen.queryByRole("option", { name: "Work" })).toBeNull();
    expect(screen.getByRole("option", { name: "Muted" })).toBeDefined();
  });

  it("navigates to the work route when the folder chip is deleted from a side screen", async () => {
    const user = userEvent.setup();
    const onQuery = vi.fn();
    const onNavigate = vi.fn();
    renderList([submission()], {
      route: "muted",
      query: {
        chips: [{ tag: "folder", value: "muted", label: "Muted" }],
        text: "",
      },
      onQuery,
      onNavigate,
    });
    await user.click(screen.getByRole("button", { name: "Remove Muted" }));
    expect(onQuery).toHaveBeenCalledWith({ chips: [], text: "" });
    expect(onNavigate).toHaveBeenCalledWith(undefined);
  });
});

describe("mark up to date", () => {
  it("marks at once, and offers a reason and the three return presets", async () => {
    const user = userEvent.setup();
    const onMarkUpToDate = vi.fn<
      (item: Submission, until: string, reason: string) => Promise<void>
    >(async () => {});
    renderList([submission()], { onMarkUpToDate });

    await user.click(screen.getByRole("button", { name: "Mark up to date" }));
    expect(onMarkUpToDate).toHaveBeenCalledWith(
      expect.objectContaining({ ojsSubmissionId: "600440" }),
      "",
      "",
    );

    await user.click(
      screen.getByRole("button", { name: "More up to date options" }),
    );
    for (const label of [
      "Add a reason",
      "Return in one week",
      "Return in two weeks",
      "Return in one month",
    ])
      expect(
        await screen.findByRole("menuitem", { name: label }),
      ).toBeDefined();

    await user.click(
      await screen.findByRole("menuitem", { name: "Return in one week" }),
    );
    const [, until] = onMarkUpToDate.mock.calls[1];
    expect(until).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});

describe("the all-folder badge", () => {
  it("shows a per-row folder badge only when no folder: chip is present", () => {
    const { container, rerender } = render(
      <ClientProviders>
        <WorkList
          items={[submission()]}
          meta={meta()}
          query={{ chips: [], text: "" }}
          editorFrequencies={[]}
          route={undefined}
          total={1}
          hasMore={false}
          loadingMore={false}
          onQuery={() => {}}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onLoadMore={() => {}}
          onMarkUpToDate={async () => {}}
        />
      </ClientProviders>,
    );
    expect(container.querySelector(".row__folder-badge")).toBeTruthy();
    rerender(
      <ClientProviders>
        <WorkList
          items={[submission()]}
          meta={meta()}
          query={workFolderQuery}
          editorFrequencies={[]}
          route={undefined}
          total={1}
          hasMore={false}
          loadingMore={false}
          onQuery={() => {}}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onLoadMore={() => {}}
          onMarkUpToDate={async () => {}}
        />
      </ClientProviders>,
    );
    expect(container.querySelector(".row__folder-badge")).toBeNull();
  });
});

describe("the return date", () => {
  it("writes the local calendar date, not the UTC one", () => {
    // 1 March 2026 at 23:00 in a zone west of UTC is already 2 March in UTC.
    const late = new Date(2026, 2, 1, 23, 0, 0);
    expect(isoDate(late)).toBe("2026-03-01");
  });
  it("clamps a month that is shorter than the one it starts in", () => {
    expect(isoDate(addMonth(new Date(2026, 0, 31)))).toBe("2026-02-28");
    expect(isoDate(addMonth(new Date(2026, 4, 31)))).toBe("2026-06-30");
    expect(isoDate(addMonth(new Date(2026, 0, 15)))).toBe("2026-02-15");
  });
});

describe("icons", () => {
  it("renders lucide icons in place of the note, checkmark and caret glyphs", () => {
    renderList([submission()]);
    expect(
      screen
        .getByRole("link", { name: "Add a note" })
        .querySelector(".lucide-pencil"),
    ).toBeDefined();
    expect(
      screen
        .getByRole("button", { name: "Mark up to date" })
        .querySelector(".lucide-check"),
    ).toBeDefined();
    expect(
      screen
        .getByRole("button", { name: "More up to date options" })
        .querySelector(".lucide-chevron-down"),
    ).toBeDefined();
    expect(screen.getByRole("link", { name: "Archived" })).toBeDefined();
  });

  it("renders the lucide warning icon in place of the finding badge's glyph", () => {
    renderList([
      submission({
        findings: [
          { kind: "missing_editor", responsible: "editor", deadline: "" },
        ],
      }),
    ]);
    expect(
      screen
        .getByText("No editor")
        .closest(".badge")
        ?.querySelector(".lucide-triangle-alert"),
    ).toBeDefined();
  });
});

describe("the elapsed line", () => {
  const now = Date.parse("2026-03-01T00:00:00Z");
  // The server ships findings worst first, so the fixture lists them that way:
  // overdue_review outranks missing_reviewer, and owns the deadline shown.
  it("headlines the overdue days of the worst finding", () => {
    expect(
      elapsed(
        submission({
          findings: [
            {
              kind: "overdue_review",
              responsible: "r",
              deadline: "2026-02-17",
            },
            { kind: "missing_reviewer", responsible: "e", deadline: "" },
          ],
        }),
        now,
      ),
    ).toEqual({ kind: "overdue", days: 12 });
  });
  it("falls back to the age of the submission when no deadline is broken", () => {
    expect(elapsed(submission({ dateSubmitted: "2026-02-23" }), now)).toEqual({
      kind: "waiting",
      days: 6,
    });
  });
  it("prefers the latest known activity over a stale dateSubmitted", () => {
    expect(
      elapsed(
        submission({
          dateSubmitted: "2025-09-01",
          lastModified: "2026-01-15",
          decisions: [{ decision: "Send to Review", date: "2026-02-20" }],
        }),
        now,
      ),
    ).toEqual({ kind: "waiting", days: 9 });
  });
  // A reviewer completing a review moves dateLastActivity alone, while
  // lastModified and decision dates can remain older.
  it("prefers dateLastActivity over lastModified and the decision dates", () => {
    expect(
      elapsed(
        submission({
          dateSubmitted: "2025-09-01",
          lastModified: "2026-01-15",
          dateLastActivity: "2026-02-25",
          decisions: [{ decision: "Send to Review", date: "2026-02-20" }],
        }),
        now,
      ),
    ).toEqual({ kind: "waiting", days: 4 });
  });
  // The ranking itself moved to the server catalog
  // (src/server/internal/editorial/catalog.go). Its order is asserted there, by
  // TestCatalogOrder and TestEvaluateReturnsFindingsWorstFirst.
});
