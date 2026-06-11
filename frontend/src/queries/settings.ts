import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { settingsClient } from "../lib/clients";
import { BussinessType } from "../gen/settings_iface/v1/settings_pb";

export const settingsKeys = {
  all: ["settings"] as const,
  businessMode: ["settings", "businessMode"] as const,
  licenseInfo: ["settings", "licenseInfo"] as const,
};

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
  });
}
