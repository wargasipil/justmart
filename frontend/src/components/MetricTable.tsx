import { Box, HStack, Spinner, Table, Text } from "@chakra-ui/react";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  MetricType,
  type MetricOrder,
  type MetricStock,
  Sort,
  SortDirection,
} from "../gen/analytics_iface/v1/analytics_pb";
import TableScroll from "./TableScroll";
import { formatDate, formatMoney } from "../lib/format";
import { matchSort, nextSort, type ColField } from "../lib/metricSort";

// MetricTable renders a paginated metric grid. The page passes:
//   - `ids` in the order the server returned (already sorted)
//   - `order` + `stock` maps keyed by id (whichever was requested)
//   - `metricTypes` so the table shows only the requested column groups
//   - `labelById` Map<id, displayName>; missing -> "—"
//   - `sort` + `onSortChange` to drive backend-side sort via column headers
type Props = {
  ids: string[];
  order?: MetricOrder;
  stock?: MetricStock;
  metricTypes: MetricType[];
  // visibleFields drives per-column visibility (field ids like "order.terjual",
  // "stock.ready"). The set is independent of metricTypes — even if ORDER is in
  // metricTypes, individual order columns are hidden when their field id is
  // absent. When omitted, defaults to "every field in any requested metric".
  visibleFields?: Set<string>;
  labelById: Map<string, string>;
  sort?: Sort;
  // undefined = clear sort (revert to backend default = sort by dimension key)
  onSortChange?: (sort: Sort | undefined) => void;
  dimensionHeader: string;
  isLoading?: boolean;
};

export default function MetricTable({
  ids,
  order,
  stock,
  metricTypes,
  visibleFields,
  labelById,
  sort,
  onSortChange,
  dimensionHeader,
  isLoading,
}: Props) {
  const { t } = useTranslation();
  // Per-field visibility, gated by both visibleFields (UI choice) and
  // metricTypes (the group must actually be requested from the backend).
  const want = (groupOk: boolean, fieldId: string) =>
    groupOk && (visibleFields ? visibleFields.has(fieldId) : true);
  const hasOrderGroup = metricTypes.includes(MetricType.ORDER);
  const hasStockGroup = metricTypes.includes(MetricType.STOCK);
  const showTerjual     = want(hasOrderGroup, "order.terjual");
  const showHpp         = want(hasOrderGroup, "order.hpp");
  const showProfit      = want(hasOrderGroup, "order.profit");
  const showLastOrder   = want(hasOrderGroup, "order.lastOrder");
  const showAvgSold     = want(hasOrderGroup, "order.avgSold");
  const showReady       = want(hasStockGroup, "stock.ready");
  const showOngoing     = want(hasStockGroup, "stock.ongoing");
  const showLastRestock = want(hasStockGroup, "stock.lastRestock");
  const showExpiring    = want(hasStockGroup, "stock.expiring");
  const orderCols =
    (showTerjual ? 1 : 0) + (showHpp ? 1 : 0) + (showProfit ? 1 : 0) +
    (showLastOrder ? 1 : 0) + (showAvgSold ? 1 : 0);
  const stockCols =
    (showReady ? 1 : 0) + (showOngoing ? 1 : 0) +
    (showLastRestock ? 1 : 0) + (showExpiring ? 1 : 0);
  const hasOrder = orderCols > 0;
  const hasStock = stockCols > 0;
  const colSpan = 1 + orderCols + stockCols;

  // Column-header click toggles sort: unsorted -> DESC -> ASC -> unsorted.
  // ColField / matchSort / nextSort live in lib/metricSort.ts.
  const sortFor = (
    field: ColField,
  ): { arrow: React.ReactNode; next: Sort | undefined; hasNext: boolean } => {
    const current = matchSort(sort, field);
    const arrow = current
      ? current === SortDirection.DESC
        ? <ArrowDown size={12} />
        : <ArrowUp size={12} />
      : null;
    return { arrow, next: nextSort(field, current), hasNext: true };
  };

  const hasGroupRow = hasOrder || hasStock;
  const { ref: groupRowRef, height: groupRowH } = useStickyRowHeight(hasGroupRow);

  return (
    <TableScroll>
      <Table.Root
        size="sm"
        variant="line"
        stickyHeader
        // Chakra's stickyHeader pins EVERY header row to the same offset
        // (`header: { "& :where(tr)": { top: var(--table-sticky-offset, 0) } }`).
        // This is the app's only two-row header, so without an override the field
        // row sticks at 0 directly ON TOP of the group row and the Order/Stock
        // spans vanish the moment you scroll. Pin the group row at 0 and push the
        // field row down by the group row's MEASURED height — a hardcoded value is
        // off by the collapsed border (2.25rem renders as 37px, not 36px) and would
        // drift again with font size, zoom, or a taller translated label. The group
        // row also takes the higher z-index so its rowSpan-ed dimension cell paints
        // over the field row rather than under it.
        css={
          hasGroupRow
            ? {
                "& thead tr:first-of-type": { zIndex: 2 },
                "& thead tr:nth-of-type(2)": { top: `${groupRowH}px` },
              }
            : undefined
        }
      >
        <Table.Header>
          {hasGroupRow && (
            <Table.Row ref={groupRowRef} bg="bg.muted">
              <Table.ColumnHeader rowSpan={2}>{dimensionHeader}</Table.ColumnHeader>
              {hasOrder && (
                <Table.ColumnHeader colSpan={orderCols} textAlign="center">
                  {t("analytics.metric.group.order")}
                </Table.ColumnHeader>
              )}
              {hasStock && (
                <Table.ColumnHeader colSpan={stockCols} textAlign="center">
                  {t("analytics.metric.group.stock")}
                </Table.ColumnHeader>
              )}
            </Table.Row>
          )}
          <Table.Row>
            {!hasGroupRow && (
              <Table.ColumnHeader>{dimensionHeader}</Table.ColumnHeader>
            )}
            {showTerjual && (
              <SortableHeader
                label={t("analytics.metric.order.terjual")}
                meta={sortFor("order-terjual")}
                onClick={onSortChange}
              />
            )}
            {showHpp && (
              <SortableHeader
                label={t("analytics.metric.order.hpp")}
                meta={sortFor("order-hpp")}
                onClick={onSortChange}
              />
            )}
            {showProfit && (
              <SortableHeader
                label={t("analytics.metric.order.profit")}
                meta={sortFor("order-profit")}
                onClick={onSortChange}
              />
            )}
            {showLastOrder && (
              <SortableHeader
                label={t("analytics.metric.order.lastOrder")}
                meta={sortFor("order-lastOrder")}
                onClick={onSortChange}
              />
            )}
            {showAvgSold && (
              <SortableHeader
                label={t("analytics.metric.order.avgSold")}
                meta={sortFor("order-avgSold")}
                onClick={onSortChange}
              />
            )}
            {showReady && (
              <SortableHeader
                label={t("analytics.metric.stock.ready")}
                meta={sortFor("stock-ready")}
                onClick={onSortChange}
              />
            )}
            {showOngoing && (
              <SortableHeader
                label={t("analytics.metric.stock.ongoing")}
                meta={sortFor("stock-ongoing")}
                onClick={onSortChange}
              />
            )}
            {showLastRestock && (
              <SortableHeader
                label={t("analytics.metric.stock.lastRestock")}
                meta={sortFor("stock-lastRestock")}
                onClick={onSortChange}
              />
            )}
            {showExpiring && (
              <SortableHeader
                label={t("analytics.metric.stock.expiring")}
                meta={sortFor("stock-expiring")}
                onClick={onSortChange}
              />
            )}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {isLoading && (
            <Table.Row>
              <Table.Cell colSpan={colSpan}>
                <Box p={6} textAlign="center">
                  <Spinner size="sm" />
                </Box>
              </Table.Cell>
            </Table.Row>
          )}
          {!isLoading &&
            ids.map((id) => {
              const o = order?.data[id];
              const s = stock?.data[id];
              // Never fall back to the raw id — a 36-char UUID is both a HARD-RULE
              // violation (referenced names resolve to "—" while pending) and the
              // single worst thing for this column's width.
              const label = labelById.get(id) ?? "—";
              return (
                <Table.Row key={id}>
                  {/* Bounded + ellipsised: Table.ScrollArea sets white-space:nowrap,
                      so an unbounded long product name stretches the table and
                      pushes every metric column off the viewport. */}
                  <Table.Cell maxW={LABEL_MAX_W} title={label}>
                    <Text truncate>{label}</Text>
                  </Table.Cell>
                  {showTerjual && <Num>{formatMoney(Number(o?.terjual ?? 0n))}</Num>}
                  {showHpp && <Num>{formatMoney(Number(o?.hpp ?? 0n))}</Num>}
                  {showProfit && <Num>{formatMoney(Number(o?.profit ?? 0n))}</Num>}
                  {showLastOrder && (
                    <Table.Cell textAlign="end" whiteSpace="nowrap">
                      {(o?.lastOrderUnix ?? 0n) > 0n ? formatDay(o!.lastOrderUnix) : "—"}
                    </Table.Cell>
                  )}
                  {showAvgSold && <Num>{String(o?.avgSold ?? 0n)}</Num>}
                  {showReady && <Num>{String(s?.ready ?? 0n)}</Num>}
                  {showOngoing && <Num>{String(s?.ongoing ?? 0n)}</Num>}
                  {showLastRestock && (
                    <Table.Cell textAlign="end" whiteSpace="nowrap">
                      {(s?.lastRestockUnix ?? 0n) > 0n ? formatDay(s!.lastRestockUnix) : "—"}
                    </Table.Cell>
                  )}
                  {showExpiring && <Num>{String(s?.expiring ?? 0n)}</Num>}
                </Table.Row>
              );
            })}
          {!isLoading && ids.length === 0 && (
            <Table.Row>
              <Table.Cell colSpan={colSpan}>
                <Text color="fg.muted" textAlign="center" py={4}>
                  {t("common.noResults")}
                </Text>
              </Table.Cell>
            </Table.Row>
          )}
        </Table.Body>
      </Table.Root>
    </TableScroll>
  );
}

/** Cap for the dimension (day / product / user) column. Long product names get
 *  ellipsised instead of stretching the grid past the viewport. */
const LABEL_MAX_W = "22rem";

// Date-only, NOT formatUnix (which appends time-of-day). "Last order" /
// "Last restock" are the two widest cells in the grid — at ~150px each
// ("Jul 11, 2026, 10:02 AM") the two of them are what pushed the Product table
// past the viewport and clipped the final column. The clock time carries no
// meaning in a roll-up spanning weeks, and dropping it recovers ~110px.
function formatDay(sec: bigint): string {
  return formatDate(Number(sec) * 1000);
}

/**
 * Live height of the group header row, used as the sticky offset for the field
 * row beneath it. Measured rather than assumed: the row's rendered height is its
 * content height PLUS the collapsed border, so any constant is off by a pixel
 * and drifts further with font scaling, browser zoom, or a translated label that
 * wraps. A 1px error is visible — the field row rides up over the group row's
 * bottom border. Returns 0 while unmeasured / when there is no group row, which
 * is the correct offset for a single-row header anyway.
 */
function useStickyRowHeight(enabled: boolean) {
  const ref = useRef<HTMLTableRowElement>(null);
  const [height, setHeight] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!enabled || !el) {
      setHeight(0);
      return;
    }
    const measure = () => setHeight(el.getBoundingClientRect().height);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [enabled]);
  return { ref, height };
}

// Num = a numeric metric cell: right-aligned + tabular mono so digits line up
// column-wise and "Rp 1.234.567" is comparable to "Rp 12" at a glance. Matches
// the money-cell convention used by Orders / OrderDetail / SupplierDetail.
function Num({ children }: { children: React.ReactNode }) {
  return (
    <Table.Cell textAlign="end" fontFamily="mono" whiteSpace="nowrap">
      {children}
    </Table.Cell>
  );
}

// SortableHeader = clickable column header that cycles direction. Right-aligned
// to sit over its numeric column; the arrow is in a fixed-width slot so the
// label doesn't jump sideways when the sort indicator appears or clears.
function SortableHeader({
  label,
  meta,
  onClick,
}: {
  label: string;
  meta: { arrow: React.ReactNode; next: Sort | undefined; hasNext: boolean };
  onClick?: (sort: Sort | undefined) => void;
}) {
  const clickable = !!onClick && meta.hasNext;
  return (
    <Table.ColumnHeader
      textAlign="end"
      whiteSpace="nowrap"
      cursor={clickable ? "pointer" : "default"}
      userSelect="none"
      onClick={clickable ? () => onClick?.(meta.next) : undefined}
    >
      <HStack gap={1} justify="flex-end">
        <Text>{label}</Text>
        <Box w="12px" flexShrink={0} lineHeight={0}>
          {meta.arrow}
        </Box>
      </HStack>
    </Table.ColumnHeader>
  );
}
