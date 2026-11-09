import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 180_000,
  // Every spec drives the same PocketBase instance and curates the same
  // submissions, so two specs at once read each other's writes.
  workers: 1,
  use: {
    baseURL: process.env.SMOKE_URL || "http://127.0.0.1:3100",
    headless: true,
  },
});
