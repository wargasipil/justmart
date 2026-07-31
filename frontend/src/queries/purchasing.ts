import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import {
  purchaseOrderClient,
  purchasePaymentClient,
  purchaseReceiptClient,
  purchaseReturnClient,
} from "../lib/clients";
import type {
  CreatePurchaseOrderRequest,
  ListPurchaseOrdersRequest,
  UpdatePurchaseOrderRequest,
} from "../gen/purchasing_iface/v1/order_pb";
import type {
  CreateReceiptRequest,
  ListReceiptsRequest,
} from "../gen/purchasing_iface/v1/receipt_pb";
import type {
  GetSupplierBalancesRequest,
  PayPurchaseRequest,
} from "../gen/purchasing_iface/v1/payment_pb";
import type { CreatePurchaseReturnRequest } from "../gen/purchasing_iface/v1/return_pb";
import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export const purchasingKeys = {
  all: ["purchasing"] as const,
  orders: (filters: object) => [...purchasingKeys.all, "orders", filters] as const,
  order: (id: string) => [...purchasingKeys.all, "order", id] as const,
  receipts: (poId: string, page: number, pageSize: number) =>
    [...purchasingKeys.all, "receipts", poId, page, pageSize] as const,
  returns: (poId: string, page: number, pageSize: number) =>
    [...purchasingKeys.all, "returns", poId, page, pageSize] as const,
  balances: (filters: object) => [...purchasingKeys.all, "balances", filters] as const,
};

// ---------- Orders ----------
// Server-paginated. Returns { rows, total }. Caller sets limit/offset on req.
export function usePurchaseOrdersQuery(req: PartialMessage<ListPurchaseOrdersRequest> = {}) {
  const q = useQuery({
    queryKey: purchasingKeys.orders(req),
    queryFn: async () => {
      const res = await purchaseOrderClient.listPurchaseOrders(req);
      return { rows: res.orders, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Imperative one-shot fetch of ALL POs matching the filters (cap ALL_LIMIT), for
// CSV export. Not a hook — call from an export handler.
export async function fetchPurchaseOrdersForExport(
  req: PartialMessage<ListPurchaseOrdersRequest> = {},
) {
  const res = await purchaseOrderClient.listPurchaseOrders({ ...req, limit: ALL_LIMIT, offset: 0 });
  return res.orders;
}

export function usePurchaseOrderQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: purchasingKeys.order(id),
    queryFn: async () => {
      const res = await purchaseOrderClient.getPurchaseOrder({ id });
      return res.order;
    },
    enabled: enabled && !!id,
  });
}

export function useCreatePurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePurchaseOrderRequest>) =>
      purchaseOrderClient.createPurchaseOrder(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
    },
  });
}

export function useUpdatePurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdatePurchaseOrderRequest>) =>
      purchaseOrderClient.updatePurchaseOrder(req),
    onSuccess: (res) => {
      if (res.order?.id) {
        void qc.invalidateQueries({ queryKey: purchasingKeys.order(res.order.id) });
      }
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
    },
  });
}

export function useSendPurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => purchaseOrderClient.sendPurchaseOrder({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: purchasingKeys.all }),
  });
}

export function useVoidPurchaseOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => purchaseOrderClient.voidPurchaseOrder({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: purchasingKeys.all }),
  });
}

// ---------- Receipts ----------
// Server-paginated; returns { rows, total }.
export function useReceiptsQuery(
  req: PartialMessage<ListReceiptsRequest>,
  opts: { page?: number; pageSize?: number } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: purchasingKeys.receipts(req.purchaseOrderId ?? "", page, pageSize),
    queryFn: async () => {
      const res = await purchaseReceiptClient.listReceipts({
        ...req,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.receipts, total: res.total };
    },
    enabled: !!req.purchaseOrderId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreateReceiptMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateReceiptRequest>) =>
      purchaseReceiptClient.createReceipt(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
      void qc.invalidateQueries({ queryKey: ["batches"] });
      void qc.invalidateQueries({ queryKey: ["stock"] });
    },
  });
}

// ---------- Returns ----------
// Server-paginated; returns { rows, total }.
export function usePurchaseReturnsQuery(
  purchaseOrderId: string,
  opts: { page?: number; pageSize?: number } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: purchasingKeys.returns(purchaseOrderId, page, pageSize),
    queryFn: async () => {
      const res = await purchaseReturnClient.listPurchaseReturns({
        purchaseOrderId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.returns, total: res.total };
    },
    enabled: !!purchaseOrderId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreatePurchaseReturnMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePurchaseReturnRequest>) =>
      purchaseReturnClient.createPurchaseReturn(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: purchasingKeys.all });
      void qc.invalidateQueries({ queryKey: ["batches"] });
      void qc.invalidateQueries({ queryKey: ["stock"] });
    },
  });
}

// ---------- Payments ----------
export function usePayPurchaseMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<PayPurchaseRequest>) =>
      purchasePaymentClient.payPurchase(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: purchasingKeys.all }),
  });
}

export function useSupplierBalancesQuery(req: PartialMessage<GetSupplierBalancesRequest> = {}) {
  return useQuery({
    queryKey: purchasingKeys.balances(req),
    queryFn: async () => {
      const res = await purchasePaymentClient.getSupplierBalances(req);
      return res.balances;
    },
  });
}
