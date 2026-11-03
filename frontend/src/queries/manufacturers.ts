import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { manufacturerClient } from "../lib/clients";
import type {
  ArchiveManufacturerRequest,
  CreateManufacturerRequest,
  UnarchiveManufacturerRequest,
  UpdateManufacturerRequest,
} from "../gen/inventory_iface/v1/manufacturer_pb";

import { ALL_LIMIT, DEFAULT_PAGE_SIZE, keepPageData } from "../lib/pagination";

export type ManufacturersQueryOpts = {
  includeInactive?: boolean;
  query?: string;
  page?: number;
  pageSize?: number;
};

export const manufacturerKeys = {
  all: ["manufacturers"] as const,
  list: (opts: Required<ManufacturersQueryOpts>) => [...manufacturerKeys.all, "list", opts] as const,
  one: (id: string) => [...manufacturerKeys.all, "one", id] as const,
  search: (query: string) => [...manufacturerKeys.all, "search", query] as const,
  products: (id: string, query: string, includeArchived: boolean, page: number, pageSize: number) =>
    [...manufacturerKeys.all, "products", id, query, includeArchived, page, pageSize] as const,
  // Paging is deliberately absent: the summary covers every matching
  // manufacturer, so paging through the table must not refetch it.
  summary: (opts: ManufacturersSummaryOpts) => [...manufacturerKeys.all, "summary", opts] as const,
};

export type ManufacturersSummaryOpts = Required<Omit<ManufacturersQueryOpts, "page" | "pageSize">>;

/**
 * Total manufacturers matching the active filters — the stat row above the list.
 *
 * Server-side aggregate on purpose: counting `useManufacturersQuery().rows`
 * would count ONE PAGE, so the figure would change as the user pages. Mirrors
 * useSuppliersSummaryQuery.
 */
export function useManufacturersSummaryQuery(opts: Partial<ManufacturersSummaryOpts> = {}) {
  const { includeInactive = false, query = "" } = opts;
  return useQuery({
    queryKey: manufacturerKeys.summary({ includeInactive, query }),
    queryFn: () => manufacturerClient.getManufacturersSummary({ includeInactive, query }),
  });
}

// Single manufacturer (detail / edit pre-fill).
export function useManufacturerQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: manufacturerKeys.one(id),
    queryFn: async () => {
      const res = await manufacturerClient.getManufacturer({ id });
      return res.manufacturer;
    },
    enabled: enabled && !!id,
  });
}

// The catalog this pabrik makes — the second half of the detail page, without
// which a manufacturer is just the seven fields the edit drawer already shows.
// Server-paginated; returns { rows, total }. `ready_stock` is scoped to the
// active warehouse by the server (X-Warehouse-Id), like every other stock read.
export function useManufacturerProductsQuery(
  manufacturerId: string,
  opts: { query?: string; includeArchived?: boolean; page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const {
    query = "",
    includeArchived = false,
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = opts;
  const key = manufacturerKeys.products(manufacturerId, query, includeArchived, page, pageSize);
  const q = useQuery({
    queryKey: key,
    placeholderData: keepPageData(key),
    queryFn: async () => {
      const res = await manufacturerClient.listManufacturerProducts({
        manufacturerId,
        query,
        includeArchived,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.products, total: res.total };
    },
    enabled: enabled && !!manufacturerId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Server-paginated. Returns { rows, total }.
export function useManufacturersQuery(opts: ManufacturersQueryOpts = {}) {
  const { includeInactive = false, query = "", page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: manufacturerKeys.list({ includeInactive, query, page, pageSize }),
    placeholderData: keepPageData(
      manufacturerKeys.list({ includeInactive, query, page, pageSize }),
    ),
    queryFn: async () => {
      const res = await manufacturerClient.listManufacturers({
        includeInactive,
        query,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.manufacturers, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useAllManufacturersQuery(includeInactive = false) {
  return useManufacturersQuery({ includeInactive, pageSize: ALL_LIMIT });
}

// Imperative search — call directly from <SearchableSelect loadOptions={...}>
// rather than via a hook (one call per debounced keystroke, no need to memoize
// in React Query). Returns the slice of matching manufacturers (max 20).
export async function searchManufacturers(query: string) {
  const res = await manufacturerClient.searchManufacturers({ query, limit: 20 });
  return res.manufacturers;
}

export function useCreateManufacturerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateManufacturerRequest>) =>
      manufacturerClient.createManufacturer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: manufacturerKeys.all }),
    // The form handles errors via useServerFormErrors (field-level + fallback).
    meta: { silentError: true },
  });
}

export function useUpdateManufacturerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateManufacturerRequest>) =>
      manufacturerClient.updateManufacturer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: manufacturerKeys.all }),
    meta: { silentError: true },
  });
}

export function useArchiveManufacturerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ArchiveManufacturerRequest>) =>
      manufacturerClient.archiveManufacturer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: manufacturerKeys.all }),
  });
}

// Restore an archived manufacturer to active. No silentError: a name collision
// (manufacturer.name_taken) surfaces via the global translated toast — there's
// no form here to attach a field error to.
export function useUnarchiveManufacturerMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UnarchiveManufacturerRequest>) =>
      manufacturerClient.unarchiveManufacturer(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: manufacturerKeys.all }),
  });
}
