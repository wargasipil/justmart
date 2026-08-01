import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { create } from "zustand";

import { useBusinessMode } from "../queries/settings";

export type Crumb = { label: string; to?: string };

// The crumb trail is derived from the URL, NOT passed per page — pages used to
// hand-author it into <PageHeader> and it drifted from the router + sidebar.
// This registry is the single source of truth and mirrors the sidebar rail, so
// the trail always reads like the nav the user clicked through.
//
// - `path` is a pattern; `:x` matches any single segment.
// - A path starting with "@" is a VIRTUAL crumb: it has no page of its own
//   (e.g. the sidebar's "Inventaris" group), so it renders unlinked and is only
//   ever reached via a child's `parent`.
// - `dynamic: true` means the label comes from the page itself via
//   useCrumbLabel() (a product name, a sale no, …). Until the page registers
//   one the crumb is simply omitted — never a raw UUID, never a "—" flash.
type RouteDef = {
  path: string;
  labelKey?: string;
  /** Pharmacy-mode override for the catalog noun (Produk -> Obat). */
  pharmacyLabelKey?: string;
  /** Derives the i18n key from the matched `:param` segment. */
  labelKeyFromParam?: (segment: string) => string;
  dynamic?: boolean;
  parent?: string;
};

const ROUTES: RouteDef[] = [
  { path: "/", labelKey: "nav.dashboard" },

  { path: "/analytics", labelKey: "nav.analytics" },
  {
    path: "/analytics/:tab",
    labelKeyFromParam: (s) => `analytics.menu.${s}`,
    parent: "/analytics",
  },

  { path: "@order", labelKey: "nav.order" },
  { path: "/pos", labelKey: "nav.pos", parent: "@order" },
  { path: "/orders", labelKey: "nav.orders", parent: "@order" },
  { path: "/orders/:id", dynamic: true, parent: "/orders" },

  { path: "/products", labelKey: "nav.products", pharmacyLabelKey: "nav.medicines" },
  { path: "/products/:id", dynamic: true, parent: "/products" },

  { path: "@inventory", labelKey: "nav.inventory" },
  { path: "/purchasing", labelKey: "nav.purchasing", parent: "@inventory" },
  { path: "/purchasing/new", labelKey: "purchasing.newPo", parent: "/purchasing" },
  // The status tabs (/purchasing/all, /draft, …) fall through to this entry and
  // register no label, so a tab switch keeps the trail at "… > Restock" — a
  // filter is not a location. Only the PO detail page fills it in.
  { path: "/purchasing/:id", dynamic: true, parent: "/purchasing" },

  { path: "/inventory/suppliers", labelKey: "inventory.tabs.suppliers", parent: "@inventory" },
  { path: "/inventory/suppliers/:id", dynamic: true, parent: "/inventory/suppliers" },
  { path: "/inventory/price-agreements", labelKey: "nav.priceAgreements", parent: "@inventory" },
  {
    path: "/inventory/price-agreements/new",
    labelKey: "inventory.priceAgreements.newTitle",
    parent: "/inventory/price-agreements",
  },
  { path: "/inventory/batches", labelKey: "inventory.tabs.batches", parent: "@inventory" },
  { path: "/inventory/movements", labelKey: "inventory.tabs.movements", parent: "@inventory" },
  { path: "/inventory/stocktake", labelKey: "inventory.tabs.stocktake", parent: "@inventory" },
  { path: "/inventory/stocktake/:id", dynamic: true, parent: "/inventory/stocktake" },
  { path: "/inventory/transfers", labelKey: "inventory.tabs.transfers", parent: "@inventory" },

  { path: "/prescriptions", labelKey: "nav.prescriptions" },
  { path: "/prescriptions/new", labelKey: "prescriptions.addTitle", parent: "/prescriptions" },

  { path: "/customers", labelKey: "nav.customers" },
  { path: "/my-performance", labelKey: "nav.myPerformance" },

  { path: "/warehouses", labelKey: "nav.warehouses" },
  { path: "/warehouses/:id", dynamic: true, parent: "/warehouses" },

  { path: "/profile", labelKey: "nav.profile" },

  { path: "/users", labelKey: "nav.users" },
  { path: "/settings", labelKey: "nav.settings" },
  {
    path: "/settings/:tab",
    labelKeyFromParam: (s) => `settings.tabs.${s}`,
    parent: "/settings",
  },

  { path: "/components", labelKey: "dev.components.title" },
  {
    path: "/components/:group",
    labelKeyFromParam: (s) => `dev.components.groups.${s}`,
    parent: "/components",
  },
];

const BY_PATH = new Map(ROUTES.map((r) => [r.path, r]));

function segments(p: string): string[] {
  return p.split("/").filter(Boolean);
}

/**
 * Most-specific match wins: same segment count, and among those the pattern
 * with the most literal (non-`:param`) segments — so /purchasing/new beats
 * /purchasing/:id.
 */
function matchRoute(pathname: string): { def: RouteDef; params: string[] } | null {
  const parts = segments(pathname);
  let best: { def: RouteDef; params: string[]; score: number } | null = null;

  for (const def of ROUTES) {
    if (def.path.startsWith("@")) continue;
    const pat = segments(def.path);
    if (pat.length !== parts.length) continue;

    const params: string[] = [];
    let score = 0;
    let ok = true;
    for (let i = 0; i < pat.length; i++) {
      if (pat[i].startsWith(":")) {
        params.push(parts[i]);
      } else if (pat[i] === parts[i]) {
        score++;
      } else {
        ok = false;
        break;
      }
    }
    if (ok && (!best || score > best.score)) best = { def, params, score };
  }

  return best ? { def: best.def, params: best.params } : null;
}

// Page-supplied label for the leaf crumb of a `:id` route. Cross-cutting by
// nature (the page knows the name, the TopBar renders it), hence a store.
type CrumbLabelState = { label: string; setLabel: (v: string) => void };

const useCrumbLabelStore = create<CrumbLabelState>((set) => ({
  label: "",
  setLabel: (label) => set({ label }),
}));

/**
 * Detail pages call this with their entity name so the trail can end in
 * "Products > Paracetamol" instead of a UUID. Pass undefined while loading —
 * the crumb is dropped rather than showing a placeholder. Cleared on unmount so
 * a stale name never leaks onto the next page.
 */
export function useCrumbLabel(label: string | undefined) {
  const setLabel = useCrumbLabelStore((s) => s.setLabel);
  useEffect(() => {
    setLabel(label ?? "");
    return () => setLabel("");
  }, [label, setLabel]);
}

export function useBreadcrumbs(): Crumb[] {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { isPharmacy } = useBusinessMode();
  const dynamicLabel = useCrumbLabelStore((s) => s.label);

  const match = matchRoute(pathname);
  if (!match) return [];

  const chain: RouteDef[] = [];
  let cursor: RouteDef | undefined = match.def;
  // Bounded by the registry depth; the guard is only there so a mistyped
  // `parent` can't spin forever.
  while (cursor && chain.length < 8) {
    chain.unshift(cursor);
    cursor = cursor.parent ? BY_PATH.get(cursor.parent) : undefined;
  }

  const crumbs: Crumb[] = [];
  for (const def of chain) {
    const isLeaf = def === match.def;

    let label = "";
    if (def.dynamic) {
      label = isLeaf ? dynamicLabel : "";
    } else if (def.labelKeyFromParam) {
      const key = def.labelKeyFromParam(match.params[0] ?? "");
      // defaultValue "" so an unregistered segment (e.g. an /analytics legacy
      // redirect passing through) drops the crumb instead of printing the key.
      label = t(key, { defaultValue: "" });
    } else {
      const key = isPharmacy && def.pharmacyLabelKey ? def.pharmacyLabelKey : def.labelKey;
      label = key ? t(key) : "";
    }
    if (!label) continue;

    // Only a static, non-leaf crumb is a destination: the leaf is where you
    // already are, and a `:param` ancestor has no canonical URL.
    const to = !isLeaf && !def.path.startsWith("@") && !def.path.includes(":") ? def.path : undefined;
    crumbs.push({ label, to });
  }

  return crumbs;
}
