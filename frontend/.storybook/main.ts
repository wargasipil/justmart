import { defineMain } from "@storybook/react-vite/node";

// Storybook for the shared-component vocabulary in src/components/.
//
// It sits alongside the in-app dev gallery at /components (routes/dev/) rather
// than replacing it: the gallery ships inside the app and is what a developer
// hits while working in the product; Storybook is the isolated bench, with
// per-prop controls, a11y checks and a static build you can publish.
//
// Prose is NOT duplicated between the two — a story pulls its summary/notes
// from the same registry entry the gallery renders, via routes/dev/storyDocs.
//
// Page stories (routes/**) get their data from MSW: routes/dev/storyMocks.ts
// answers the Connect RPCs with typed fixtures, so a page renders with no
// backend and can stage loading / error / role / mode states on demand.
export default defineMain({
  stories: ["../src/**/*.stories.@(ts|tsx)"],

  addons: ["@storybook/addon-docs", "@storybook/addon-a11y", "msw-storybook-addon"],

  // Serves MSW's mockServiceWorker.js at the root. It lives in .storybook/public
  // rather than the app's public/ on purpose: Vite copies public/ into dist,
  // which is embedded into the production binary — a mock worker has no
  // business shipping to a shop.
  staticDirs: ["./public"],

  framework: { name: "@storybook/react-vite", options: {} },

  // This runs on shop-adjacent dev machines; don't phone home by default.
  //
  // allowedHosts: Storybook 10 rejects any Host / WebSocket Origin that isn't
  // localhost ("Invalid websocket origin"), which breaks it behind the Google
  // Cloud Workstations port proxy (6006-<ws>.cloudworkstations.dev). The
  // leading dot is a suffix match; builder-vite forwards the same list to
  // Vite's own host check, so this one entry covers both.
  core: { disableTelemetry: true, allowedHosts: [".cloudworkstations.dev"] },

  // Storybook loads the project's own vite.config.ts, so the `/api` -> :8080
  // proxy comes along for free. That is what makes the three server-backed
  // components (SupplierSelect, BatchSelect, ProductPickerDialog) work here
  // when `make run` is up — they ARE a backend search, so a fixture list would
  // demo the opposite of the thing. Everything else is server-free.
  viteFinal: (config) => {
    // The app pins server.port 5175 for `make web`; Storybook must keep its own
    // port (6006) or the two fight over the same socket.
    if (config.server) delete config.server.port;
    return config;
  },
});
