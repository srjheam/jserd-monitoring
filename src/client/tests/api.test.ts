import { afterEach, describe, expect, it, vi } from "vitest";
import {
  allFolderParams,
  appendJournalEntry,
  loadActivity,
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
  saveTracking,
  sendBackToWork,
  setMuted,
} from "../src/lib/api";
import type { Submission } from "../src/lib/monitoring";

// The list endpoint defaults its own `queue` param to `"daily"` when
// nothing else says otherwise (parseListQuery in query.go), so a genuine
// all-folder search — no `folder:` token anywhere in the wire query — must
// say `queue=all` itself, or it would silently narrow to the daily queue.
describe("allFolderParams", () => {
  it("asks for every queue when the wire query names no folder", () => {
    expect(allFolderParams("")).toEqual({ queue: "all" });
    expect(allFolderParams('editor:"Jordan Reed" job ads')).toEqual({
      queue: "all",
    });
  });

  it("leaves the queue to the folder: token when one is present", () => {
    expect(allFolderParams("folder:muted")).toEqual({});
    expect(allFolderParams('editor:"Jordan Reed" folder:archived')).toEqual({});
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

const submission = (id: string): Submission =>
  ({ ojsSubmissionId: id }) as Submission;

describe("restoreSession", () => {
  it("reports signedOut with no stored token", async () => {
    vi.spyOn(pb.authStore, "token", "get").mockReturnValue("");
    await expect(restoreSession()).resolves.toBe("signedOut");
  });

  it("clears an expired token and reports expired", async () => {
    vi.spyOn(pb.authStore, "token", "get").mockReturnValue("stale");
    vi.spyOn(pb.authStore, "isValid", "get").mockReturnValue(false);
    const clear = vi.spyOn(pb.authStore, "clear").mockImplementation(() => {});
    await expect(restoreSession()).resolves.toBe("expired");
    expect(clear).toHaveBeenCalledOnce();
  });

  it("reports signedIn when the server accepts the refresh", async () => {
    vi.spyOn(pb.authStore, "token", "get").mockReturnValue("valid");
    vi.spyOn(pb.authStore, "isValid", "get").mockReturnValue(true);
    const authRefresh = vi.fn().mockResolvedValue({});
    vi.spyOn(pb, "collection").mockReturnValue({ authRefresh } as never);
    await expect(restoreSession()).resolves.toBe("signedIn");
  });

  it("clears the store and reports expired when the refresh fails", async () => {
    vi.spyOn(pb.authStore, "token", "get").mockReturnValue("valid");
    vi.spyOn(pb.authStore, "isValid", "get").mockReturnValue(true);
    const authRefresh = vi.fn().mockRejectedValue(new Error("gone"));
    vi.spyOn(pb, "collection").mockReturnValue({ authRefresh } as never);
    const clear = vi.spyOn(pb.authStore, "clear").mockImplementation(() => {});
    await expect(restoreSession()).resolves.toBe("expired");
    expect(clear).toHaveBeenCalledOnce();
  });
});

describe("loadMonitoringMeta", () => {
  it("GETs the monitoring endpoint", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await loadMonitoringMeta();
    expect(send).toHaveBeenCalledWith("/api/jserd/monitoring", {
      method: "GET",
    });
  });
});

describe("loadSubmissionPage", () => {
  it("carries the cursor and query only when present", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await loadSubmissionPage({ limit: 25, query: "" });
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/submissions",
      expect.objectContaining({
        query: { limit: "25", sort: "priority", queue: "all" },
      }),
    );
  });

  it("passes a folder-scoped query and cursor through untouched", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await loadSubmissionPage({
      limit: 25,
      cursor: "c1",
      query: "folder:muted",
    });
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/submissions",
      expect.objectContaining({
        query: {
          limit: "25",
          sort: "priority",
          cursor: "c1",
          query: "folder:muted",
        },
      }),
    );
  });
});

describe("loadSideScreen", () => {
  it("orders the Archived screen by last activity", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await loadSideScreen("", "archived");
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/submissions",
      expect.objectContaining({
        query: expect.objectContaining({ sort: "activity", dir: "desc" }),
      }),
    );
  });

  it("keeps the server's plain id order for Muted and Up to date", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await loadSideScreen("editor:ana", "muted");
    const query = send.mock.calls[0]?.[1]?.query as Record<string, string>;
    expect(query.sort).toBeUndefined();
    expect(query.query).toBe("editor:ana");
  });
});

describe("loadEditorFrequencies", () => {
  it("sends only the scope fields that are present", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue([]);
    await loadEditorFrequencies({});
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/aggregates/editors",
      expect.objectContaining({ query: {} }),
    );

    await loadEditorFrequencies({ folder: "muted", text: "an" });
    expect(send).toHaveBeenLastCalledWith(
      "/api/jserd/monitoring/aggregates/editors",
      expect.objectContaining({ query: { folder: "muted", text: "an" } }),
    );
  });
});

describe("setMuted", () => {
  it("does nothing for a submission with no tracking row yet", async () => {
    const collection = vi.spyOn(pb, "collection");
    await setMuted(undefined, true);
    expect(collection).not.toHaveBeenCalled();
  });

  it("updates the existing tracking row's muted flag", async () => {
    const update = vi.fn().mockResolvedValue({});
    vi.spyOn(pb, "collection").mockReturnValue({ update } as never);
    await setMuted("track-1", true);
    expect(update).toHaveBeenCalledWith("track-1", { muted: true });
  });
});

describe("loadSubmission and loadActivity", () => {
  it("encode the submission id into the URL", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await loadSubmission("42/x");
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/submissions/42%2Fx",
      { method: "GET" },
    );

    await loadActivity("42/x", "panel");
    expect(send).toHaveBeenLastCalledWith(
      "/api/jserd/monitoring/submissions/42%2Fx/activity",
      { method: "GET", requestKey: "panel" },
    );
  });
});

describe("appendJournalEntry", () => {
  it("POSTs the entry body to the submission's journal", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await appendJournalEntry("42", "noted");
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/submissions/42/journal",
      { method: "POST", body: { body: "noted" } },
    );
  });
});

describe("saveTracking", () => {
  it("creates a tracking row when it has no id yet", async () => {
    const create = vi.fn().mockResolvedValue({});
    vi.spyOn(pb, "collection").mockReturnValue({ create } as never);
    await saveTracking(undefined, {
      ojsSubmissionId: "42",
      conference: "",
    });
    expect(create).toHaveBeenCalledWith({
      ojs_submission_id: "42",
      conference: "",
    });
  });

  it("updates the existing row by id otherwise", async () => {
    const update = vi.fn().mockResolvedValue({});
    vi.spyOn(pb, "collection").mockReturnValue({ update } as never);
    await saveTracking("track-1", {
      ojsSubmissionId: "42",
      conference: "conf-a",
    });
    expect(update).toHaveBeenCalledWith("track-1", {
      ojs_submission_id: "42",
      conference: "conf-a",
    });
  });
});

describe("markUpToDate and sendBackToWork", () => {
  it("POST to the submission's up-to-date and send-back-to-work routes", async () => {
    const send = vi.spyOn(pb, "send").mockResolvedValue({});
    await markUpToDate(submission("42"), "2026-07-01", "delegated");
    expect(send).toHaveBeenCalledWith(
      "/api/jserd/monitoring/submissions/42/up-to-date",
      expect.objectContaining({
        method: "POST",
        body: expect.objectContaining({
          until: "2026-07-01",
          reason: "delegated",
        }),
      }),
    );

    await sendBackToWork(submission("42"));
    expect(send).toHaveBeenLastCalledWith(
      "/api/jserd/monitoring/submissions/42/send-back-to-work",
      { method: "POST" },
    );
  });
});

describe("saveConference and removeConference", () => {
  it("creates, updates, and deletes conferences", async () => {
    const create = vi.fn().mockResolvedValue({});
    const update = vi.fn().mockResolvedValue({});
    const del = vi.fn().mockResolvedValue(true);
    const collection = vi.spyOn(pb, "collection").mockReturnValue({
      create,
      update,
      delete: del,
    } as never);

    await saveConference(undefined, "Conference A", "desc");
    expect(collection).toHaveBeenNthCalledWith(1, "conferences");
    expect(create).toHaveBeenCalledWith({
      name: "Conference A",
      description: "desc",
    });

    await saveConference("conf-1", "Conference B", "desc2");
    expect(collection).toHaveBeenNthCalledWith(2, "conferences");
    expect(update).toHaveBeenCalledWith("conf-1", {
      name: "Conference B",
      description: "desc2",
    });

    await removeConference("conf-1");
    expect(collection).toHaveBeenNthCalledWith(3, "conferences");
    expect(del).toHaveBeenCalledWith("conf-1");
  });
});
