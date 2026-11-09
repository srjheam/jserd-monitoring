import { describe, expect, it } from "vitest";
import en from "../messages/en.json";
import ptBR from "../messages/pt-BR.json";
import { findingKinds } from "../src/lib/finding-kinds";

// The kind list is generated from the server catalog, so this asserts that a
// kind the server can send always has words to show. A missing key falls back
// to the raw slug in `row.tsx`, which reads as a bug rather than a label.
describe("every finding kind has a label in both locales", () => {
  const locales = { en, "pt-BR": ptBR } as const;

  for (const [name, messages] of Object.entries(locales)) {
    describe(name, () => {
      it.each(findingKinds)("%s has an attention label", (kind) => {
        expect(messages.enums.attention).toHaveProperty(kind);
      });

      it.each(findingKinds)("%s has a badge label", (kind) => {
        expect(messages.workList.badge).toHaveProperty(kind);
      });
    });
  }
});

it("gives the copyediting finding its stage-specific label", () => {
  expect(findingKinds).toContain("missing_copyeditor");
  expect(en.enums.attention.missing_copyeditor).toBe("No copyeditor assigned");
  expect(ptBR.enums.attention.missing_copyeditor).toBe("Sem editor de texto");
});
