import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { backupClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export const backupKeys = {
  all: ["backups"] as const,
  list: (page: number, pageSize: number) =>
    [...backupKeys.all, "list", page, pageSize] as const,
};

// Server-paginated; returns { rows, total }. Refetches every minute so a fresh
// backup created via the button shows up promptly even without interaction.
export function useBackupsQuery(opts: { page?: number; pageSize?: number } = {}) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: backupKeys.list(page, pageSize),
    queryFn: async () => {
      const res = await backupClient.listBackups({ limit: pageSize, offset: page * pageSize });
      return { rows: res.backups, total: res.total };
    },
    staleTime: 60_000,
    refetchInterval: 60_000,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreateBackupMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => backupClient.createBackup({}),
    onSuccess: () => qc.invalidateQueries({ queryKey: backupKeys.all }),
  });
}

export function useDeleteBackupMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => backupClient.deleteBackup({ name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: backupKeys.all }),
  });
}
