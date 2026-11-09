import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  clampPageLimit,
  defaultPageLimit,
  maxPageLimit,
  pathForScreen,
  readPageLocation,
  screenFromPath,
  writePageLocation,
} from "../src/lib/monitoring";
import { emptyQuery } from "../src/lib/query";

describe("page location url state", () => {
  it("falls back to defaults when params are missing or invalid, prefilling folder:work", () => {
    const empty = readPageLocation(new URLSearchParams(""), "/");
    expect(empty).toEqual({
      limit: defaultPageLimit,
      screen: undefined,
      query: {
        chips: [{ tag: "folder", value: "work", label: "Work" }],
        text: "",
      },
    });
    const invalid = readPageLocation(
      new URLSearchParams("limit=0&article="),
      "/",
    );
    expect(invalid.limit).toBe(defaultPageLimit);
    expect(invalid.article).toBeUndefined();
  });
  it("reads article and clamped limits", () => {
    const location = readPageLocation(
      new URLSearchParams("limit=999&article=42"),
      "/",
    );
    expect(location.limit).toBe(maxPageLimit);
    expect(location.article).toBe("42");
    expect(clampPageLimit(-3)).toBe(defaultPageLimit);
    expect(clampPageLimit(7)).toBe(7);
  });
  it("writes back a reproducible query string without limit", () => {
    const params = new URLSearchParams("unrelated=1&limit=10");
    writePageLocation(params, { limit: 10, article: "42", query: emptyQuery });
    expect(params.toString()).toBe("unrelated=1&article=42");
    const cleared = new URLSearchParams(params);
    writePageLocation(cleared, { limit: 25, query: emptyQuery });
    expect(cleared.toString()).toBe("unrelated=1");
  });
  it("round-trips a committed query through ?q", () => {
    const params = new URLSearchParams();
    writePageLocation(params, {
      limit: 25,
      query: {
        chips: [
          { tag: "folder", value: "muted", label: "Muted" },
          { tag: "editor", value: "anderson-s", label: "Anderson S." },
        ],
        text: "code smell",
      },
    });
    expect(params.get("q")).toBe(
      'folder:muted editor:"Anderson S." code smell',
    );
    const location = readPageLocation(params, "/muted");
    expect(location.query.chips).toEqual([
      { tag: "folder", value: "muted", label: "Muted" },
      { tag: "editor", value: "anderson-s", label: "Anderson S." },
    ]);
    expect(location.query.text).toBe("code smell");
  });
});

describe("side screen routes", () => {
  it("maps a screen to a hyphenated path and back", () => {
    expect(pathForScreen()).toBe("/");
    expect(pathForScreen("muted")).toBe("/muted");
    expect(pathForScreen("up_to_date")).toBe("/up-to-date");
    expect(screenFromPath("/")).toBeUndefined();
    expect(screenFromPath("/muted")).toBe("muted");
    expect(screenFromPath("/up-to-date")).toBe("up_to_date");
    expect(screenFromPath("/unknown")).toBeUndefined();
  });
  it("reads the screen from the path, not the query string", () => {
    const location = readPageLocation(new URLSearchParams(""), "/up-to-date");
    expect(location.screen).toBe("up_to_date");
  });
});

it("keeps browser data access on PocketBase instead of OJS", () => {
  const clientSources = [
    "src/lib/api.ts",
    "src/components/monitoring-dashboard.tsx",
  ]
    .map((file) => readFileSync(file, "utf8"))
    .join("\n");
  expect(clientSources).not.toMatch(/https?:\/\/[^\s"']*ojs/i);
  expect(clientSources).not.toMatch(
    /collection\("ojs|OJS.*(?:update|create|delete)/i,
  );
});
