import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "light" | "dark";
export type Locale = "id" | "en";

type PreferencesState = {
  theme: Theme;
  locale: Locale;
  sidebarCollapsed: boolean;
  // Per-base-unit display preference for the Products list stock cells.
  // Key = base unit name (e.g. "tablet", "ml"); value = chosen derivative name
  // ("" = base / raw count). Rows whose product's base unit isn't in the map
  // render in their base unit by default.
  productStockUnitsByBase: Record<string, string>;
  setTheme: (t: Theme) => void;
  setLocale: (l: Locale) => void;
  toggleSidebar: () => void;
  setSidebarCollapsed: (c: boolean) => void;
  setProductStockUnitByBase: (baseName: string, deriv: string) => void;
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
