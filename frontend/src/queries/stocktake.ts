import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { stocktakeClient } from "../lib/clients";
import type {
  AddAllInStockBatchesRequest,
  AddBatchesToSessionRequest,
  CompleteStocktakeRequest,
  RecordCountRequest,
  RemoveLineRequest,
  SetLineDispositionRequest,
  StartStocktakeRequest,
  VoidStocktakeRequest,
} from "../gen/stocktake_iface/v1/stocktake_pb";

import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

// The date range the list and the stat row share. dateField "" = Any date (the
// picker's own off state), in which case no bounds are sent at all.
export type StocktakeDateFilters = {
  fromUnix?: bigint;
  toUnix?: bigint;
  dateField?: string;
};

export type StocktakesQueryOpts = StocktakeDateFilters & {
  status?: string;
  page?: number;
  pageSize?: number;
};

export const stocktakeKeys = {
  all: ["stocktakes"] as const,
  list: (opts: Required<StocktakesQueryOpts>) => [...stocktakeKeys.all, "list", opts] as const,
  detail: (id: string) => [...stocktakeKeys.all, "detail", id] as const,
  summary: (opts: Required<StocktakeDateFilters>) =>
    [...stocktakeKeys.all, "summary", opts] as const,
};

// Server-side aggregate over every session matching the list's date range — the
// stat row above the table. It takes no `status` (see the RPC comment): the
// figures are the status breakdown itself. Keyed under stocktakeKeys.all so
// every mutation's existing invalidate refreshes it.
export function useStocktakeSummaryQuery(opts: StocktakeDateFilters = {}) {
  const { fromUnix = 0n, toUnix = 0n, dateField = "" } = opts;
  return useQuery({
    queryKey: stocktakeKeys.summary({ fromUnix, toUnix, dateField }),
    queryFn: () => stocktakeClient.getStocktakeSummary({ fromUnix, toUnix, dateField }),
  });
}

// Server-paginated. Returns { rows, total }.
export function useStocktakesQuery(opts: StocktakesQueryOpts = {}) {
  const {
    status = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
    fromUnix = 0n,
    toUnix = 0n,
    dateField = "",
  } = opts;
  const q = useQuery({
    queryKey: stocktakeKeys.list({ status, page, pageSize, fromUnix, toUnix, dateField }),
    queryFn: async () => {
      const res = await stocktakeClient.listStocktakes({
        status,
        limit: pageSize,
        offset: page * pageSize,
        fromUnix,
        toUnix,
        dateField,
      });
      return { rows: res.sessions, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useStocktakeQuery(id: string | undefined) {
  return useQuery({
    queryKey: stocktakeKeys.detail(id ?? ""),
    enabled: !!id,
    queryFn: async () => {
      const res = await stocktakeClient.getStocktake({ id: id ?? "" });
      return res;
    },
  });
}

export function useStartStocktakeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<StartStocktakeRequest>) =>
      stocktakeClient.startStocktake(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: stocktakeKeys.all }),
  });
}

export function useAddBatchesMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddBatchesToSessionRequest>) =>
      stocktakeClient.addBatchesToSession(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}

export function useAddAllInStockBatchesMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddAllInStockBatchesRequest>) =>
      stocktakeClient.addAllInStockBatches(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}

export function useRecordCountMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RecordCountRequest>) =>
      stocktakeClient.recordCount(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
    },
  });
}

export function useSetLineDispositionMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetLineDispositionRequest>) =>
      stocktakeClient.setLineDisposition(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
    },
  });
}

export function useRemoveLineMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RemoveLineRequest>) =>
      stocktakeClient.removeLine(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
    },
  });
}

export function useCompleteStocktakeMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CompleteStocktakeRequest>) =>
      stocktakeClient.completeStocktake(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}

export function useVoidStocktakeMutation(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<VoidStocktakeRequest>) =>
      stocktakeClient.voidStocktake(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: stocktakeKeys.detail(sessionId) });
      qc.invalidateQueries({ queryKey: stocktakeKeys.all });
    },
  });
}
