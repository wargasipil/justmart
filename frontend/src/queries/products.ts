import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { productClient } from "../lib/clients";
import { ProductImageVariant } from "../gen/inventory_iface/v1/product_pb";
import type {
  ArchiveProductRequest,
  CreateProductRequest,
  ImportProductsRequest,
  UnarchiveProductRequest,
  UpdateProductRequest,
} from "../gen/inventory_iface/v1/product_pb";

import { dataUrlFromBytes, makeImageRenditions } from "../lib/imageRenditions";
import { ALL_LIMIT, DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type ProductsQueryOpts = {
  includeInactive?: boolean;
  onlyArchived?: boolean; // when true: only archived (active=false); overrides includeInactive
  query?: string;
  opnameBefore?: string; // YYYY-MM-DD; filter to products counted before this date OR never counted
  page?: number;
  pageSize?: number;
  // Off by default for callers that mount before they should fetch — e.g. a
  // picker dialog that stays mounted (Ark body-lock rule) but must stay idle
  // until it is opened.
  enabled?: boolean;
};

export const productKeys = {
  all: ["products"] as const,
  // `enabled` is a gate, not a filter — it stays out of the key.
  list: (opts: Required<Omit<ProductsQueryOpts, "enabled">>) =>
    [...productKeys.all, "list", opts] as const,
  one: (id: string) => [...productKeys.all, "one", id] as const,
  prices: (productId: string, page: number, pageSize: number) =>
    [...productKeys.all, "prices", productId, page, pageSize] as const,
  unitPrices: (productId: string, page: number, pageSize: number) =>
    [...productKeys.all, "unitPrices", productId, page, pageSize] as const,
  restockLogs: (productId: string, page: number, pageSize: number) =>
    [...productKeys.all, "restockLogs", productId, page, pageSize] as const,
  search: (query: string) => [...productKeys.all, "search", query] as const,
  // Paging is deliberately absent: the summary covers every matching product,
  // so paging through the table must not refetch it.
  summary: (opts: ProductsSummaryOpts) => [...productKeys.all, "summary", opts] as const,
};

export const productImageKeys = {
  all: ["productImages"] as const,
  // `version` is the product's image_updated_at. Baking it into the key is what
  // makes a re-upload appear without a manual invalidate — new timestamp, new
  // cache entry — and lets the old bytes stay cached forever.
  //
  // Deliberately its OWN root key, not under productKeys: every product
  // mutation invalidates productKeys.all, and re-fetching every thumbnail on an
  // unrelated price edit is exactly the cost this cache exists to avoid.
  one: (productId: string, variant: ProductImageVariant, version: number) =>
    [...productImageKeys.all, productId, variant, version] as const,
};

// Low-stock list for the TopBar bell — products whose ready_stock in the
// caller's active warehouse is <= the configured threshold. Polls every 60s;
// also auto-refetches on warehouse switch (existing invalidateQueries) and on
// threshold update (the settings mutation invalidates ["lowStock"]).
// The dropdown shows a short list while the badge shows `total` (ALL matches),
// so the request is bounded but the count stays honest. The badge used to read
// the row count under a server-side Limit(100) and so capped at "100".
export function useLowStockQuery(
  opts: { enabled?: boolean; page?: number; pageSize?: number } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: ["lowStock", page, pageSize],
    queryFn: async () => {
      const res = await productClient.listLowStock({
        limit: pageSize,
        offset: page * pageSize,
      });
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
    onlyArchived = false,
    query = "",
    opnameBefore = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = opts;
  const q = useQuery({
    queryKey: productKeys.list({ includeInactive, onlyArchived, query, opnameBefore, page, pageSize }),
    queryFn: async () => {
      const res = await productClient.listProducts({
        includeInactive,
        onlyArchived,
        query,
        opnameBefore,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.products, total: res.total };
    },
    enabled,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// The list's filters minus paging — the summary is an aggregate over ALL
// matching products, so it must be requested with the same filter set the table
// uses and nothing else.
export type ProductsSummaryOpts = Omit<
  Required<Omit<ProductsQueryOpts, "enabled">>,
  "page" | "pageSize"
>;

/**
 * Catalog-wide stock totals (ready + on-order, each with its valuation at cost)
 * over every product matching the active filters — the stat row above the list.
 *
 * Server-side aggregate on purpose: summing `useProductsQuery().rows` would sum
 * ONE PAGE, so the figures would change as the user pages. Mirrors
 * useSalesSummaryQuery on /orders.
 *
 * OWNER + PHARMACIST only (the valuations are cost data) — pass `enabled: false`
 * from a surface a cashier can reach.
 */
export function useProductsSummaryQuery(
  opts: Partial<ProductsSummaryOpts> & { enabled?: boolean } = {},
) {
  const {
    includeInactive = false,
    onlyArchived = false,
    query = "",
    opnameBefore = "",
    enabled = true,
  } = opts;
  return useQuery({
    queryKey: productKeys.summary({ includeInactive, onlyArchived, query, opnameBefore }),
    queryFn: () =>
      productClient.getProductsSummary({ includeInactive, onlyArchived, query, opnameBefore }),
    enabled,
  });
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
  const { includeInactive = false, onlyArchived = false, query = "", opnameBefore = "" } = opts;
  const res = await productClient.listProducts({
    includeInactive,
    onlyArchived,
    query,
    opnameBefore,
    limit: ALL_LIMIT,
    offset: 0,
  });
  return res.products;
}

// Legacy BASE-unit-only price history, superseded by useProductUnitPricesQuery.
// Server-paginated; returns { rows, total }.
export function useProductPricesQuery(
  productId: string,
  opts: { page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE, enabled = true } = opts;
  const q = useQuery({
    queryKey: productKeys.prices(productId, page, pageSize),
    queryFn: async () => {
      const res = await productClient.listProductPrices({
        productId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.prices, total: res.total };
    },
    enabled: enabled && !!productId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Per-unit sell-price history (one row per change, grouped by unit). Superset of
// the base-only listProductPrices — used by the product detail Price-history tab.
// Server-paginated; returns { rows, total }.
export function useProductUnitPricesQuery(
  productId: string,
  opts: { page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE, enabled = true } = opts;
  const q = useQuery({
    queryKey: productKeys.unitPrices(productId, page, pageSize),
    queryFn: async () => {
      const res = await productClient.listProductUnitPrices({
        productId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.prices, total: res.total };
    },
    enabled: enabled && !!productId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Append-only restock history for a product in the active warehouse (newest
// first). Server-paginated; returns { rows, total }. Drives the product-detail
// "restock price history" tab. Warehouse-scoped via the X-Warehouse-Id header.
export function useProductRestockLogsQuery(
  productId: string,
  opts: { page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE, enabled = true } = opts;
  const q = useQuery({
    queryKey: productKeys.restockLogs(productId, page, pageSize),
    queryFn: async () => {
      const res = await productClient.listProductRestockLogs({
        productId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.logs, total: res.total };
    },
    enabled: enabled && !!productId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreateProductMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateProductRequest>) =>
      productClient.createProduct(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
    // The form handles errors via useServerFormErrors (field-level + fallback).
    meta: { silentError: true },
  });
}

// Bulk CSV import (create-only; existing SKUs skipped). Invalidates the list so
// the newly-imported products appear.
export function useImportProductsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ImportProductsRequest>) =>
      productClient.importProducts(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
  });
}

export function useUpdateProductMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateProductRequest>) =>
      productClient.updateProduct(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
    meta: { silentError: true },
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

export function useUnarchiveProductMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UnarchiveProductRequest>) =>
      productClient.unarchiveProduct(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
  });
}

/**
 * A product's picture as a ready-to-render data URL ("" when it has none).
 * Defaults to the THUMB rendition — per the two-rendition HARD RULE, only a
 * deliberate full-size view passes ORIGINAL.
 *
 * `version` is the product's `imageUpdatedAt`; pass 0 and the query is disabled
 * entirely, so a product with no picture costs zero requests no matter how many
 * rows render it. That matters most in POS, where one product can occupy
 * several search rows (one per sellable unit) — they share this cache entry.
 */
export function useProductImageQuery(
  productId: string,
  version: number,
  variant: ProductImageVariant = ProductImageVariant.THUMB,
  // Extra gate on top of the version check. The ORIGINAL rendition is fetched
  // ONLY while a lightbox is actually open — without this, every row on a list
  // would preload the heavy bytes and undo the whole point of the thumbnail.
  enabled = true,
) {
  return useQuery({
    queryKey: productImageKeys.one(productId, variant, version),
    queryFn: async () => {
      const res = await productClient.getProductImage({ productId, variant });
      return dataUrlFromBytes(res.imageData, res.contentType);
    },
    enabled: enabled && !!productId && version > 0,
    // Immutable by construction: the version in the key changes on re-upload,
    // so a cached entry can never go stale.
    staleTime: Infinity,
    gcTime: 30 * 60_000,
  });
}

/**
 * Upload a product's picture. Takes the raw File and does the two-rendition
 * downscale itself, so no call site can forget the thumbnail.
 */
export function useUploadProductImageMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ productId, file }: { productId: string; file: File }) => {
      const { original, thumb, contentType } = await makeImageRenditions(file);
      return productClient.uploadProductImage({
        productId,
        imageData: original,
        thumbData: thumb,
        contentType,
      });
    },
    // The product's imageUpdatedAt IS the cache key every surface renders from,
    // so re-reading the product is what swaps the picture app-wide.
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
    meta: { silentError: true },
  });
}

export function useDeleteProductImageMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (productId: string) => productClient.deleteProductImage({ productId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: productKeys.all }),
  });
}
