import { describe, expect, it } from "vitest";
import {
  editorSlug,
  emptyQuery,
  folderChipScreen,
  folderLabel,
  parseWireQuery,
  reconcileQuery,
  tagRegistry,
  toWireQuery,
} from "../src/lib/query";

describe("editorSlug", () => {
  it("mirrors the backend: lowercase, letters/digits kept, space and hyphen collapse to one hyphen", () => {
    expect(editorSlug("Anderson S.")).toBe("anderson-s");
    expect(editorSlug("Casey Lee")).toBe("casey-lee");
    expect(editorSlug("Jean-Pierre")).toBe("jean-pierre");
    // A letter with a diacritic is still a unicode letter and survives; an
    // apostrophe is punctuation and is dropped outright (not turned into a
    // separator), the same as the Go implementation it mirrors.
    expect(editorSlug("Ana D'Ávila")).toBe("ana-dávila");
  });
});

describe("the wire codec", () => {
  it("writes editor chips quoted with the raw label and folder chips as a bare slug", () => {
    expect(toWireQuery(emptyQuery)).toBe("");
    expect(toWireQuery({ chips: [], text: "code smell" })).toBe("code smell");
    expect(
      toWireQuery({
        chips: [
          { tag: "folder", value: "muted", label: "Muted" },
          { tag: "editor", value: "anderson-s", label: "Anderson S." },
        ],
        text: " code smell ",
      }),
    ).toBe('folder:muted editor:"Anderson S." code smell');
  });

  it("round-trips through parseWireQuery", () => {
    const wire = 'folder:archived editor:"Jordan Reed" title words';
    const parsed = parseWireQuery(wire);
    expect(parsed).toEqual({
      chips: [
        { tag: "folder", value: "archived", label: "Archived" },
        { tag: "editor", value: "jordan-reed", label: "Jordan Reed" },
      ],
      text: "title words",
    });
    expect(toWireQuery(parsed)).toBe(wire);
  });

  it("drops an unrecognized folder slug instead of throwing", () => {
    expect(parseWireQuery("folder:bogus title")).toEqual({
      chips: [],
      text: "title",
    });
  });

  it("keeps whitespace inside a quoted editor name", () => {
    expect(parseWireQuery('editor:"Ana Maria" rest').chips).toEqual([
      { tag: "editor", value: "ana-maria", label: "Ana Maria" },
    ]);
  });
});

describe("reconcileQuery", () => {
  it("prefills folder:<route folder> when ?q is absent", () => {
    expect(reconcileQuery(null, undefined)).toEqual({
      chips: [{ tag: "folder", value: "work", label: "Work" }],
      text: "",
    });
    expect(reconcileQuery(null, "muted")).toEqual({
      chips: [{ tag: "folder", value: "muted", label: "Muted" }],
      text: "",
    });
  });

  it("trusts ?q as-is, including a folder: token that disagrees with the route", () => {
    const query = reconcileQuery("folder:archived", "muted");
    expect(query.chips).toEqual([
      { tag: "folder", value: "archived", label: "Archived" },
    ]);
    expect(folderChipScreen(query)).toBe("archived");
  });

  it("treats a ?q with no folder: token as genuine all-folder search", () => {
    const query = reconcileQuery("editor:anderson-s", undefined);
    expect(query.chips.some((chip) => chip.tag === "folder")).toBe(false);
    expect(folderChipScreen(query)).toBeUndefined();
  });
});

describe("folderChipScreen", () => {
  it("is undefined for no chip or an explicit folder:work", () => {
    expect(folderChipScreen(emptyQuery)).toBeUndefined();
    expect(
      folderChipScreen({
        chips: [{ tag: "folder", value: "work", label: "Work" }],
        text: "",
      }),
    ).toBeUndefined();
  });
  it("names the route for every other folder slug", () => {
    for (const slug of ["muted", "up_to_date", "archived"] as const) {
      expect(
        folderChipScreen({
          chips: [{ tag: "folder", value: slug, label: folderLabel(slug) }],
          text: "",
        }),
      ).toBe(slug);
    }
  });
});

describe("the tag registry", () => {
  it("has exactly editor and folder, folder exclusive with the four spec slugs", () => {
    expect(Object.keys(tagRegistry).sort()).toEqual(["editor", "folder"]);
    expect(tagRegistry.folder.exclusive).toBe(true);
    expect(tagRegistry.editor.exclusive).toBe(false);
    const folderSuggestions = tagRegistry.folder.suggestions({
      query: emptyQuery,
      typed: "",
      route: undefined,
      editorFrequencies: [],
    });
    expect(folderSuggestions.map((s) => s.value)).toEqual([
      "work",
      "up_to_date",
      "muted",
      "archived",
    ]);
  });

  it("agrees with the backend's slugify for editor chips", () => {
    expect(tagRegistry.editor.slugify("Anderson S.")).toBe("anderson-s");
  });

  it("ranks editor suggestions from the already-fetched frequency list, filtered by typed text and existing chips", () => {
    const suggestions = tagRegistry.editor.suggestions({
      query: {
        chips: [{ tag: "editor", value: "jordan-reed", label: "Jordan Reed" }],
        text: "",
      },
      typed: "and",
      route: undefined,
      editorFrequencies: [
        { slug: "anderson-s", label: "Anderson S.", count: 5 },
        { slug: "jordan-reed", label: "Jordan Reed", count: 3 },
        { slug: "casey-lee", label: "Casey Lee", count: 1 },
      ],
    });
    // "Jordan Reed" is already a chip, and "Casey Lee" does not
    // match "and": only "Anderson S." survives both filters.
    expect(suggestions).toEqual([
      { tag: "editor", value: "anderson-s", label: "Anderson S." },
    ]);
  });

  it("moves the folder onCommit navigation target to the committed slug, work meaning the Work route", () => {
    expect(tagRegistry.folder.onCommit?.("archived", emptyQuery)).toEqual({
      navigate: "archived",
    });
    expect(tagRegistry.folder.onCommit?.("work", emptyQuery)).toEqual({
      navigate: undefined,
    });
  });
});
