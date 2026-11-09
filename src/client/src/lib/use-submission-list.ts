import { useCallback, useEffect, useRef, useState } from "react";
import type { MonitoringMeta, Submission, SubmissionPage } from "./monitoring";

export type ListMode = "paged" | "batch";

export type LoadPage = (request: {
  limit: number;
  cursor?: string;
  query: string;
}) => Promise<SubmissionPage>;

export type SubmissionListOptions = {
  // "paged" is the work list: a keyset window that grows with loadMore.
  // "batch" is a side screen: one request, no cursor, because Muted has to
  // group its whole set before it can render a cluster.
  mode: ListMode;
  query: string;
  limit: number;
  enabled: boolean;
  loadPage: LoadPage;
  loadMeta: () => Promise<MonitoringMeta>;
  onFailure: (error: unknown) => void;
};

type Batch = {
  items: Submission[];
  total: number;
  nextCursor: string;
};

/**
 * The one place the dashboard loads submissions.
 *
 * It owns three rules that used to be restated in every load routine:
 *
 * 1. The stale-batch guard. A response that resolves after a newer load
 *    started belongs to a query nobody is looking at, so it is discarded.
 *    This runs both ways. A load-more that started first is discarded by its
 *    ticket. A load-more that starts *during* a reload is refused outright,
 *    because its cursor was encoded before the reload's own rows moved, and a
 *    ticket cannot tell that batch from a current one.
 * 2. The replay. Refetching the batches loaded so far chains each *fresh*
 *    response's cursor. Replaying the *stored* cursors is what used to emit a
 *    row twice: each stored cursor still points just past the batch tail it
 *    was encoded from, so a row that slid up after a removal appeared in both
 *    the batch it moved into and the batch it came from.
 * 3. The meta travels with the list. Every caller wants both, and a footer
 *    count fetched apart can disagree with the rows above it.
 */
export function useSubmissionList(options: SubmissionListOptions) {
  const { mode, query, limit, enabled, loadPage, loadMeta, onFailure } =
    options;
  const [items, setItems] = useState<Submission[]>([]);
  const [meta, setMeta] = useState<MonitoringMeta | null>(null);
  const [total, setTotal] = useState(0);
  const [nextCursor, setNextCursor] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const [loaded, setLoaded] = useState(false);
  // How many batches are on screen. It replaces the array of stored cursors:
  // a replay derives its cursors fresh, so only the count has to survive.
  const batchCount = useRef(1);
  const generation = useRef(0);
  const loadingRef = useRef(false);
  // True while a reload or a replay is in flight. The cursor on screen during
  // that window was encoded before the mutation that started the reload, so a
  // batch fetched from it can repeat a row that has since moved up.
  const reloadingRef = useRef(false);
  // The cursor the last load-more asked for. The scroll sentinel can fire
  // again after a load-more finishes but before React has re-rendered with the
  // new cursor. That second fire reads the old cursor from the old closure and
  // appends the same batch twice, which is where the duplicate rows came from.
  const requestedCursor = useRef("");

  const commit = useCallback(
    (batch: Batch, nextMeta: MonitoringMeta, batches: number) => {
      batchCount.current = batches;
      // A fresh window: the next load-more asks for whatever cursor this
      // batch ends on, even when that repeats the last one asked for.
      requestedCursor.current = "";
      setItems(batch.items);
      setTotal(batch.total);
      setNextCursor(batch.nextCursor);
      setMeta(nextMeta);
      setLoaded(true);
    },
    [],
  );

  // fetchBatches walks `count` batches from the start, chaining each fresh
  // response's cursor. One batch is a plain reload; more than one is a replay.
  const fetchBatches = useCallback(
    async (count: number): Promise<Batch> => {
      let cursor: string | undefined;
      let items: Submission[] = [];
      let page: SubmissionPage | undefined;
      for (let index = 0; index < count; index++) {
        page = await loadPage({ limit, cursor, query });
        items = items.concat(page.items);
        cursor = page.nextCursor;
        if (!cursor) break;
      }
      return {
        items,
        total: page?.total ?? 0,
        nextCursor: page?.nextCursor ?? "",
      };
    },
    [limit, loadPage, query],
  );

  const run = useCallback(
    async (batches: number) => {
      const ticket = ++generation.current;
      reloadingRef.current = true;
      try {
        const nextMeta = await loadMeta();
        const batch = await fetchBatches(batches);
        if (ticket !== generation.current) return;
        commit(batch, nextMeta, batches);
      } catch (error) {
        onFailure(error);
      } finally {
        // A newer run took over, so it owns the flag and clears it itself.
        if (ticket === generation.current) reloadingRef.current = false;
      }
    },
    [commit, fetchBatches, loadMeta, onFailure],
  );

  // reload restarts from the first batch: a new query, or a screen change.
  const reload = useCallback(() => run(1), [run]);

  // replay refetches the batches already on screen, so a mutation that moved a
  // row is reflected without losing the reader's place.
  const replay = useCallback(
    () => run(mode === "batch" ? 1 : batchCount.current),
    [mode, run],
  );

  const loadMore = useCallback(async () => {
    if (mode === "batch" || !nextCursor || loadingRef.current) return;
    if (reloadingRef.current || requestedCursor.current === nextCursor) return;
    requestedCursor.current = nextCursor;
    loadingRef.current = true;
    const ticket = generation.current;
    setLoadingMore(true);
    try {
      const page = await loadPage({ limit, cursor: nextCursor, query });
      if (ticket !== generation.current) return;
      batchCount.current += 1;
      setItems((current) => [...current, ...page.items]);
      setTotal(page.total);
      setNextCursor(page.nextCursor);
    } catch (error) {
      onFailure(error);
    } finally {
      loadingRef.current = false;
      setLoadingMore(false);
    }
  }, [limit, loadPage, mode, nextCursor, onFailure, query]);

  // A different screen is a different list, so its rows start empty. Without
  // this, the new screen renders the previous screen's rows until its own
  // first batch lands: the Muted screen would cluster work-list rows, which
  // carry no tracking, into one "no conference" group.
  //
  // `loaded` going back to false is what stops an empty screen from claiming
  // "nothing is muted" before it has looked.
  // biome-ignore lint/correctness/useExhaustiveDependencies: keyed on list identity, not on the values this resets
  useEffect(() => {
    setItems([]);
    setTotal(0);
    setNextCursor("");
    setLoaded(false);
    batchCount.current = 1;
    requestedCursor.current = "";
  }, [mode, loadPage]);

  // A committed query change restarts the current screen, after a short
  // debounce. Typing never lands here: the combobox commits, it does not
  // report every keystroke.
  useEffect(() => {
    if (!enabled) return;
    const timer = window.setTimeout(() => void reload(), 250);
    return () => window.clearTimeout(timer);
  }, [enabled, reload]);

  return {
    items,
    meta,
    total,
    loaded,
    loadingMore,
    hasMore: mode !== "batch" && Boolean(nextCursor),
    reload,
    replay,
    loadMore,
  };
}
