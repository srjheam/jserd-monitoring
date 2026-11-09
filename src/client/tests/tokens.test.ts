import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync("src/app/globals.css", "utf8");

const token = (name: string) => {
  const found = css.match(
    new RegExp(`--ojs-${name}:\\s*#([0-9a-f]{3,6})\\s*;`, "i"),
  );
  if (!found) throw new Error(`missing token --ojs-${name}`);
  const value = found[1];
  return value.length === 3
    ? `#${[...value].map((digit) => digit + digit).join("")}`
    : `#${value}`;
};

const channel = (value: number) =>
  value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;

const luminance = (hex: string) => {
  const [r, g, b] = [1, 3, 5].map((at) =>
    channel(Number.parseInt(hex.slice(at, at + 2), 16) / 255),
  );
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};

const contrast = (a: string, b: string) => {
  const [high, low] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (high + 0.05) / (low + 0.05);
};

describe("ojs tokens", () => {
  it("keeps every required text pair at 4.5:1", () => {
    const pairs: [string, string][] = [
      [token("text-muted"), token("canvas")],
      [token("text-muted"), token("surface-muted")],
      ["#ffffff", token("warnable")],
      ["#ffffff", token("primary")],
      [token("text"), token("canvas")],
      [token("text"), token("surface-muted")],
    ];
    for (const [text, background] of pairs)
      expect({
        pair: `${text} on ${background}`,
        passes: contrast(text, background) >= 4.5,
      }).toEqual({ pair: `${text} on ${background}`, passes: true });
  });

  it("drops every colour of the teal palette", () => {
    const teal = [
      "#17302f",
      "#58706d",
      "#f7fbfa",
      "#b8d6d2",
      "#087a7f",
      "#dceae7",
      "#edf7f5",
      "#d9efeb",
      "#e4f3f0",
      "#e9efee",
      "#385452",
      "#102c2a",
      "#8d2525",
      "#a33535",
    ];
    for (const value of teal) expect(css.toLowerCase()).not.toContain(value);
  });
});
