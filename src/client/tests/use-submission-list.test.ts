import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  MonitoringMeta,
  Submission,
  SubmissionPage,
} from "../src/lib/monitoring";
import { useSubmissionList } from "../src/lib/use-submission-list";

const row = (id: string): Submission => ({
  ojsSubmissionId: id,
  title: `Submission ${id}`,
  status: "Queued",
  url: "",
  dateSubmitted: "2026-01-01",
  lastModified: "",
  editors: [],
  decisions: [],
  reviews: [],
  findings: [],
  effectiveFinalState: "",
  effectiveQueue: "daily",
  effectiveAttention: "normal",
  findingDigest: "digest",
  provenance: {},
});

const meta = (): MonitoringMeta => ({
  generatedAt: "",
  availability: "available",
  conferences: [],
  aggregates: {
    metrics: {},
    triage: {},
    dailyRequiringAttention: 0,
    statuses: [],
    editors: [],
    total: 0,
  },
});

// A fake backing store that pages by cursor the way the server does: a cursor
// names the row just past the batch it was encoded from, so a page starts at
// the row after that id.
function makeSource(ids: string[]) {
  const store = { ids: [...ids] };
  const loadPage = vi.fn(
    async ({
      limit,
      cursor,
    }: {
      limit: number;
      cursor?: string;
      query: string;
    }): Promise<SubmissionPage> => {
      const start = cursor ? store.ids.indexOf(cursor) + 1 : 0;
      const slice = store.ids.slice(start, start + limit);
      const last = slice[slice.length - 1];
      const more = start + limit < store.ids.length;
      return {
        items: slice.map(row),
        nextCursor: more && last ? last : "",
        total: store.ids.length,
      };
    },
  );
  return { store, loadPage, loadMeta: vi.fn(async () => meta()) };
}

function listOptions(source: ReturnType<typeof makeSource>) {
  return {
    mode: "paged" as const,
    query: "",
    limit: 2,
    enabled: true,
    loadPage: source.loadPage,
    loadMeta: source.loadMeta,
    onFailure: () => {},
  };
}

describe("useSubmissionList", () => {
  beforeEach(() => vi.clearAllMocks());

  // This is the defect `drop` used to work around. Replaying the *stored*
  // cursors emits the row that slid up into the previous batch twice, because
  // each stored cursor still points just past the batch tail it was encoded
  // from. Chaining each fresh response's cursor tiles the current data exactly.
  it("replays without emitting a row twice after one leaves the list", async () => {
    const source = makeSource(["a", "b", "c", "d", "e"]);
    const { result } = renderHook(() =>
      useSubmissionList({ ...listOptions(source), enabled: false }),
    );

    await act(async () => await result.current.reload());
    await act(async () => await result.current.loadMore());
    expect(result.current.items.map((item) => item.ojsSubmissionId)).toEqual([
      "a",
      "b",
      "c",
      "d",
    ]);

    // A curation action removes "b", so "c", "d" and "e" each slide up one.
    source.store.ids = ["a", "c", "d", "e"];
    await act(async () => await result.current.replay());

    const ids = result.current.items.map((item) => item.ojsSubmissionId);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids).toEqual(["a", "c", "d", "e"]);
  });

  it("keeps the batch count across a replay", async () => {
    const source = makeSource(["a", "b", "c", "d", "e", "f"]);
    const { result } = renderHook(() =>
      useSubmissionList({ ...listOptions(source), enabled: false }),
    );

    await act(async () => await result.current.reload());
    await act(async () => await result.current.loadMore());
    expect(result.current.items).toHaveLength(4);

    await act(async () => await result.current.replay());
    expect(result.current.items).toHaveLength(4);
    expect(result.current.hasMore).toBe(true);
  });

  // The stale-batch rule, stated once instead of four times: a response that
  // resolves after its generation passed belongs to a query nobody is looking
  // at, so it must not be committed.
  it("discards a batch that resolves after a newer load started", async () => {
    const source = makeSource(["a", "b", "c", "d"]);
    let release: (() => void) | undefined;
    source.loadPage.mockImplementationOnce(async () => {
      await new Promise<void>((resolve) => {
        release = resolve;
      });
      return { items: [row("stale")], nextCursor: "", total: 1 };
    });

    const { result } = renderHook(() => useSubmissionList(listOptions(source)));
    await act(async () => {
      const slow = result.current.reload();
      await result.current.reload();
      release?.();
      await slow;
    });

    expect(
      result.current.items.map((item) => item.ojsSubmissionId),
    ).not.toContain("stale");
  });

  it("loads a side screen as one batch and reports when it has looked", async () => {
    const source = makeSource(["a", "b", "c"]);
    const { result } = renderHook(() =>
      useSubmissionList({ ...listOptions(source), mode: "batch" }),
    );

    await waitFor(() => expect(result.current.loaded).toBe(true));
    expect(source.loadPage).toHaveBeenCalledTimes(1);
    expect(result.current.hasMore).toBe(false);
  });

  it("does not load while disabled", async () => {
    const source = makeSource(["a"]);
    renderHook(() =>
      useSubmissionList({ ...listOptions(source), enabled: false }),
    );
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(source.loadPage).not.toHaveBeenCalled();
  });
});
