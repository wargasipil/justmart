import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { POStatus } from "../../gen/purchasing_iface/v1/order_pb";
import { resolveRange, type DateRange } from "../../lib/dateRange";

// Tab path -> PO status. ONE source of truth: the Purchasing shell derives the
// active status from the URL and publishes it here; PurchaseOrdersList reads it
// back instead of taking a prop. That's what lets the stat row (above the tabs)
// and the table (below them) describe the same slice without two derivations
// that can drift apart.
export const PO_STATUS_BY_TAB: Record<string, POStatus> = {
  all: POStatus.PO_STATUS_UNSPECIFIED,
  draft: POStatus.PO_STATUS_DRAFT,
  sent: POStatus.PO_STATUS_SENT,
  partial: POStatus.PO_STATUS_PARTIALLY_RECEIVED,
  received: POStatus.PO_STATUS_RECEIVED,
  closed: POStatus.PO_STATUS_CLOSED,
  voided: POStatus.PO_STATUS_VOIDED,
};

// The filter set both the list request and the summary request carry, minus
// paging. Passing this object verbatim to each is what keeps the stat row
// honest — it can only ever summarize the rows the table is showing.
export type RestockRequestFilters = {
  status: POStatus;
  supplierId: string;
  onlyOutstanding: boolean;
  query: string;
  fromUnix: bigint;
  toUnix: bigint;
  dateField: string;
};

export type RestockFilters = {
  request: RestockRequestFilters;
  // Raw controls, owned here because the toolbar (in the list) and the stat row
  // (in the shell, above the tabs) sit on opposite sides of the tab strip.
  searchInput: string;
  setSearchInput: (v: string) => void;
  supplierId: string;
  setSupplierId: (v: string) => void;
  onlyOutstanding: boolean;
  setOnlyOutstanding: (v: boolean) => void;
  dateField: string;
  setDateField: (v: string) => void;
  range: DateRange;
  setRange: (v: DateRange) => void;
};

const RestockFiltersContext = createContext<RestockFilters | null>(null);

export function RestockFiltersProvider({
  status,
  children,
}: {
  status: POStatus;
  children: ReactNode;
}) {
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [supplierId, setSupplierId] = useState("");
  const [onlyOutstanding, setOnlyOutstanding] = useState(false);
  // "" = Any date (the picker's own off state) — send no bounds at all then.
  const [dateField, setDateField] = useState("");
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));

  // Debounce the search box (250ms) into the query that drives the requests.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const fromUnix = dateField ? BigInt(range.fromUnix) : 0n;
  const toUnix = dateField ? BigInt(range.toUnix) : 0n;

  const value = useMemo<RestockFilters>(
    () => ({
      request: { status, supplierId, onlyOutstanding, query, fromUnix, toUnix, dateField },
      searchInput,
      setSearchInput,
      supplierId,
      setSupplierId,
      onlyOutstanding,
      setOnlyOutstanding,
      dateField,
      setDateField,
      range,
      setRange,
    }),
    [status, supplierId, onlyOutstanding, query, fromUnix, toUnix, dateField, searchInput, range],
  );

  return (
    <RestockFiltersContext.Provider value={value}>{children}</RestockFiltersContext.Provider>
  );
}

export function useRestockFilters(): RestockFilters {
  const ctx = useContext(RestockFiltersContext);
  if (!ctx) {
    throw new Error("useRestockFilters must be used inside <RestockFiltersProvider>");
  }
  return ctx;
}

// A stable string of the active filters, for usePageState's reset key — paging
// snaps back to 0 whenever any of them changes.
export function restockPageKey(f: RestockRequestFilters): string {
  return `${f.status}|${f.supplierId}|${f.onlyOutstanding}|${f.query}|${f.dateField}|${f.fromUnix}|${f.toUnix}`;
}
