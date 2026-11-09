import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// jsdom ships no matchMedia, and the top bar asks it for the compact layout.
window.matchMedia = (query: string) =>
  ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }) as unknown as MediaQueryList;

afterEach(cleanup);
