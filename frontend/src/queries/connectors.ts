import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { connectorClient, settingsClient } from "../lib/clients";

export const connectorKeys = {
  all: ["connectors"] as const,
  list: () => [...connectorKeys.all, "list"] as const,
  target: () => [...connectorKeys.all, "print-target"] as const,
};

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
