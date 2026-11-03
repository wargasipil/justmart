import type { HttpHandler } from "msw";
import type { ReactNode } from "react";
import { Route, Routes } from "react-router-dom";

import AppShell from "../components/AppShell";
import { ProductService } from "../gen/inventory_iface/v1/product_connect";
import { SettingsService } from "../gen/settings_iface/v1/settings_connect";
import { WarehouseService } from "../gen/warehouse_iface/v1/warehouse_connect";
import { RETAIL_CATALOG, WAREHOUSES } from "../routes/dev/fixtures";
import { mockRpc } from "../routes/dev/storyMocks";

// The frame a screen story renders a route in: screens/<device>/* show the
// whole app as a user sees it — the real AppShell around the page: on desktop
// the rail + TopBar (warehouse picker, low-stock bell, user menu); on a phone
// the bottom bar + Menu drawer, and no TopBar. A scenario declares its routes
// once and the desktop/mobile files only pick a viewport.

export function screenRender(routes: ReactNode) {
  return function AppScreen() {
    return (
      <Routes>
        <Route element={<AppShell />}>{routes}</Route>
      </Routes>
    );
  };
}

/** What the shell itself reads, regardless of the page inside it. */
export function shellHandlers(): HttpHandler[] {
  const low = RETAIL_CATALOG.filter((p) => p.active && p.readyStock <= 10n);
  return [
    mockRpc(WarehouseService, "listUserWarehouses", {
      warehouses: WAREHOUSES,
      memberships: WAREHOUSES.map((w) => ({ userId: "", warehouseId: w.id, isDefault: w.isDefault })),
      total: WAREHOUSES.length,
    }),
    mockRpc(ProductService, "listLowStock", (req) => ({
      products: low.slice(req.offset, req.offset + (req.limit || 25)),
      threshold: 10,
      total: low.length,
    })),
    // DocumentTitle reads the public branding; "" = the built-in brand.
    mockRpc(SettingsService, "getBranding", { appTitle: "" }),
    mockRpc(SettingsService, "checkUpdate", { checked: true, enabled: true, currentVersion: "dev" }),
  ];
}

type WithMsw = { parameters?: { msw?: HttpHandler[] } & Record<string, unknown> };

/**
 * Add the shell's handlers to a scenario's own. MSW answers with the first
 * match, and a scenario's `msw` ends in mockApi's catch-all, so the shell's
 * go AFTER the scenario's handlers and BEFORE that catch-all: the shell's
 * reads are answered, and a scenario that mocks one of them itself (the
 * Profile page's single-warehouse cashier reads the same ListUserWarehouses
 * as the TopBar picker) wins over the shell default. Done per story, because
 * Storybook REPLACES an array parameter rather than merging it.
 */
export function withShell<T extends WithMsw>(story: T): T {
  const msw = story.parameters?.msw;
  if (!msw) return story;
  const own = msw.slice(0, -1);
  const catchAll = msw.slice(-1);
  return { ...story, parameters: { ...story.parameters, msw: [...own, ...shellHandlers(), ...catchAll] } };
}
