import { defineConfig, devices } from "@playwright/test";

// Config for the RELEASE VIDEO recorder (frontend/tests/release/*.spec.ts),
// which captures release/video/product-tour.webm.
//
// Separate from both playwright.config.ts and playwright.faq.config.ts because
// this one does not point at the dev stack at all. It records against a
// throwaway demo instance — its own binary, its own SQLite file, its own port —
// seeded by release/seed-demo.mjs. That isolation is the point: the dev
// database is full of half-finished test rows, and none of them belong in
// something published to YouTube.
//
// It asserts nothing and must never run as part of `make test-browser`.
//
// Run with `make release-video`. See release/README.md.
export default defineConfig({
  testDir: "./tests/release",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  // A tour runs for minutes by design — captions hold, the cursor glides.
  timeout: 15 * 60_000,

  use: {
    ...devices["Desktop Chrome"],
    baseURL: process.env.JUSTMART_DEMO_URL ?? "http://127.0.0.1:8099",
    // The recorder opens its own context with recordVideo so it can name the
    // output file; per-test artifacts would only duplicate it.
    trace: "off",
    screenshot: "off",
    video: "off",
    actionTimeout: 20_000,
  },

  expect: { timeout: 15_000 },
});
