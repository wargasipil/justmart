import type { StoryObj } from "@storybook/react";
import type { HttpHandler } from "msw";
import { Navigate, Route } from "react-router-dom";

import { BackupService } from "../../gen/backup_iface/v1/backup_connect";
import { ConnectorService } from "../../gen/connector_iface/v1/connector_connect";
import { SettingsService } from "../../gen/settings_iface/v1/settings_connect";
import { UnitService } from "../../gen/unit_iface/v1/unit_connect";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc } from "../../routes/dev/storyMocks";
import Settings from "../../routes/Settings";
import SettingsBackups from "../../routes/settings/SettingsBackups";
import SettingsGeneral from "../../routes/settings/SettingsGeneral";
import SettingsPrinting from "../../routes/settings/SettingsPrinting";
import SettingsTunnel from "../../routes/settings/SettingsTunnel";
import SettingsUnits from "../../routes/settings/SettingsUnits";
import SettingsUpdates from "../../routes/settings/SettingsUpdates";
import { TUNNEL_SAVED, UPDATE_AVAILABLE, shopHandlers } from "./settingsShop";

// /settings — the OWNER-only panel, and the one page in the app that is really
// six pages: General · Units · Printing · Remote access · Backups · Updates,
// each its own route under RouteTabs (a rail on md+, a scrolling strip below).
// Rendered by screens/{desktop,mobile}/pages/settings/Settings.stories.tsx.
//
// Every story stages ALL six panels and differs only in which one it lands on
// and what state that panel is in — so the tabs are live and the whole page is
// walkable from any story. A per-panel story could not show that the rail and
// the page header belong to the parent route and survive every panel's own
// loading and empty states.
//
// The data is a writeable fake shop — see scenarios/settingsShop.ts for what it
// serves and why the mocks accept writes.
//
// There is no non-owner story: /settings sits behind
// `<ProtectedRoute requiredRole={OWNER}>` in main.tsx, and a story mounts the
// route itself, so a cashier story here would show a page the app never routes
// them to. Role gating for this page is the router's, not the panels'.

export const routes = (
  <Route path="/settings" element={<Settings />}>
    {/* The real router's index redirect — /settings alone is not a panel. */}
    <Route index element={<Navigate to="general" replace />} />
    <Route path="general" element={<SettingsGeneral />} />
    <Route path="units" element={<SettingsUnits />} />
    <Route path="printing" element={<SettingsPrinting />} />
    <Route path="tunnel" element={<SettingsTunnel />} />
    <Route path="backups" element={<SettingsBackups />} />
    <Route path="updates" element={<SettingsUpdates />} />
  </Route>
);

/**
 * A story's handlers: its own overrides first — MSW answers with the first
 * match — then the full six-panel set over a fresh shop.
 */
const settings = (...overrides: HttpHandler[]) => mockApi(...overrides, ...shopHandlers());

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: Settings,
  parameters: {
    router: { initialEntries: ["/settings/general"] },
    msw: settings(),
  },
  decorators: [withPageContext],
};

/** A story landing on `/settings/<path>`, with `overrides` over the fake shop. */
const at = (path: string, description: string, ...overrides: HttpHandler[]): StoryObj => ({
  parameters: {
    router: { initialEntries: [`/settings/${path}`] },
    msw: settings(...overrides),
    docs: { description: { story: description } },
  },
});

export const stories = {
  General: at(
    "general",
    "Where /settings opens (it redirects here). Business identity: the shop title, the " +
      "retail-vs-pharmacy mode and the low-stock threshold. Saving works — rename the shop and " +
      "the sidebar brand and the browser tab follow; switch to Pharmacy and save, and the nav " +
      "grows Resep while the catalog noun turns Produk into Obat. Both without a reload, which " +
      "is what the mutation invalidating the branding keys buys. The title cap is the server's " +
      "(60 characters): the field takes the 61st and Save answers with the field error.",
  ),
  Units: at(
    "units",
    "The global unit catalog: base units with their derivatives (strip ×10, box ×100). The " +
      "inline Add forms and the archive buttons write to the story's shop, so a base you add " +
      "stays and the \"factor must be greater than 1\" check is live.",
  ),
  UnitsEmpty: at(
    "units",
    "A fresh install with no unit catalog yet — the empty state that names the next step, " +
      "rather than an empty table.",
    mockRpc(UnitService, "listUnitBases", { bases: [], total: 0 }),
  ),
  Printing: at(
    "printing",
    "Connector mode: two connectors are online, and the default connector + printer the POS " +
      "Print button uses are chosen here. Below the rule, the printed receipt's header, footer " +
      "and paper width — which apply in every print mode.",
  ),
  PrintingNoConnector: at(
    "printing",
    "Connector mode with nothing connected. The pickers stay, and the empty line says where a " +
      "connector comes from: it runs next to the printer and dials in.",
    mockRpc(ConnectorService, "listConnectors", { connectors: [] }),
  ),
  PrintingUsb: at(
    "printing",
    "USB mode: receipts go straight to a printer installed on the server PC, so the picker " +
      "lists what Windows reports instead of a connector's printers.",
    mockRpc(SettingsService, "getPrintingInfo", {
      mode: "usb",
      localPrinters: ["EPSON TM-T82 Receipt", "Microsoft Print to PDF"],
    }),
  ),
  PrintingTcp: at(
    "printing",
    "TCP mode: the printer's address lives in config.yaml, so there is no picker at all — one " +
      "sentence saying where it is set and how to move off it. Only the receipt block is " +
      "editable here.",
    mockRpc(SettingsService, "getPrintingInfo", { mode: "tcp", localPrinters: [] }),
  ),
  Tunnel: at(
    "tunnel",
    "Remote access with no token anywhere: the shop is reachable on the LAN only. Try pasting " +
      "the whole `cloudflared service install <token>` line — the field takes the bare token, " +
      "and the fake shape-checks it exactly as the server does, so you get the real field error " +
      "instead of a stored credential nobody validated.",
  ),
  TunnelPendingRestart: at(
    "tunnel",
    "A token is saved, but this process booted without one. The badge reads Off and the status " +
      "line says why — saving schedules a change rather than making one, because the token is " +
      "read once at boot. The instruction to restart lives in the alert, and only there.",
    mockRpc(SettingsService, "getTunnelSettings", {
      ...TUNNEL_SAVED,
      active: false,
      source: "settings",
      restartRequired: true,
      supported: true,
    }),
  ),
  TunnelEnvOff: at(
    "tunnel",
    "The environment's off-switch, which the UI cannot beat: JUSTMART_CLOUDFLARE_TUNNEL_TOKEN " +
      "disables tunnels on this machine and the saved token stays stored but unused. It is " +
      "still shown — an owner told \"nothing configured\" would only save it again.",
    mockRpc(SettingsService, "getTunnelSettings", {
      ...TUNNEL_SAVED,
      active: false,
      source: "env_off",
      restartRequired: false,
      supported: true,
    }),
  ),
  TunnelUnsupported: at(
    "tunnel",
    "The same panel on a Linux or Docker deploy: cloudflared is fetched as a windows-amd64 exe " +
      "today, so the token saves but this server will not start one. Worth staging because the " +
      "field stays usable — the warning is the whole difference.",
    mockRpc(SettingsService, "getTunnelSettings", {
      ...TUNNEL_SAVED,
      active: false,
      source: "settings",
      restartRequired: true,
      supported: false,
    }),
  ),
  Backups: at(
    "backups",
    "A week of snapshots. Create backup prepends a row and Delete asks first — both move real " +
      "rows here, so the confirm dialog and the empty state are a few clicks apart.",
  ),
  BackupsEmpty: at(
    "backups",
    "Nothing backed up yet: a new shop's first sight of this tab, where the Create button has " +
      "to read as the obvious next step.",
    mockRpc(BackupService, "listBackups", { backups: [], total: 0 }),
  ),
  Updates: at(
    "updates",
    "A newer release is out on a portable Windows install: the version pair, the release notes, " +
      "Update now (download + verify + stage, applied on the next restart) and a revert to the " +
      "backup version. The shell's bell reads the same RPC, so it flags the update too.",
    mockRpc(SettingsService, "checkUpdate", UPDATE_AVAILABLE),
  ),
  UpdatesUpToDate: at("updates", "Nothing to do: the running build is the latest release."),
  UpdatesCheckFailed: at(
    "updates",
    "The GitHub lookup failed — offline, rate-limited or a 404. `checked: false` drops the " +
      "latest-version row entirely rather than showing a blank or a stale one, and the panel " +
      "says to check the connection.",
    mockRpc(SettingsService, "checkUpdate", { currentVersion: "v1.9.0", checked: false, enabled: true }),
  ),
  UpdatesDisabled: at(
    "updates",
    "Update checks turned off in config.yaml (update.disabled) — a shop whose IT does its own " +
      "rollouts. Nothing is fetched, and the panel says why it is quiet.",
    mockRpc(SettingsService, "checkUpdate", { currentVersion: "v1.9.0", checked: false, enabled: false }),
  ),
  Loading: at(
    "general",
    "GetSettings never answers: the panel's spinner. The tab rail and the page header render " +
      "anyway — they belong to the parent route — so the page never looks broken while one " +
      "panel waits.",
    mockRpc(SettingsService, "getSettings", { settings: {} }, { delay: "infinite" }),
  ),
};
