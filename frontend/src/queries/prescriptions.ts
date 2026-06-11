import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { prescriptionClient } from "../lib/clients";
import type {
  CreatePrescriptionRequest,
  UpdatePrescriptionRequest,
} from "../gen/prescription_iface/v1/prescription_pb";

export type PrescriptionsQueryOpts = {
  status?: string;
  customerId?: string;
  limit?: number;
  offset?: number;
  enabled?: boolean;
};

export const prescriptionKeys = {
  all: ["prescriptions"] as const,
  // `enabled` is a fetch toggle, not a filter — it stays out of the cache key.
  list: (opts: { status: string; customerId: string; limit: number; offset: number }) =>
    [...prescriptionKeys.all, "list", opts] as const,
};

// Server-paginated. Returns { rows, total }. `total` reflects the customer-scoped
// base rows; the computed-status filter is applied server-side over the page
// (documented v1 limitation, mirrors the backend handler).
export function usePrescriptionsQuery(opts: PrescriptionsQueryOpts = {}) {
  const { status = "", customerId = "", limit = 25, offset = 0, enabled = true } = opts;
  const q = useQuery({
    queryKey: prescriptionKeys.list({ status, customerId, limit, offset }),
    queryFn: async () => {
      const res = await prescriptionClient.listPrescriptions({
        status,
        customerId,
        limit,
        offset,
      });
      return { rows: res.prescriptions, total: res.total };
    },
    enabled,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreatePrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePrescriptionRequest>) =>
      prescriptionClient.createPrescription(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: prescriptionKeys.all }),
  });
}

export function useUpdatePrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdatePrescriptionRequest>) =>
      prescriptionClient.updatePrescription(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: prescriptionKeys.all }),
  });
}

export function useVoidPrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => prescriptionClient.voidPrescription({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: prescriptionKeys.all }),
  });
}

// Count of currently-active prescriptions — for the pharmacist dashboard tile.
// The handler applies the computed-status filter over the page, so we count the
// returned rows (cap 1000 is plenty for a single pharmacy).
export function useActiveRxCountQuery(enabled = true) {
  const q = useQuery({
    queryKey: [...prescriptionKeys.all, "activeCount"],
    queryFn: async () => {
      const res = await prescriptionClient.listPrescriptions({
        status: "ACTIVE",
        limit: 1000,
      });
      return res.prescriptions.length;
    },
    enabled,
    staleTime: 30_000,
  });
  return { ...q, count: q.data ?? 0 };
}
