import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { tableClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";
import type {
  CreateTableRequest,
  DiningTable,
  UpdateTableRequest,
} from "../gen/table_iface/v1/table_pb";

export const tableKeys = {
  all: ["tables"] as const,
  list: (filters: string) => [...tableKeys.all, "list", filters] as const,
  detail: (id: string) => [...tableKeys.all, "detail", id] as const,
};

export type TableFilters = {
  includeInactive?: boolean;
  onlyOccupied?: boolean;
  area?: string;
  page?: number;
  pageSize?: number;
  /** Skip the RPC entirely outside restaurant mode (the Dashboard tile). */
  enabled?: boolean;
};

// One page of the ACTIVE WAREHOUSE's floor, each table carrying its current open
// bill (if any). `occupied` counts across ALL matches, not the page — it drives
// the "12/20 occupied" header, which a page-local count would understate.
//
// Short refetch interval: the floor is shared, so another waiter seating a party
// must show up here without a manual refresh. 10s is the same order as the
// connector poll and cheap (one small query).
export function useTablesQuery(filters: TableFilters = {}) {
  const {
    includeInactive = false,
    onlyOccupied = false,
    area = "",
    page = 0,
    pageSize = DEFAULT_PAGE_SIZE,
    enabled = true,
  } = filters;
  const q = useQuery({
    queryKey: tableKeys.list(
      JSON.stringify({ includeInactive, onlyOccupied, area, page, pageSize }),
    ),
    queryFn: async () => {
      const res = await tableClient.listTables({
        includeInactive,
        onlyOccupied,
        area,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.tables, total: res.total, occupied: res.occupied };
    },
    enabled,
    refetchInterval: 10_000,
  });
  return {
    ...q,
    rows: q.data?.rows ?? [],
    total: q.data?.total ?? 0,
    occupied: q.data?.occupied ?? 0,
  };
}

export function useTableQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: tableKeys.detail(id),
    queryFn: async () => (await tableClient.getTable({ id })).table as DiningTable | undefined,
    enabled: enabled && !!id,
  });
}

function invalidate(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: tableKeys.all });
}

export function useCreateTableMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateTableRequest>) => tableClient.createTable(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useUpdateTableMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateTableRequest>) => tableClient.updateTable(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useArchiveTableMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => tableClient.archiveTable({ id }),
    onSuccess: () => invalidate(qc),
  });
}

export function useUnarchiveTableMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => tableClient.unarchiveTable({ id }),
    onSuccess: () => invalidate(qc),
  });
}

// Opening a table returns its EXISTING bill when there is one (`resumed`), which
// is the normal way to add a second round — not an error the caller has to check
// for first.
export function useOpenTableMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { tableId: string; guestCount?: number }) =>
      tableClient.openTable({ tableId: req.tableId, guestCount: req.guestCount ?? 0 }),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useMoveOrderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { saleId: string; toTableId: string }) => tableClient.moveOrder(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

// Areas present on the current floor, for the area filter. Derived from the
// loaded page rather than a separate RPC — a restaurant has a handful of areas,
// and an "Areas" endpoint would be a query for data already on screen.
export function areasOf(tables: DiningTable[]): string[] {
  const seen = new Set<string>();
  for (const t of tables) if (t.area) seen.add(t.area);
  return [...seen].sort();
}
