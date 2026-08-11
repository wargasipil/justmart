import { defineConfig, devices } from "@playwright/test";

// Config for the FAQ tutorial RECORDERS (frontend/tests/faq/*.spec.ts), which
// capture faq/videos/<question>/tutorial.webm. Separate from playwright.config.ts
// because these are not tests: they assert nothing, they run for minutes by
// design, and they must never run as part of `make test-browser`.
//
// Unlike the test suite, this config STARTS THE SERVERS ITSELF (webServer
// below) so recording is one command — `make faq-video` — with nothing to set
// up first and nothing left running after. It still writes to the dev DB; each
// recorder seeds its own scenario and cleans up after itself. See faq/README.md.
//
// Run with `make faq-video` (optionally `q=<slug>` to record just one).
export default defineConfig({
  testDir: "./tests/faq",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  // A recording is deliberately slow — captions hold, the cursor glides, and
  // the whole point is that a human can follow it.
  timeout: 5 * 60_000,

  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://localhost:5175",
    // The recorder opens its own context with recordVideo so it can name the
    // output file; the per-test artifacts would only duplicate it.
    trace: "off",
    screenshot: "off",
    video: "off",
    actionTimeout: 15_000,
  },

  expect: { timeout: 10_000 },

  // Bring up the backend and the Vite dev server, wait for both, and tear them
  // down when the run ends. `reuseExistingServer` means an already-running
  // `make dev` is used as-is rather than fought over the port.
  webServer: [
    {
      // Stamped with a real, older release version instead of the default "dev".
      // The Settings ▸ Updates panel compares this against the latest GitHub
      // release, and "dev" is unparseable — it can never report an update, so
      // the how-do-i-update-the-app recorder would have nothing to film. A shop
      // never runs an unstamped build either, so this is also the more honest
      // state to record. Keep it BELOW every published release.
      command: 'go -C ../backend run -ldflags "-X main.version=1.1.0" ./cmd/server',
      url: "http://127.0.0.1:8080/healthz",
      reuseExistingServer: true,
      timeout: 180_000, // first run compiles the whole Go binary
      stdout: "ignore",
      stderr: "pipe",
      env: {
        // Relative to the backend's CWD, matching the Makefile.
        JUSTMART_CONFIG: "../config.yaml",
        // Recording must never open a public tunnel to the dev database. The
        // env var only gained the ability to say "off" for this reason — an
        // empty value means "not set", so a token sitting in config.yaml could
        // not otherwise be overridden. See internal/config/config.go.
        JUSTMART_CLOUDFLARE_TUNNEL_TOKEN: "off",
      },
    },
    {
      command: "npm run dev",
      url: "http://localhost:5175",
      reuseExistingServer: true,
      timeout: 120_000,
      stdout: "ignore",
      stderr: "pipe",
    },
  ],
});
