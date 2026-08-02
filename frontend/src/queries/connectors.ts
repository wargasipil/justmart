import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { connectorClient, settingsClient } from "../lib/clients";

export const connectorKeys = {
  all: ["connectors"] as const,
  list: () => [...connectorKeys.all, "list"] as const,
  target: () => [...connectorKeys.all, "print-target"] as const,
  kitchenTarget: () => [...connectorKeys.all, "kitchen-target"] as const,
  printingInfo: () => [...connectorKeys.all, "printing-info"] as const,
};

// Active print mode (config connector.mode) + the server host's local printers
// (usb mode only; empty off-Windows). Drives the mode-aware Printing panel.
export function usePrintingInfoQuery(enabled = true) {
  return useQuery({
    queryKey: connectorKeys.printingInfo(),
    queryFn: async () => {
      const res = await settingsClient.getPrintingInfo({});
      return { mode: res.mode, localPrinters: res.localPrinters };
    },
    enabled,
    staleTime: 30_000,
  });
}

// Live list of connected print connectors + their printers. Polls every 5s so
// the Settings ▸ Printing picker reflects a connector coming/going. Manager-tier
// RPC; meta.silentError so a transient failure doesn't toast.
export function useConnectorsQuery(enabled = true) {
  return useQuery({
    queryKey: connectorKeys.list(),
    queryFn: async () => {
      const res = await connectorClient.listConnectors({});
      return res.connectors;
    },
    enabled,
    refetchInterval: 5_000,
    staleTime: 2_000,
    meta: { silentError: true },
  });
}

// The saved default print target (connector device + printer).
export function usePrintTargetQuery(enabled = true) {
  return useQuery({
    queryKey: connectorKeys.target(),
    queryFn: async () => {
      const res = await settingsClient.getPrintTarget({});
      return { connectorDeviceId: res.connectorDeviceId, printerName: res.printerName };
    },
    enabled,
    staleTime: 30_000,
  });
}

export function useSetPrintTargetMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { connectorDeviceId: string; printerName: string }) =>
      settingsClient.setPrintTarget(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: connectorKeys.target() }),
  });
}

// The saved KITCHEN ticket target (restaurant mode). Both fields empty = not
// configured, in which case the server fires tickets to the RECEIPT printer —
// so a one-printer warung works without touching this at all.
export function useKitchenPrintTargetQuery(enabled = true) {
  return useQuery({
    queryKey: connectorKeys.kitchenTarget(),
    queryFn: async () => {
      const res = await settingsClient.getKitchenPrintTarget({});
      return { connectorDeviceId: res.connectorDeviceId, printerName: res.printerName };
    },
    enabled,
    staleTime: 30_000,
  });
}

export function useSetKitchenPrintTargetMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { connectorDeviceId: string; printerName: string }) =>
      settingsClient.setKitchenPrintTarget(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: connectorKeys.kitchenTarget() }),
  });
}

// Printed-receipt header (shop name/address) + footer (closing lines) + paper
// width (chars per line: 32 = 58mm, 48 = 80mm).
export function useReceiptSettingsQuery(enabled = true) {
  return useQuery({
    queryKey: [...connectorKeys.all, "receipt"],
    queryFn: async () => {
      const res = await settingsClient.getReceiptSettings({});
      return { header: res.header, footer: res.footer, width: res.width };
    },
    enabled,
    staleTime: 30_000,
  });
}

export function useSetReceiptSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { header: string; footer: string; width: number }) =>
      settingsClient.setReceiptSettings(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: [...connectorKeys.all, "receipt"] }),
  });
}
