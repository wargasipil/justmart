import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { productClient } from "../lib/clients";
import type {
  ArchiveProductRequest,
  CreateProductRequest,
  UpdateProductRequest,
} from "../gen/inventory_iface/v1/product_pb";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type ProductsQueryOpts = {
  includeInactive?: boolean;
  query?: string;
  opnameBefore?: string; // YYYY-MM-DD; filter to products counted before this date OR never counted
  page?: number;
  pageSize?: number;
};

export const productKeys = {
  all: ["products"] as const,
  list: (opts: Required<ProductsQueryOpts>) =>
    [...productKeys.all, "list", opts] as const,
  one: (id: string) => [...productKeys.all, "one", id] as const,
  prices: (productId: string) =>
    [...productKeys.all, "prices", productId] as const,
  unitPrices: (productId: string) =>
    [...productKeys.all, "unitPrices", productId] as const,
  search: (query: string) => [...productKeys.all, "search", query] as const,
};

// Low-stock list for the TopBar bell — products whose ready_stock in the
// caller's active warehouse is <= the configured threshold. Polls every 60s;
// also auto-refetches on warehouse switch (existing invalidateQueries) and on
// threshold update (the settings mutation invalidates ["lowStock"]).
export function useLowStockQuery(opts: { enabled?: boolean } = {}) {
  const q = useQuery({
    queryKey: ["lowStock"],
    queryFn: async () => {
      const res = await productClient.listLowStock({});
      return { products: res.products, threshold: res.threshold, total: res.total };
    },
    enabled: opts.enabled ?? true,
    refetchInterval: 60_000,
    staleTime: 30_000,
    meta: { silentError: true },
  });
  return q;
}

// Single product (detail page). GetProduct is stock-enriched server-side
// (ready_stock for the active warehouse + on_order_stock).
export function useProductQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: productKeys.one(id),
    queryFn: async () => {
      const res = await productClient.getProduct({ id });
      return res.product;
    },
    enabled: enabled && !!id,
  });
}

// Server-paginated. Returns { rows, total } plus the React Query state.
// For page-level name maps / preload selects pass { pageSize: ALL_LIMIT }.
export function useProductsQuery(opts: ProductsQueryOpts = {}) {
  const {
    includeInactive = false,
    query = "",
    opnameBefore = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
  } = opts;
  const q = useQuery({
    queryKey: productKeys.list({ includeInactive, query, opnameBefore, page, pageSize }),
    queryFn: async () => {
      const res = await productClient.listProducts({
        includeInactive,
        query,
        opnameBefore,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.products, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Convenience for page-level name maps / preload selects that need the full list.
export function useAllProductsQuery(includeInactive = false) {
  return useProductsQuery({ includeInactive, pageSize: ALL_LIMIT });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>.
// Mirrors the SearchCustomers / searchSuppliers contract.
export async function searchProducts(query: string) {
  const res = await productClient.searchProducts({ query, limit: 20 });
  return res.products;
}

// Imperative one-shot fetch of ALL products matching the filter (cap
// ALL_LIMIT), for CSV export. Not a hook — call from an export handler.
export async function fetchProductsForExport(opts: ProductsQueryOpts = {}) {
  const { includeInactive = false, query = "", opnameBefore = "" } = opts;
  const res = await productClient.listProducts({
    includeInactive,
    query,
    opnameBefore,
    limit: ALL_LIMIT,
    offset: 0,
  });
  return res.products;
}

export function useProductPricesQuery(productId: string, enabled = true) {
  return useQuery({
    queryKey: productKeys.prices(productId),
    queryFn: async () => {
      const res = await productClient.listProductPrices({ productId });
      return res.prices;
    },
    enabled: enabled && !!productId,
  });
}

// Per-unit sell-price history (one row per change, grouped by unit). Superset of
// the base-only listProductPrices — used by the product detail Price-history tab.
export function useProductUnitPricesQuery(productId: string, enabled = true) {
  return useQuery({
    queryKey: productKeys.unitPrices(productId),
    queryFn: async () => {
      const res = await productClient.listProductUnitPrices({ productId });
      return res.prices;
    },
    enabled: enabled && !!productId,
  });
}

export function useCreateProductMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateProductRequest>) =>
      productClient.createProduct(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
  });
}

export function useUpdateProductMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateProductRequest>) =>
      productClient.updateProduct(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
  });
}

export function useArchiveProductMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveProductRequest>) =>
      productClient.archiveProduct(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
  });
}
