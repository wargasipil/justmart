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
export default defineMain({
  stories: ["../src/**/*.stories.@(ts|tsx)"],

  addons: ["@storybook/addon-docs", "@storybook/addon-a11y"],

  framework: { name: "@storybook/react-vite", options: {} },

  // This runs on shop-adjacent dev machines; don't phone home by default.
  core: { disableTelemetry: true },

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
