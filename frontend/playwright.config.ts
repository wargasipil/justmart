import { defineConfig, devices } from "@playwright/test";

// Playwright config for browser E2E tests. Suite assumes the Vite dev server
// (`make web`) and the Go backend (`make run`) are running locally. baseURL must
// track vite.config.ts `server.port` (5175) — it read 5173 (Vite's stock
// default) for a while, which made every spec fail at the login setup step. Tests share the dev DB; worker count is pinned to 1 to keep
// writes serial.
//
// Run with `make test-browser` from the repo root. For interactive debugging:
//   cd frontend && npx playwright test --ui
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: process.env.CI ? "github" : [["list"], ["html", { open: "never" }]],

  use: {
    baseURL: "http://localhost:5175",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    actionTimeout: 10_000,
  },

  projects: [
    // One-shot login → writes auth tokens to tests/e2e/.auth/owner.json.
    {
      name: "setup",
      testMatch: /global\.setup\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: "http://localhost:5175",
      },
    },
    // Authenticated suite — loads the storage state produced by setup.
    {
      name: "chromium",
      dependencies: ["setup"],
      testIgnore: /global\.setup\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: "http://localhost:5175",
        storageState: "tests/e2e/.auth/owner.json",
      },
    },
  ],

  expect: { timeout: 5_000 },
});
