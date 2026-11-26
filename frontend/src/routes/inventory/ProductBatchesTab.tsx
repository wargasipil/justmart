import { useMemo, useState } from "react";
import { Table, Text } from "@chakra-ui/react";

import Pagination from "../../components/Pagination";
import SortableColumnHeader, {
  cycleColumnSort,
  type ColumnSort,
} from "../../components/SortableColumnHeader";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { baseUnitsIn, costForUnit, costUnitsOf } from "../../lib/unitCost";
import { useBatchesQuery } from "../../queries/batches";
import { useSupplierRefs } from "../../queries/refs";
import { useTranslation } from "react-i18next";

import type { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import type { ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import BatchExpiryCell from "./BatchExpiryCell";
import BatchExpiryDialog from "./BatchExpiryDialog";

// Every cost column sorts by the server's one "cost" key (b.cost_price, per
// base unit): a lot that is cheaper per tablet is cheaper per box too, so one
// order serves them all. Locally each column keeps its own key, "cost:<unit>",
// so the arrow lands on the column that was clicked and a click on another
// cost column starts its own asc/desc/clear cycle.
const COST_SORT = "cost";
const COST_SORT_PREFIX = `${COST_SORT}:`;

/** One Cost column: the base units it is priced per, and its header. */
type CostColumn = { key: string; label: string; factor: bigint };

// In-stock lots for this product in the ACTIVE warehouse — "what's actually on
// the shelf and when does it expire". The supplier and cost columns follow
// showCost (every role today; the backend redacts both fields if cost
// visibility is ever narrowed again). The expiry correction is a WRITE —
// SetBatchExpiry is OWNER + PHARMACIST — so it hangs off canManage
// (lib/roles.ts canManageProducts), not showCost.
export default function ProductBatchesTab({
  productId,
  productName,
  showCost,
  canManage,
  units = [],
}: {
  productId: string;
  productName?: string;
  showCost: boolean;
  canManage: boolean;
  /** The product's units — one Cost column per unit (tablet, strip, box). */
  units?: ProductUnit[];
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Batch | null>(null);
  // Sorted on the SERVER (ListBatches sort_by / sort_desc) so the order spans
  // every page, not just the 25 rows on screen. null = the default order,
  // soonest expiry first.
  const [sort, setSort] = useState<ColumnSort>(null);
  const serverSortBy = sort?.by.startsWith(COST_SORT_PREFIX) ? COST_SORT : sort?.by;
  // Keyed by product id so navigating to a different product resets the pager
  // rather than landing on page 3 of a shorter list; keyed by the server sort
  // too, so re-sorting starts again from the first page.
  const page = usePageState(`batches:${productId}:${serverSortBy ?? ""}:${sort?.desc ?? ""}`);
  const q = useBatchesQuery({
    productId,
    onlyInStock: true,
    sortBy: serverSortBy,
    sortDesc: sort?.desc,
    page: page.page,
    pageSize: page.pageSize,
  });
  // Keyed off showCost because the supplier id is one of the fields the
  // redactor blanks: if cost visibility is ever narrowed again the ids arrive
  // empty, and an empty list keeps the hook from firing at all.
  const supplierRefs = useSupplierRefs(
    useMemo(
      () =>
        showCost
          ? Array.from(
              new Set(
                (q.rows ?? [])
                  .map((b) => b.supplierId)
                  .filter((s): s is string => !!s),
              ),
            )
          : [],
      [q.rows, showCost],
    ),
  );

  // One Cost column per unit — tablet, strip, box — because a supplier's
  // invoice prices the pack and the shelf sells the piece. A lot received
  // against a per-box order line carries that box's exact cost, so the box
  // column reads the faktur's figure rather than per-tablet × 30 (see
  // lib/unitCost.ts). A product sold only in its base unit keeps the one plain
  // "Cost" column.
  const costColumns = useMemo((): CostColumn[] => {
    if (!showCost) return [];
    const perUnit = costUnitsOf(units);
    if (perUnit.length < 2 || !perUnit.some((u) => u.isBase)) {
      return [{ key: "base", label: t("inventory.batches.cost"), factor: 1n }];
    }
    return perUnit.map((u) => ({
      key: u.id,
      label: t("inventory.batches.costInUnit", { unit: u.name }),
      factor: baseUnitsIn(u),
    }));
  }, [showCost, units, t]);

  // Props for one sortable header; `by` is the local sort key (the server's
  // sort_by column, or "cost:<unit>" for a Cost column).
  const sortable = (by: string) => ({
    active: sort?.by === by,
    desc: sort?.by === by && sort.desc,
    onClick: () => setSort((s) => cycleColumnSort(s, by)),
  });

  // Lot number, expiry and quantity, plus supplier and the cost columns.
  const columnCount = 3 + (showCost ? 1 : 0) + costColumns.length;

  return (
    <>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header bg="bg.muted">
            <Table.Row>
              <SortableColumnHeader
                label={t("inventory.batches.batchNumber")}
                {...sortable("batch_number")}
              />
              {showCost && (
                <SortableColumnHeader
                  label={t("inventory.batches.supplier")}
                  {...sortable("supplier")}
                />
              )}
              <SortableColumnHeader
                label={t("inventory.batches.expiry")}
                {...sortable("expiry")}
              />
              {costColumns.map((c) => (
                <SortableColumnHeader
                  key={c.key}
                  label={c.label}
                  align="end"
                  {...sortable(COST_SORT_PREFIX + c.key)}
                />
              ))}
              <SortableColumnHeader
                label={t("inventory.batches.qty")}
                align="end"
                {...sortable("qty")}
              />
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {q.rows.map((b) => (
              <Table.Row key={b.id}>
                <Table.Cell>{b.batchNumber || "—"}</Table.Cell>
                {showCost && (
                  <Table.Cell>
                    {b.supplierId
                      ? (() => {
                          const s = supplierRefs.get(b.supplierId);
                          return s ? `${s.code} · ${s.name}` : "—";
                        })()
                      : "—"}
                  </Table.Cell>
                )}
                <Table.Cell>
                  <BatchExpiryCell batch={b} onEdit={canManage ? () => setEditing(b) : undefined} />
                </Table.Cell>
                {costColumns.map((c) => (
                  <Table.Cell key={c.key} textAlign="end">
                    {formatMoney(costForUnit(b.costPrice, b.costPackPrice, b.costPackFactor, c.factor))}
                  </Table.Cell>
                ))}
                <Table.Cell textAlign="end">
                  {b.currentQuantity.toString()}
                </Table.Cell>
              </Table.Row>
            ))}
            {q.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={columnCount}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      </TableScroll>
      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={q.total} loading={q.isPlaceholderData}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />
      <BatchExpiryDialog batch={editing} productName={productName} onClose={() => setEditing(null)} />
    </>
  );
}
