import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { settingsClient } from "../lib/clients";
import { BussinessType } from "../gen/settings_iface/v1/settings_pb";

export const settingsKeys = {
  all: ["settings"] as const,
  businessMode: ["settings", "businessMode"] as const,
  branding: ["settings", "branding"] as const,
};

// Branding (business mode + configured app title) via the PUBLIC GetBranding RPC
// — readable WITHOUT auth, so the login screen + browser tab title can brand the
// shop before anyone logs in (the authenticated business-mode query is dormant
// pre-login). Last-known branding is cached in localStorage and used as
// placeholder so a repeat visitor sees the right brand instantly, no "Justmart"
// flash, while the query still refetches for freshness.
const BRANDING_CACHE_KEY = "justmart_branding";
type CachedBranding = { businessType: number; appTitle: string };

function readBrandingCache(): CachedBranding | undefined {
  try {
    const raw = localStorage.getItem(BRANDING_CACHE_KEY);
    if (!raw) return undefined;
    const p = JSON.parse(raw) as Partial<CachedBranding>;
    if (typeof p?.businessType === "number" && typeof p?.appTitle === "string") {
      return { businessType: p.businessType, appTitle: p.appTitle };
    }
  } catch {
    // malformed / unavailable cache — ignore, fall back to the default brand
  }
  return undefined;
}

function writeBrandingCache(b: CachedBranding) {
  try {
    localStorage.setItem(BRANDING_CACHE_KEY, JSON.stringify(b));
  } catch {
    // private mode / quota — non-fatal
  }
}

export function useBrandingQuery() {
  return useQuery({
    queryKey: settingsKeys.branding,
    queryFn: async (): Promise<CachedBranding> => {
      const res = await settingsClient.getBranding({});
      return { businessType: res.businessType, appTitle: res.appTitle };
    },
    staleTime: 5 * 60_000,
    placeholderData: readBrandingCache,
    meta: { silentError: true }, // failure just falls back to the retail default
  });
}

// Public branding accessor for pre-login surfaces (login page + tab title). Same
// shape as useBusinessMode; defaults to RETAIL when unset. Persists each fresh
// result to the localStorage cache for the next cold load.
export function useBranding() {
  const q = useBrandingQuery();
  const mode = q.data?.businessType ?? BussinessType.UNSPECIFIED;
  useEffect(() => {
    if (q.data) writeBrandingCache(q.data);
  }, [q.data]);
  return {
    mode,
    isPharmacy: mode === BussinessType.PHARMACY_SHOP,
    isRetail: mode !== BussinessType.PHARMACY_SHOP,
    appTitle: q.data?.appTitle ?? "",
    isLoading: q.isLoading,
  };
}

// The single source of the app's display name, used by every brand surface (tab
// title, sidebar brand, login heading): the owner-configured title from
// Settings ▸ General wins; otherwise fall back to the built-in brand for the
// active mode ("Apotek"/"Pharmacy" vs "Justmart"). Reads the PUBLIC branding
// query so it's correct pre-login too.
export function useAppTitle() {
  const { isPharmacy, appTitle } = useBranding();
  const { t } = useTranslation();
  return appTitle || (isPharmacy ? t("app.pharmacyName") : t("app.name"));
}

// The shop's business mode (configured in Settings ▸ General). Readable by every
// authenticated role; drives branding, navigation, and POS Rx behavior. Long
// staleTime — the mode changes only when the owner edits it (the mutation
// invalidates this key). `enabled` lets callers skip the (authenticated) RPC
// before login. Errors are silenced — a failure just falls back to retail.
export function useBusinessModeQuery(enabled = true) {
  return useQuery({
    queryKey: settingsKeys.businessMode,
    queryFn: () => settingsClient.getBussinessSettings({}), // { type, name }
    staleTime: 5 * 60_000,
    enabled,
    meta: { silentError: true },
  });
}

// Convenience accessor. Defaults to RETAIL when unset/unspecified so a fresh
// install (nothing configured) behaves as the completed retail product, not a
// half-rendered pharmacy. Pharmacy features must opt in via `isPharmacy`.
// `appTitle` is the owner-configured shop title ("" = use the built-in brand).
export function useBusinessMode(enabled = true) {
  const q = useBusinessModeQuery(enabled);
  const mode = q.data?.type ?? BussinessType.UNSPECIFIED;
  return {
    mode,
    isPharmacy: mode === BussinessType.PHARMACY_SHOP,
    isRetail: mode !== BussinessType.PHARMACY_SHOP, // UNSPECIFIED falls back to retail
    appTitle: q.data?.appTitle ?? "",
    isLoading: q.isLoading,
  };
}

export function useSettingsQuery() {
  const q = useQuery({
    queryKey: settingsKeys.all,
    queryFn: async () => {
      const res = await settingsClient.getSettings({});
      return res.settings;
    },
    staleTime: 60_000,
  });
  return q;
}

// Saves the General settings panel. The title + mode re-brand the whole app
// (tab title, sidebar, glossary, nav, POS Rx gate), so invalidate both branding
// keys — the app re-themes live, no reload.
export function useUpdateSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: {
      lowStockThreshold: number;
      appTitle: string;
      businessType: BussinessType;
    }) => settingsClient.updateSettings(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
      qc.invalidateQueries({ queryKey: settingsKeys.businessMode });
      qc.invalidateQueries({ queryKey: settingsKeys.branding });
      // Threshold change → bell badge / dropdown re-evaluate.
      qc.invalidateQueries({ queryKey: ["lowStock"] });
    },
    // Errors handled by the form via useServerFormErrors.
    meta: { silentError: true },
  });
}
