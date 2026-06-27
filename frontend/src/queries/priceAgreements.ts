import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { priceAgreementClient } from "../lib/clients";
import type {
  ArchivePriceAgreementRequest,
  CreatePriceAgreementsRequest,
  UnarchivePriceAgreementRequest,
  UpdatePriceAgreementRequest,
} from "../gen/inventory_iface/v1/price_agreement_pb";
import { PriceAgreementValidity } from "../gen/inventory_iface/v1/price_agreement_pb";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type PriceAgreementsQueryOpts = {
  supplierId?: string;
  productId?: string;
  query?: string;
  includeInactive?: boolean;
  validity?: PriceAgreementValidity;
  page?: number;
  pageSize?: number;
  enabled?: boolean;
};

export const priceAgreementKeys = {
  all: ["priceAgreements"] as const,
  // `enabled` is a fetch gate, not part of the data identity — keep it out of the key.
  list: (opts: Required<Omit<PriceAgreementsQueryOpts, "enabled">>) =>
    [...priceAgreementKeys.all, "list", opts] as const,
};

// Server-paginated. Returns { rows, total }. Pass supplierId to scope to one
// supplier (the supplier-detail section); omit for the all-suppliers page.
export function usePriceAgreementsQuery(opts: PriceAgreementsQueryOpts = {}) {
  const {
    supplierId = "",
    productId = "",
    query = "",
    includeInactive = false,
    validity = PriceAgreementValidity.UNSPECIFIED,
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = opts;
  const q = useQuery({
    enabled,
    queryKey: priceAgreementKeys.list({ supplierId, productId, query, includeInactive, validity, page, pageSize }),
    queryFn: async () => {
      const res = await priceAgreementClient.listPriceAgreements({
        supplierId,
        productId,
        query,
        includeInactive,
        validity,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.agreements, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Batch create: many product/unit lines for one supplier in a single submit
// (backs the dedicated "New price agreement" page). The page handles errors
// (toast + inline duplicate check), so it does NOT silence them here.
export function useCreatePriceAgreementsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePriceAgreementsRequest>) =>
      priceAgreementClient.createPriceAgreements(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: priceAgreementKeys.all }),
  });
}

export function useUpdatePriceAgreementMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdatePriceAgreementRequest>) =>
      priceAgreementClient.updatePriceAgreement(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: priceAgreementKeys.all }),
    meta: { silentError: true },
  });
}

export function useArchivePriceAgreementMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchivePriceAgreementRequest>) =>
      priceAgreementClient.archivePriceAgreement(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: priceAgreementKeys.all }),
  });
}

export function useUnarchivePriceAgreementMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UnarchivePriceAgreementRequest>) =>
      priceAgreementClient.unarchivePriceAgreement(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: priceAgreementKeys.all }),
  });
}
