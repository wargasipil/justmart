import {
  OrderMetricField,
  Sort,
  SortDirection,
  StockMetricField,
} from "../gen/analytics_iface/v1/analytics_pb";

// Pure sort derivation for the analytics <MetricTable> — no JSX, no React.
// Split out of the component because it is the one part of that file that is
// plain logic, and because `ColField` used to be declared twice (once inside
// the component, once at module scope) with the two copies free to drift.
//
// The `"<group>-<field>"` string is the table's own column key. It exists
// because a column header needs ONE token that can address either arm of the
// proto's `Sort.field` oneof; the two maps below are the only place that token
// is translated to a proto enum.

export type ColField =
  | "order-terjual"
  | "order-hpp"
  | "order-profit"
  | "order-lastOrder"
  | "order-avgSold"
  | "stock-ready"
  | "stock-ongoing"
  | "stock-lastRestock"
  | "stock-expiring";

const ORDER_FIELD_MAP: Record<string, OrderMetricField | undefined> = {
  "order-terjual":   OrderMetricField.TERJUAL,
  "order-hpp":       OrderMetricField.HPP,
  "order-profit":    OrderMetricField.PROFIT,
  "order-lastOrder": OrderMetricField.LAST_ORDER,
  "order-avgSold":   OrderMetricField.AVG_SOLD,
};
const STOCK_FIELD_MAP: Record<string, StockMetricField | undefined> = {
  "stock-ready":       StockMetricField.READY,
  "stock-ongoing":     StockMetricField.ONGOING,
  "stock-lastRestock": StockMetricField.LAST_RESTOCK,
  "stock-expiring":    StockMetricField.EXPIRING,
};

/** Current direction for a column, or null when the sort points elsewhere. */
export function matchSort(sort: Sort | undefined, field: ColField): SortDirection | null {
  if (!sort || !sort.field) return null;
  switch (sort.field.case) {
    case "order": {
      const want = ORDER_FIELD_MAP[field];
      return want !== undefined && sort.field.value === want ? sort.direction : null;
    }
    case "stock": {
      const want = STOCK_FIELD_MAP[field];
      return want !== undefined && sort.field.value === want ? sort.direction : null;
    }
  }
  return null;
}

/** Cycles a column's sort: unsorted -> DESC -> ASC -> unsorted (undefined). */
export function nextSort(field: ColField, current: SortDirection | null): Sort | undefined {
  const newDir =
    current === null
      ? SortDirection.DESC
      : current === SortDirection.DESC
      ? SortDirection.ASC
      : null;
  if (newDir === null) {
    // Cycle back to "unsorted" — let the caller clear it.
    return undefined;
  }
  if (field.startsWith("order-")) {
    const orderField = ORDER_FIELD_MAP[field];
    if (orderField === undefined) return undefined;
    return new Sort({
      direction: newDir,
      field: { case: "order", value: orderField },
    });
  }
  const stockField = STOCK_FIELD_MAP[field];
  if (stockField === undefined) return undefined;
  return new Sort({
    direction: newDir,
    field: { case: "stock", value: stockField },
  });
}
