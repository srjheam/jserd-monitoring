import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ClientProviders } from "../src/app/provider";
import {
  ArchivedScreen,
  MutedScreen,
  UpToDateScreen,
} from "../src/components/side-screens";
import { WorkList } from "../src/components/work-list";
import type {
  ActivityItem,
  EditorFrequency,
  MonitoringMeta,
  Submission,
} from "../src/lib/monitoring";
import type { Query } from "../src/lib/query";
import { reasonOf, settingAsideEntry } from "../src/lib/side-screens";

const activity = vi.hoisted(() =>
  vi.fn<(id: string, requestKey?: string) => Promise<ActivityItem[]>>(),
);
vi.mock("../src/lib/api", () => ({ loadActivity: activity }));

beforeEach(() => {
  activity.mockReset();
  activity.mockResolvedValue([]);
});

const submission = (overrides: Partial<Submission> = {}): Submission => ({
  ojsSubmissionId: "600440",
  title: "Synthetic Job Ads Article",
  status: "Queued",
  url: "",
  dateSubmitted: "2026-01-01",
  lastModified: "2026-01-01",
  editors: ["Casey Lee"],
  decisions: [],
  reviews: [],
  findings: [],
  effectiveFinalState: "",
  effectiveQueue: "muted",
  effectiveAttention: "",
  findingDigest: "digest",
  provenance: {},
  ...overrides,
});

const tracking = (overrides: Partial<Submission["tracking"]> = {}) => ({
  id: "t1",
  conference: "",
  muted: true,
  mutedDigest: "digest",
  upToDateAt: "",
  upToDateUntil: "",
  created: "",
  updated: "",
  ...overrides,
});

const meta = (
  conferences: { id: string; name: string; description: string }[] = [],
): MonitoringMeta => ({
  generatedAt: "",
  availability: "available",
  conferences,
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

const mutedFolderQuery: Query = {
  chips: [{ tag: "folder", value: "muted", label: "Muted" }],
  text: "",
};
const workFolderQuery: Query = {
  chips: [{ tag: "folder", value: "work", label: "Work" }],
  text: "",
};

const conferences = [
  { id: "sbes", name: "SBES 2025", description: "" },
  { id: "sast", name: "SAST 2025", description: "" },
  { id: "sbqs2", name: "SBQS 2", description: "" },
  { id: "sbqs10", name: "SBQS 10", description: "" },
];

function renderMuted(
  items: Submission[],
  options: {
    editorFrequencies?: EditorFrequency[];
    query?: Query;
    onQuery?: (query: Query) => void;
    onUnmute?: (item: Submission) => Promise<void>;
    selectedId?: string;
  } = {},
) {
  return render(
    <ClientProviders>
      <MutedScreen
        items={items}
        loaded
        meta={meta(conferences)}
        query={options.query ?? mutedFolderQuery}
        editorFrequencies={options.editorFrequencies ?? []}
        route="muted"
        selectedId={options.selectedId}
        onQuery={options.onQuery ?? (() => {})}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={() => {}}
        onUnmute={options.onUnmute ?? (async () => {})}
      />
    </ClientProviders>,
  );
}

function renderUpToDate(
  items: Submission[],
  options: {
    editorFrequencies?: EditorFrequency[];
    query?: Query;
    onQuery?: (query: Query) => void;
    selectedId?: string;
    onSendBackToWork?: (submission: Submission) => Promise<void>;
  } = {},
) {
  return render(
    <ClientProviders>
      <UpToDateScreen
        items={items}
        loaded
        meta={meta(conferences)}
        query={
          options.query ?? {
            chips: [
              { tag: "folder", value: "up_to_date", label: "Up to date" },
            ],
            text: "",
          }
        }
        editorFrequencies={options.editorFrequencies ?? []}
        route="up_to_date"
        selectedId={options.selectedId}
        onQuery={options.onQuery ?? (() => {})}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={() => {}}
        onSendBackToWork={options.onSendBackToWork ?? (async () => {})}
      />
    </ClientProviders>,
  );
}

const clusters = (container: HTMLElement) => [
  ...container.querySelectorAll<HTMLElement>(".cluster"),
];

const clusterNames = (container: HTMLElement) =>
  clusters(container).map(
    (node) => within(node).getByRole("heading").textContent,
  );

const rowIDs = (container: HTMLElement) =>
  [...container.querySelectorAll(".row__id")].map((node) =>
    (node.textContent ?? "").replace("ID ", ""),
  );

it("shows final outcome badges on the Archived screen", () => {
  render(
    <ClientProviders>
      <ArchivedScreen
        meta={meta()}
        items={[
          submission({
            effectiveQueue: "archived",
            effectiveFinalState: "accepted",
          }),
          submission({
            ojsSubmissionId: "600387",
            effectiveQueue: "archived",
            effectiveFinalState: "rejected",
          }),
          submission({
            ojsSubmissionId: "5488",
            effectiveQueue: "archived",
            effectiveFinalState: "withdrawn",
          }),
        ]}
        loaded
        query={{ chips: [], text: "" }}
        editorFrequencies={[]}
        route="archived"
        onQuery={() => {}}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={() => {}}
      />
    </ClientProviders>,
  );
  expect(screen.getByText("Accepted")).toBeDefined();
  expect(screen.getByText("Rejected")).toBeDefined();
  expect(screen.getByText("Withdrawn")).toBeDefined();
});

// Clusters sort alphanumerically by conference name, so SBQS 2 comes before
// SBQS 10 and a plain string sort would get that pair wrong.
it("sorts clusters alphanumerically by conference name", () => {
  const { container } = renderMuted([
    submission({
      ojsSubmissionId: "1",
      tracking: tracking({ conference: "sbes" }),
    }),
    submission({
      ojsSubmissionId: "2",
      tracking: tracking({ conference: "sbqs10" }),
    }),
    submission({
      ojsSubmissionId: "3",
      tracking: tracking({ conference: "sast" }),
    }),
    submission({
      ojsSubmissionId: "4",
      tracking: tracking({ conference: "sbqs2" }),
    }),
  ]);
  expect(clusterNames(container)).toEqual([
    "⟨SAST 2025⟩",
    "⟨SBES 2025⟩",
    "⟨SBQS 2⟩",
    "⟨SBQS 10⟩",
  ]);
});

// Inside a cluster the oldest activity rises, which is the opposite of the
// work list: nobody is watching these, so the one waiting longest is first.
it("orders rows inside a cluster by oldest activity first", () => {
  const { container } = renderMuted([
    submission({
      ojsSubmissionId: "middle",
      lastModified: "2026-02-01",
      tracking: tracking({ conference: "sast" }),
    }),
    submission({
      ojsSubmissionId: "newest",
      lastModified: "2026-03-01",
      tracking: tracking({ conference: "sast" }),
    }),
    submission({
      ojsSubmissionId: "oldest",
      lastModified: "2026-01-01",
      tracking: tracking({ conference: "sast" }),
    }),
  ]);
  expect(rowIDs(container)).toEqual(["oldest", "middle", "newest"]);
});

// The whole point of the state: a muted submission stays current, and a
// finding that arrives after the mute marks the row where it stands. Moving
// the row would undo the delegation the chief editor decided on.
it("marks a changed finding as new since muted without moving the row", () => {
  const before = [
    submission({
      ojsSubmissionId: "first",
      lastModified: "2026-01-01",
      tracking: tracking({ conference: "sast" }),
    }),
    submission({
      ojsSubmissionId: "second",
      lastModified: "2026-02-01",
      findingDigest: "digest",
      tracking: tracking({ conference: "sast" }),
    }),
    submission({
      ojsSubmissionId: "third",
      lastModified: "2026-03-01",
      tracking: tracking({ conference: "sast" }),
    }),
  ];
  const { container, rerender } = renderMuted(before);
  expect(rowIDs(container)).toEqual(["first", "second", "third"]);
  // Asserted on the marker's own class, not on its wording: the copy is a
  // translation and must stay free to change without breaking this test.
  expect(container.querySelectorAll(".row__since-muted")).toHaveLength(0);

  // The next collection finds a reviewer missing on the middle row. Its
  // digest moves away from the one stamped at mute time; nothing else does.
  const after = before.map((item) =>
    item.ojsSubmissionId === "second"
      ? {
          ...item,
          findingDigest: "changed",
          findings: [
            { kind: "missing_reviewer", responsible: "editor", deadline: "" },
          ],
        }
      : item,
  );
  rerender(
    <ClientProviders>
      <MutedScreen
        items={after}
        loaded
        meta={meta(conferences)}
        query={mutedFolderQuery}
        editorFrequencies={[]}
        route="muted"
        onQuery={() => {}}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={() => {}}
        onUnmute={async () => {}}
      />
    </ClientProviders>,
  );
  expect(rowIDs(container)).toEqual(["first", "second", "third"]);
  expect(container.querySelectorAll(".row__since-muted")).toHaveLength(1);
  const marked = container.querySelectorAll(".row")[1];
  expect(marked.querySelector(".row__since-muted")).toBeTruthy();
});

// Conference associations are managed in JSERD Monitoring separately from OJS.
it("carries the JSERD marker on every cluster header", () => {
  renderMuted([
    submission({
      ojsSubmissionId: "1",
      tracking: tracking({ conference: "sast" }),
    }),
    submission({
      ojsSubmissionId: "2",
      tracking: tracking({ conference: "sbes" }),
    }),
  ]);
  const markers = screen.getAllByRole("button", { name: "JSERD" });
  expect(markers).toHaveLength(2);
  for (const marker of markers)
    expect(marker.querySelector(".lucide-info")).toBeDefined();
});

// "No submission is muted" is a claim about the work, and the first batch has
// not answered yet when the screen first renders.
it("does not claim the screen is empty before the first batch answers", () => {
  const { rerender } = render(
    <ClientProviders>
      <MutedScreen
        items={[]}
        loaded={false}
        meta={meta(conferences)}
        query={mutedFolderQuery}
        editorFrequencies={[]}
        route="muted"
        onQuery={() => {}}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={() => {}}
        onUnmute={async () => {}}
      />
    </ClientProviders>,
  );
  expect(screen.queryByText("No submission is muted")).toBeNull();
  rerender(
    <ClientProviders>
      <MutedScreen
        items={[]}
        loaded
        meta={meta(conferences)}
        query={mutedFolderQuery}
        editorFrequencies={[]}
        route="muted"
        onQuery={() => {}}
        onNavigate={() => {}}
        onOpen={() => {}}
        onScreen={() => {}}
        onUnmute={async () => {}}
      />
    </ClientProviders>,
  );
  expect(screen.getByText("No submission is muted")).toBeTruthy();
  expect(
    screen
      .getByText("No submission is muted")
      .closest(".empty-state")
      ?.querySelector(".lucide-inbox"),
  ).toBeDefined();
});

it("unmutes a submission from its row", async () => {
  const onUnmute = vi.fn(async () => {});
  const item = submission({
    ojsSubmissionId: "600400",
    tracking: tracking({ conference: "sast" }),
  });
  renderMuted([item], { onUnmute });
  await userEvent.click(screen.getByRole("button", { name: "Unmute" }));
  expect(onUnmute).toHaveBeenCalledWith(item);
});

// The row used to have no `active`/selected concept at all: the open
// submission never got a background here, even though the same row on the
// work list has always highlighted it.
it("highlights the open row on the Muted screen, the same as the work list does", () => {
  renderMuted(
    [
      submission({
        ojsSubmissionId: "600400",
        tracking: tracking({ conference: "sast" }),
      }),
      submission({
        ojsSubmissionId: "600387",
        tracking: tracking({ conference: "sast" }),
      }),
    ],
    { selectedId: "600400" },
  );
  const rows = screen.getAllByRole("article");
  const openRow = rows.find((row) => row.textContent?.includes("600400"));
  const otherRow = rows.find((row) => row.textContent?.includes("600387"));
  expect(openRow?.className).toContain("row--active");
  expect(otherRow?.className).not.toContain("row--active");
});

describe("the Up to date screen", () => {
  it("is a flat list and says why each row is there", async () => {
    activity.mockResolvedValue([
      {
        kind: "journal",
        occurredAt: "2026-08-27T10:00:00Z",
        authorLabel: "morgan@example.test",
        body: "Set aside until 2026-09-17. Author on vacation for three weeks.",
      },
    ]);
    const { container } = renderUpToDate([
      submission({
        ojsSubmissionId: "no-findings",
        effectiveQueue: "up_to_date",
        findings: [],
        // It carries an older marking, but no finding is why it is here
        // now, so the row has no journal entry to go looking for.
        tracking: tracking({
          muted: false,
          mutedDigest: "",
          upToDateAt: "2026-08-27 00:00:00.000Z",
        }),
      }),
      submission({
        ojsSubmissionId: "set-aside",
        effectiveQueue: "up_to_date",
        findings: [
          { kind: "missing_reviewer", responsible: "editor", deadline: "" },
        ],
        tracking: tracking({
          muted: false,
          mutedDigest: "",
          upToDateAt: "2026-08-27 00:00:00.000Z",
          upToDateUntil: "2026-09-17 00:00:00.000Z",
        }),
      }),
    ]);
    // A flat list: no cluster, and so no conference header.
    expect(clusters(container)).toHaveLength(0);
    // A submission with no findings shows no reason label at all.
    expect(screen.queryByText("No findings")).toBeNull();
    // The whole line, not a substring of it: the row already carries the day
    // and the return date, so the journal sentence that repeats them both is
    // dropped and only the words the chief editor added survive.
    expect(
      (await screen.findByText(/Set aside on 2026-08-27 by/)).textContent,
    ).toBe(
      "Set aside on 2026-08-27 by morgan@example.test · Returns on 2026-09-17 · Author on vacation for three weeks.",
    );
    // Only the set-aside row asks. A row that is here because it has no
    // finding has no marking to explain.
    expect(activity).toHaveBeenCalledTimes(1);
    expect(activity.mock.calls[0][0]).toBe("set-aside");
  });

  // The journal is free text and the timeline arrives newest first, so a note
  // that merely opens with the same words must not pass itself off as the
  // marking and lend it an author.
  it("ignores a later note that only looks like the marking", () => {
    const marking = {
      kind: "journal" as const,
      occurredAt: "2026-08-27T10:00:00Z",
      authorLabel: "morgan@example.test",
      body: "Set aside until 2026-09-17. Author on vacation.",
    };
    expect(
      settingAsideEntry([
        {
          kind: "journal",
          occurredAt: "2026-08-28T10:00:00Z",
          authorLabel: "someone@example.test",
          body: "Set aside for now, I will chase the reviewer on Monday.",
        },
        marking,
      ]),
    ).toBe(marking);
    expect(reasonOf(marking.body)).toBe("Author on vacation.");
    expect(reasonOf("Set aside.")).toBe("");
  });

  it("says why a row is there without a journal entry to read", async () => {
    renderUpToDate([
      submission({
        ojsSubmissionId: "set-aside",
        effectiveQueue: "up_to_date",
        findings: [
          { kind: "missing_reviewer", responsible: "editor", deadline: "" },
        ],
        tracking: tracking({
          muted: false,
          mutedDigest: "",
          upToDateAt: "2026-08-27 00:00:00.000Z",
        }),
      }),
    ]);
    await waitFor(() => expect(activity).toHaveBeenCalled());
    expect(screen.getByText(/Set aside on 2026-08-27/)).toBeTruthy();
  });

  it("highlights the open row, the same as the work list does", () => {
    renderUpToDate(
      [
        submission({ ojsSubmissionId: "600400", effectiveQueue: "up_to_date" }),
        submission({ ojsSubmissionId: "600387", effectiveQueue: "up_to_date" }),
      ],
      { selectedId: "600400" },
    );
    const rows = screen.getAllByRole("article");
    const openRow = rows.find((row) => row.textContent?.includes("600400"));
    const otherRow = rows.find((row) => row.textContent?.includes("600387"));
    expect(openRow?.className).toContain("row--active");
    expect(otherRow?.className).not.toContain("row--active");
  });

  it("sends a submission back to work from its row", async () => {
    const onSendBackToWork = vi.fn(async () => {});
    const item = submission({
      ojsSubmissionId: "600400",
      effectiveQueue: "up_to_date",
    });
    renderUpToDate([item], { onSendBackToWork });
    await userEvent.click(
      screen.getByRole("button", { name: "Send back to work" }),
    );
    expect(onSendBackToWork).toHaveBeenCalledWith(item);
  });
});

// The same combobox, with the same chip behaviour, on all three screens. A
// second search would be a second set of rules to learn.
describe("the search chips behave identically on all three screens", () => {
  const item = submission({
    ojsSubmissionId: "600400",
    tracking: tracking({ conference: "sast" }),
  });
  const editorFrequencies: EditorFrequency[] = [
    { slug: "casey-lee", label: "Casey Lee", count: 3 },
  ];
  const screens: [
    string,
    (onQuery: (query: Query) => void, query: Query) => ReactElement,
    Query,
  ][] = [
    [
      "work list",
      (onQuery: (query: Query) => void, query: Query) => (
        <WorkList
          items={[item]}
          meta={meta(conferences)}
          query={query}
          editorFrequencies={editorFrequencies}
          route={undefined}
          total={1}
          hasMore={false}
          loadingMore={false}
          onQuery={onQuery}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onLoadMore={() => {}}
          onMarkUpToDate={async () => {}}
        />
      ),
      workFolderQuery,
    ],
    [
      "Muted",
      (onQuery: (query: Query) => void, query: Query) => (
        <MutedScreen
          items={[item]}
          loaded
          meta={meta(conferences)}
          query={query}
          editorFrequencies={editorFrequencies}
          route="muted"
          onQuery={onQuery}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onUnmute={async () => {}}
        />
      ),
      mutedFolderQuery,
    ],
    [
      "Up to date",
      (onQuery: (query: Query) => void, query: Query) => (
        <UpToDateScreen
          items={[item]}
          loaded
          meta={meta(conferences)}
          query={query}
          editorFrequencies={editorFrequencies}
          route="up_to_date"
          onQuery={onQuery}
          onNavigate={() => {}}
          onOpen={() => {}}
          onScreen={() => {}}
          onSendBackToWork={async () => {}}
        />
      ),
      {
        chips: [{ tag: "folder", value: "up_to_date", label: "Up to date" }],
        text: "",
      },
    ],
  ];

  for (const [name, view, folderOnlyQuery] of screens) {
    it(`accepts a suggestion as a chip on the ${name}`, async () => {
      const onQuery = vi.fn();
      const { rerender } = render(
        <ClientProviders>{view(onQuery, folderOnlyQuery)}</ClientProviders>,
      );
      await userEvent.click(screen.getByRole("combobox"));
      expect(screen.getByRole("option", { name: "Casey Lee" })).toBeTruthy();
      await userEvent.click(screen.getByRole("option", { name: "Casey Lee" }));
      expect(onQuery).toHaveBeenCalledWith({
        chips: [
          ...folderOnlyQuery.chips,
          {
            tag: "editor",
            value: "casey-lee",
            label: "Casey Lee",
          },
        ],
        text: "",
      });

      // The accepted chip renders with its own remove control, and removing
      // it hands back the folder-only query.
      onQuery.mockClear();
      const withChip: Query = {
        chips: [
          ...folderOnlyQuery.chips,
          {
            tag: "editor",
            value: "casey-lee",
            label: "Casey Lee",
          },
        ],
        text: "",
      };
      rerender(<ClientProviders>{view(onQuery, withChip)}</ClientProviders>);
      await userEvent.click(
        screen.getByRole("button", { name: "Remove Casey Lee" }),
      );
      expect(onQuery).toHaveBeenCalledWith(folderOnlyQuery);

      // Clear drops the chip and any free text, but keeps the folder chip.
      onQuery.mockClear();
      await userEvent.click(screen.getByRole("button", { name: "Clear" }));
      expect(onQuery).toHaveBeenCalledWith(folderOnlyQuery);
    });
  }
});

// The counts ship as links now: ticket 08 left them as text because the
// screens they name did not exist yet.
it("links the work list footer counts to the two side screens", async () => {
  const onScreen = vi.fn();
  render(
    <ClientProviders>
      <WorkList
        items={[]}
        meta={meta(conferences)}
        query={workFolderQuery}
        editorFrequencies={[]}
        route={undefined}
        total={0}
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
  const muted = screen.getByRole("link", { name: "Muted 14" });
  const upToDate = screen.getByRole("link", { name: "Up to date 8" });
  expect(muted.getAttribute("href")).toBe("/muted");
  expect(upToDate.getAttribute("href")).toBe("/up-to-date");

  // A plain click stays in the page, so `limit` and the open submission
  // survive the trip.
  await userEvent.click(muted);
  expect(onScreen).toHaveBeenCalledWith("muted");
});
