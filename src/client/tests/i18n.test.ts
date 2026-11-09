import { describe, expect, it } from "vitest";
import { defaultLocale, localeMessages } from "../src/i18n/locales";

const flatten = (value: unknown, prefix = ""): string[] =>
  typeof value === "object" && value !== null
    ? Object.entries(value).flatMap(([key, child]) =>
        flatten(child, prefix ? `${prefix}.${key}` : key),
      )
    : [prefix];

describe("locale messages", () => {
  it("defaults to English", () => {
    expect(defaultLocale).toBe("en");
  });

  it("holds the same key set in every locale", () => {
    const english = flatten(localeMessages.en).sort();
    for (const [locale, messages] of Object.entries(localeMessages)) {
      expect({ locale, keys: flatten(messages).sort() }).toEqual({
        locale,
        keys: english,
      });
    }
  });
});
