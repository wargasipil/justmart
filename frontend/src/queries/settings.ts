import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { settingsClient } from "../lib/clients";
import { BussinessType } from "../gen/settings_iface/v1/settings_pb";

export const settingsKeys = {
  all: ["settings"] as const,
  businessMode: ["settings", "businessMode"] as const,
  branding: ["settings", "branding"] as const,
  licenseInfo: ["settings", "licenseInfo"] as const,
};

// Branding (business type + licensed shop name) via the PUBLIC GetBranding RPC —
// readable WITHOUT auth, so the login screen + browser tab title can brand the
// pharmacy shop before anyone logs in (the authenticated business-mode query is
// dormant pre-login). Last-known branding is cached in localStorage and used as
// placeholder so a repeat visitor sees the right brand instantly, no "Justmart"
// flash, while the query still refetches for freshness.
const BRANDING_CACHE_KEY = "justmart_branding";
type CachedBranding = { businessType: number; shopName: string };

function readBrandingCache(): CachedBranding | undefined {
  try {
    const raw = localStorage.getItem(BRANDING_CACHE_KEY);
    if (!raw) return undefined;
    const p = JSON.parse(raw) as Partial<CachedBranding>;
    if (typeof p?.businessType === "number" && typeof p?.shopName === "string") {
      return { businessType: p.businessType, shopName: p.shopName };
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
      return { businessType: res.businessType, shopName: res.shopName };
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
    shopName: q.data?.shopName ?? "",
    isLoading: q.isLoading,
  };
}

// The shop's business mode (license-driven). Readable by every authenticated
// role; drives branding, navigation, and POS Rx behavior. Long staleTime — the
// mode only changes on a server restart (re-applied from the license on boot).
// `enabled` lets callers skip the (authenticated) RPC before login. Errors are
// silenced — a failure just falls back to retail, no global toast.
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
// install (no license) behaves as the completed retail product, not a
// half-rendered pharmacy. Pharmacy features must opt in via `isPharmacy`.
// `shopName` is the licensed holder name (used for the pharmacy-mode header).
export function useBusinessMode(enabled = true) {
  const q = useBusinessModeQuery(enabled);
  const mode = q.data?.type ?? BussinessType.UNSPECIFIED;
  return {
    mode,
    isPharmacy: mode === BussinessType.PHARMACY_SHOP,
    isRetail: mode !== BussinessType.PHARMACY_SHOP, // UNSPECIFIED falls back to retail
    shopName: q.data?.name ?? "",
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

// License info for the Settings › License page (OWNER). Returns the applied
// license holder + active business type.
export function useLicenseInfoQuery() {
  return useQuery({
    queryKey: settingsKeys.licenseInfo,
    queryFn: () => settingsClient.getLicenseInfo({}),
    staleTime: 60_000,
  });
}

// Apply a pasted license key. On success the business mode may change, so we
// invalidate the mode query (re-themes the whole app via GlossaryBridge / nav)
// plus the license-info panel.
export function useApplyLicenseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => settingsClient.applyLicense({ token }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.businessMode });
      qc.invalidateQueries({ queryKey: settingsKeys.branding });
      qc.invalidateQueries({ queryKey: settingsKeys.licenseInfo });
    },
    meta: { silentError: true }, // the page surfaces the verify error inline
  });
}

export function useUpdateSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { lowStockThreshold: number }) =>
      settingsClient.updateSettings(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
      // Threshold change → bell badge / dropdown re-evaluate.
      qc.invalidateQueries({ queryKey: ["lowStock"] });
    },
    // Errors handled by the form via useServerFormErrors.
    meta: { silentError: true },
  });
}
