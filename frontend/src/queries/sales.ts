import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { saleClient } from "../lib/clients";
import type {
  AddItemRequest,
  AttachPrescriptionRequest,
  CompleteSaleRequest,
  DetachPrescriptionRequest,
  SetServiceFeeRequest,
  SetLineDiscountRequest,
  ClearLineDiscountRequest,
  SetCartDiscountRequest,
  GetSalesSummaryRequest,
  ListSalesRequest,
  RefundSaleRequest,
  RemoveItemRequest,
  SetItemQuantityRequest,
  SetSaleCustomerRequest,
  VoidSaleRequest,
} from "../gen/pos_iface/v1/sale_pb";
import { ALL_LIMIT } from "../lib/pagination";

export const saleKeys = {
  all: ["sales"] as const,
  list: (filters: object) => [...saleKeys.all, "list", filters] as const,
  summary: (filters: object) => [...saleKeys.all, "summary", filters] as const,
  detail: (id: string) => [...saleKeys.all, "detail", id] as const,
  todaySnapshot: () => [...saleKeys.all, "today-snapshot"] as const,
};

// orderType is restaurant-mode only ("" | TAKEAWAY | DELIVERY). DINE_IN is not
// accepted here — a seated order is opened through TableService.OpenTable, the
// only path that binds a table.
export function useStartSaleMutation() {
  return useMutation({
    mutationFn: (req: { orderType?: string } = {}) =>
      saleClient.startSale({ orderType: req.orderType ?? "" }),
  });
}

export function useAddItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddItemRequest>) => saleClient.addItem(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetItemQuantityMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetItemQuantityRequest>) =>
      saleClient.setItemQuantity(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useRemoveItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RemoveItemRequest>) => saleClient.removeItem(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetSaleCustomerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetSaleCustomerRequest>) =>
      saleClient.setSaleCustomer(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useAttachPrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AttachPrescriptionRequest>) =>
      saleClient.attachPrescription(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useDetachPrescriptionMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<DetachPrescriptionRequest>) =>
      saleClient.detachPrescription(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetServiceFeeMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetServiceFeeRequest>) =>
      saleClient.setServiceFee(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetLineDiscountMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetLineDiscountRequest>) =>
      saleClient.setLineDiscount(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useClearLineDiscountMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ClearLineDiscountRequest>) =>
      saleClient.clearLineDiscount(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useSetCartDiscountMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetCartDiscountRequest>) =>
      saleClient.setCartDiscount(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
  });
}

export function useCompleteSaleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CompleteSaleRequest>) => saleClient.completeSale(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: saleKeys.all });
      void qc.invalidateQueries({ queryKey: ["stock"] });
      void qc.invalidateQueries({ queryKey: ["batches"] });
    },
  });
}

export function useVoidSaleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<VoidSaleRequest>) => saleClient.voidSale(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: saleKeys.all }),
  });
}

// Full-order refund of a COMPLETED sale (OWNER/PHARMACIST). Invalidates sales +
// stock reads so the order list/detail, summaries, dashboards, and inventory
// reflect the reversal.
export function useRefundSaleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<RefundSaleRequest>) => saleClient.refundSale(req),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: saleKeys.all });
      void qc.invalidateQueries({ queryKey: ["stock"] });
      void qc.invalidateQueries({ queryKey: ["batches"] });
    },
  });
}

// Send the lines added since the last fire to the kitchen printer. Incremental
// server-side: firing twice with nothing new is a no-op success (firedItems: 0)
// and does NOT reprint, so a double tap is harmless.
export function useFireToKitchenMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { saleId: string; connectorDeviceId?: string; printerName?: string }) =>
      saleClient.fireToKitchen(req),
    // The response carries no Sale, but every line's firedAt just changed —
    // refetch so the cart's per-line "fired" cue is accurate.
    onSuccess: (_res, req) => {
      qc.invalidateQueries({ queryKey: saleKeys.detail(req.saleId) });
    },
    meta: { silentError: true },
  });
}

// Cook-facing note on a cart line ("no ice", "extra pedas"). Never moves an
// amount — a priced modifier is a separate line, not a note.
export function useSetItemNoteMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { saleId: string; itemId: string; note: string }) =>
      saleClient.setItemNote(req),
    onSuccess: (res) => {
      if (res.sale?.id) qc.setQueryData(saleKeys.detail(res.sale.id), res.sale);
    },
    meta: { silentError: true },
  });
}

// Print a completed sale's receipt. An empty connectorDeviceId/printerName lets
// the server resolve the saved default target (or the sole connected connector).
export function usePrintReceiptMutation() {
  return useMutation({
    mutationFn: (req: { saleId: string; connectorDeviceId?: string; printerName?: string }) =>
      saleClient.printReceipt(req),
  });
}

// Server-paginated. Returns { rows, total }. Caller sets limit/offset on filters.
export function useListSalesQuery(filters: PartialMessage<ListSalesRequest> = {}) {
  const q = useQuery({
    queryKey: saleKeys.list(filters),
    queryFn: async () => {
      const res = await saleClient.listSales(filters);
      return { rows: res.sales, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Server-side aggregate over ALL rows matching the same filters as
// useListSalesQuery (NOT a client-side sum of a page). Backs the order-history
// summary bar.
export function useSalesSummaryQuery(filters: PartialMessage<GetSalesSummaryRequest> = {}) {
  return useQuery({
    queryKey: saleKeys.summary(filters),
    queryFn: () => saleClient.getSalesSummary(filters),
    staleTime: 30_000,
  });
}

// Imperative one-shot fetch of ALL rows matching the filters (cap ALL_LIMIT),
// for CSV export. Not a hook — call from an export handler.
export async function fetchSalesForExport(filters: PartialMessage<ListSalesRequest> = {}) {
  const res = await saleClient.listSales({ ...filters, limit: ALL_LIMIT, offset: 0 });
  return res.sales;
}

// useSaleQuery loads one sale by id (drives the OrderDetail page). The Connect
// response carries the full proto incl. items preloaded; the detail page
// resolves referenced names (customer, cashier, products) via the existing
// Resolve<Domain> hooks.
export function useSaleQuery(id: string) {
  return useQuery({
    queryKey: saleKeys.detail(id),
    queryFn: async () => {
      const res = await saleClient.getSale({ id });
      return res.sale;
    },
    enabled: !!id,
  });
}

export function useTodaySnapshotQuery(opts: { cashierUserId?: string } = {}) {
  const cashierUserId = opts.cashierUserId ?? "";
  return useQuery({
    queryKey: [...saleKeys.todaySnapshot(), { cashierUserId }],
    queryFn: async () => saleClient.getTodaySnapshot({ cashierUserId }),
    staleTime: 30_000,
  });
}
