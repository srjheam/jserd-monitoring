import { describe, expect, it } from "vitest";
import { editorSlug, parseWireQuery, toWireQuery } from "../src/lib/query";
import { nameSlugCases, searchQueryCases } from "./generated/wire-grammar";

// The search wire grammar and the name slug each have a parser in Go and a
// parser in TypeScript, because each side has to read the form the other
// writes. Two parsers cannot be one source of truth, so the answers are: the
// corpus below is generated from the Go parsers, and these tests fail the
// moment the client disagrees with one of them.
//
// Keep the client parser aligned with the server grammar corpus.

describe("parseWireQuery agrees with the server's parseSearchQuery", () => {
  it.each(searchQueryCases)("$raw", (testCase) => {
    const query = parseWireQuery(testCase.raw);
    const editorChips = query.chips.filter((chip) => chip.tag === "editor");
    const folderChips = query.chips.filter((chip) => chip.tag === "folder");

    expect(editorChips.map((chip) => chip.label)).toEqual(testCase.chipNames);
    expect(editorChips.map((chip) => chip.value)).toEqual(testCase.chipSlugs);
    expect(folderChips.map((chip) => chip.value)).toEqual(
      testCase.folder === "" ? [] : [testCase.folder],
    );
    expect(query.text).toEqual(testCase.free);
  });
});

describe("editorSlug agrees with the server's normalizer.NameSlug", () => {
  it.each(nameSlugCases)("$raw", (testCase) => {
    expect(editorSlug(testCase.raw)).toEqual(testCase.slug);
  });
});

// A wire form the server accepts has to survive a trip through the client and
// read the same way to the server afterwards. This catches a round trip that
// parses correctly but writes a form the server reads differently.
describe("a parsed wire form writes back to a form that parses the same", () => {
  it.each(searchQueryCases)("$raw", (testCase) => {
    const once = parseWireQuery(testCase.raw);
    expect(parseWireQuery(toWireQuery(once))).toEqual(once);
  });
});
