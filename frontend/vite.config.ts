/// <reference types="vitest/config" />
import path from "node:path";
import { fileURLToPath } from "node:url";

import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5175,
    proxy: {
      // The backend now serves Connect handlers UNDER /api (it strips the
      // prefix internally), matching how the embedded SPA is served in
      // production. So forward /api/* unchanged — no rewrite.
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },

  // Storybook's Vitest integration: every story becomes a test that mounts the
  // component in a real browser and fails on a render error (plus its `play`
  // function, once stories start having one). It lives HERE rather than in a
  // separate vitest.config.ts because `extends: true` reuses this file's
  // plugins + resolve — a story must build exactly like the app does, or the
  // bench passes on a tree the SPA build would reject.
  //
  // `test` is ignored by `vite build`, so the production bundle embedded into
  // the Go binary is unaffected.
  test: {
    projects: [
      {
        extends: true,
        plugins: [storybookTest({ configDir: path.join(dirname, ".storybook") })],
        test: {
          name: "storybook",
          browser: {
            enabled: true,
            headless: true,
            provider: "playwright",
            instances: [{ browser: "chromium" }],
            // PINNED, not ephemeral. Windows reserves several port ranges for
            // Hyper-V/WinNAT (`netsh interface ipv4 show excludedportrange
            // protocol=tcp`), and Vitest's random pick lands in one often
            // enough to fail the run with a bare
            // `listen EACCES ::1:<port>` that says nothing about why.
            // 6007 sits just above Storybook's 6006 and clear of those ranges.
            api: { port: 6007 },
          },
        },
      },
    ],
  },
});
