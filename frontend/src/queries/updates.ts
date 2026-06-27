import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { settingsClient } from "../lib/clients";

export const updateKeys = {
  info: ["updates", "info"] as const,
};

// Checks the running version against the latest GitHub release (OWNER-only RPC).
// Long staleTime — version checks are not urgent and hit the GitHub API. silent
// so an offline/rate-limited check never toasts. Used by the Settings ▸ Updates
// panel and the TopBar bell.
export function useUpdateInfoQuery(enabled = true) {
  return useQuery({
    queryKey: updateKeys.info,
    queryFn: () => settingsClient.checkUpdate({}),
    enabled,
    staleTime: 60 * 60_000,
    meta: { silentError: true },
  });
}

// Downloads + verifies + stages the latest release (applied on next restart).
export function useApplyUpdateMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (restart: boolean) => settingsClient.applyUpdate({ restart }),
    onSuccess: () => qc.invalidateQueries({ queryKey: updateKeys.info }),
  });
}

// Stages the previous-version backup to roll back the last update (applied on
// next restart). Windows portable only; the button only shows when canRevert.
export function useRevertUpdateMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => settingsClient.revertUpdate({}),
    onSuccess: () => qc.invalidateQueries({ queryKey: updateKeys.info }),
  });
}
