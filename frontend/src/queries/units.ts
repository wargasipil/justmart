import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { unitClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";
import type {
  ArchiveUnitBaseRequest,
  ArchiveUnitDerivativeRequest,
  CreateUnitBaseRequest,
  CreateUnitDerivativeRequest,
  UpdateUnitBaseRequest,
  UpdateUnitDerivativeRequest,
} from "../gen/unit_iface/v1/unit_pb";

export const unitKeys = {
  all: ["units"] as const,
  bases: (includeInactive: boolean, page: number, pageSize: number) =>
    [...unitKeys.all, "bases", includeInactive, page, pageSize] as const,
};

// Global unit catalog. Returns base units (active by default) with their
// active derivatives hydrated. Used by Settings + the Products list popover.
// Server-paginated; returns { rows, total }. Callers that need the whole
// catalog in memory (the Products list "Units" popover) pass ALL_LIMIT.
export function useUnitBasesQuery(
  opts: { includeInactive?: boolean; page?: number; pageSize?: number } = {},
) {
  const { includeInactive = false, page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: unitKeys.bases(includeInactive, page, pageSize),
    queryFn: async () => {
      const res = await unitClient.listUnitBases({
        includeInactive,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.bases, total: res.total };
    },
    staleTime: 60_000,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreateUnitBaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateUnitBaseRequest>) =>
      unitClient.createUnitBase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useUpdateUnitBaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateUnitBaseRequest>) =>
      unitClient.updateUnitBase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useArchiveUnitBaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveUnitBaseRequest>) =>
      unitClient.archiveUnitBase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useCreateUnitDerivativeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateUnitDerivativeRequest>) =>
      unitClient.createUnitDerivative(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useUpdateUnitDerivativeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateUnitDerivativeRequest>) =>
      unitClient.updateUnitDerivative(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}

export function useArchiveUnitDerivativeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveUnitDerivativeRequest>) =>
      unitClient.archiveUnitDerivative(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: unitKeys.all }),
  });
}
