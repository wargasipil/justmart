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

  // addon-vitest turns every story into a test (`npx vitest --project=storybook`,
  // or the run button in Storybook's own UI): it mounts the story in a real
  // Chromium and fails on a render error, plus its `play` function once stories
  // start having one. Its Vitest project lives in vite.config.ts — see there for
  // why it extends the app's config rather than standing alone.
  addons: [
    "@storybook/addon-docs",
    "@storybook/addon-a11y",
    "msw-storybook-addon",
    "@storybook/addon-vitest",
  ],

  // Serves MSW's mockServiceWorker.js at the root. It lives in .storybook/public
  // rather than the app's public/ on purpose: Vite copies public/ into dist,
  // which is embedded into the production binary — a mock worker has no
  // business shipping to a shop.
  staticDirs: ["./public"],

  framework: { name: "@storybook/react-vite", options: {} },

  // This runs on shop-adjacent dev machines; don't phone home by default.
  //
  // allowedHosts: Storybook 10 rejects any Host / WebSocket Origin that isn't
  // localhost — iframe.html itself answers a bare 403 "Invalid host", so the
  // manager loads, finds the story in the index, and then sits on "Loading
  // story..." forever with nothing in the console saying why. That is what a
  // remote-access port proxy looks like from the browser: the Google Cloud
  // Workstations proxy (6006-<ws>.cloudworkstations.dev), a VS Code dev tunnel
  // (<name>-6006.<region>.devtunnels.ms), ngrok, a LAN IP. `true` accepts any
  // Host instead of naming each one, so the bench is reachable through
  // whatever tunnel is in front of it today.
  //
  // The check it disables is DNS-rebinding protection, and turning it off is
  // fine HERE and nowhere else: this is the dev bench, it serves fixtures and
  // stories with no credentials, and it is not part of any build the shop
  // runs. Do NOT copy this to the app's vite.config.ts — `make web` proxies
  // /api to a logged-in backend. builder-vite forwards this to Vite's own host
  // check too. Read at boot: a change here needs a Storybook restart.
  core: { disableTelemetry: true, allowedHosts: true },

  // Storybook loads the project's own vite.config.ts, so the `/api` -> :8080
  // proxy comes along for free. That is what makes the three server-backed
  // components (SupplierSelect, BatchSelect, ProductPickerDialog) work here
  // when `make run` is up — they ARE a backend search, so a fixture list would
  // demo the opposite of the thing. Everything else is server-free.
  viteFinal: (config) => {
    // The app pins server.port 5175 for `make web`; Storybook must keep its own
    // port (6006) or the two fight over the same socket.
    if (config.server) delete config.server.port;

    // builder-vite hands Vite `server.hmr = { port: <storybook port>, server }`,
    // and Vite bakes that port into the HMR client as an ABSOLUTE socket URL:
    // `wss://<page host>:6006/`. On localhost that is right and on any proxied
    // host it is wrong — a VS Code dev tunnel serves the page on 443, so the
    // socket dials a port the tunnel does not listen on and HMR never connects
    // (the page still renders; edits just stop reaching it).
    //
    // Dropping the port is what makes both work with no per-host setting:
    // Vite's client falls back to the port the page itself was loaded from
    // (6006 locally, none => 443 through the tunnel). It only reaches the
    // fallback when the port is absent, and `hmr.server` stays set, so the
    // socket still rides Storybook's own server rather than a second one.
    if (config.server?.hmr && typeof config.server.hmr === "object") {
      delete config.server.hmr.port;
    }
    return config;
  },
});
