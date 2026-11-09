import { defineConfig, mergeConfig } from "vitest/config";
import baseConfig from "./vitest.config";

// Stryker mutates a sandbox copy of this directory alone, so a test that
// reads a file outside it (../server/internal/events/events.go) cannot run
// there. The test itself is unaffected: it still runs under plain `pnpm test`.
export default mergeConfig(
  baseConfig,
  defineConfig({
    test: {
      exclude: ["tests/submission-page.test.tsx"],
    },
  }),
);
