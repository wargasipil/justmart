import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "light" | "dark";
export type Locale = "id" | "en";

// Default-visible product-list columns. The restock columns (lastRestock*,
// lastSupplier) are intentionally OFF by default to keep the table uncluttered;
// the user opts in via the Columns selector (persisted in productListColumns).
// `name` is always shown (identity) and is not part of this toggle set.
export const DEFAULT_PRODUCT_COLUMNS = [
  "sku",
  "unit",
  "unitPrice",
  "ready",
  "onOrder",
  "lastStocktake",
];

type PreferencesState = {
  theme: Theme;
  locale: Locale;
  sidebarCollapsed: boolean;
  // Per-base-unit display preference for the Products list stock cells.
  // Key = base unit name (e.g. "tablet", "ml"); value = chosen derivative name
  // ("" = base / raw count). Rows whose product's base unit isn't in the map
  // render in their base unit by default.
  productStockUnitsByBase: Record<string, string>;
  // Visible product-list columns (ids; see DEFAULT_PRODUCT_COLUMNS). Persisted so
  // the user's column choice sticks across sessions.
  productListColumns: string[];
  setTheme: (t: Theme) => void;
  setLocale: (l: Locale) => void;
  toggleSidebar: () => void;
  setSidebarCollapsed: (c: boolean) => void;
  setProductStockUnitByBase: (baseName: string, deriv: string) => void;
  setProductListColumns: (cols: string[]) => void;
};

// Flip Chakra's default semantic tokens between light/dark by toggling the
// `data-theme` attribute on <html>. This is the platform mechanism documented
// for Chakra v3 — we don't wrap it in a custom system.
function applyTheme(theme: Theme) {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-theme", theme);
  document.documentElement.classList.toggle("dark", theme === "dark");
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set, get) => ({
      theme: "light",
      locale: "id",
      sidebarCollapsed: false,
      productStockUnitsByBase: {},
      productListColumns: DEFAULT_PRODUCT_COLUMNS,
      setTheme: (theme) => {
        applyTheme(theme);
        set({ theme });
      },
      setLocale: (locale) => set({ locale }),
      toggleSidebar: () => set({ sidebarCollapsed: !get().sidebarCollapsed }),
      setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
      setProductStockUnitByBase: (baseName, deriv) =>
        set({
          productStockUnitsByBase: {
            ...get().productStockUnitsByBase,
            [baseName]: deriv,
          },
        }),
      setProductListColumns: (productListColumns) => set({ productListColumns }),
    }),
    {
      name: "justmart_preferences",
      onRehydrateStorage: () => (state) => {
        if (state) applyTheme(state.theme);
      },
    },
  ),
);

// Apply current theme on import (no FOUC for the second pageview).
applyTheme(usePreferencesStore.getState().theme);
