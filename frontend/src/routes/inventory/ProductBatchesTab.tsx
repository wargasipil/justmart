import { useMemo, useState } from "react";
import { HStack, Table, Text } from "@chakra-ui/react";

import EnumSelect from "../../components/EnumSelect";
import Pagination from "../../components/Pagination";
import SortableColumnHeader, {
  cycleColumnSort,
  type ColumnSort,
} from "../../components/SortableColumnHeader";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useBatchesQuery } from "../../queries/batches";
import { useSupplierRefs } from "../../queries/refs";
import { useTranslation } from "react-i18next";

import type { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import type { ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import BatchExpiryCell from "./BatchExpiryCell";
import BatchExpiryDialog from "./BatchExpiryDialog";

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
  /** The product's units — the Cost column can be shown per pack (strip, box). */
  units?: ProductUnit[];
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<Batch | null>(null);
  // Sorted on the SERVER (ListBatches sort_by / sort_desc) so the order spans
  // every page, not just the 25 rows on screen. null = the default order,
  // soonest expiry first.
  const [sort, setSort] = useState<ColumnSort>(null);
  // Keyed by product id so navigating to a different product resets the pager
  // rather than landing on page 3 of a shorter list; keyed by the sort too, so
  // re-sorting starts again from the first page.
  const page = usePageState(`batches:${productId}:${sort?.by ?? ""}:${sort?.desc ?? ""}`);
  const q = useBatchesQuery({
    productId,
    onlyInStock: true,
    sortBy: sort?.by,
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

  // A lot's cost is stored per BASE unit (tablet). The picker re-expresses the
  // Cost column per pack — cost × factor, exact in bigint — because that is how
  // a supplier invoice prices it. Display only: the server still sorts by the
  // per-base cost, and multiplying every row by one factor keeps that order.
  const costUnits = useMemo(
    () =>
      units
        .filter((u) => u.active && (u.isBase || u.factor > 1n))
        .sort((a, b) => Number(unitFactor(a) - unitFactor(b))),
    [units],
  );
  const baseUnit = costUnits.find((u) => u.isBase);
  const [costUnitId, setCostUnitId] = useState("");
  // An id from another product (or a pack since removed) falls back to the base.
  const costUnit = costUnits.find((u) => u.id === costUnitId) ?? baseUnit;
  const costFactor = costUnit ? unitFactor(costUnit) : 1n;
  const showCostUnits = showCost && !!baseUnit && costUnits.length > 1;
  const costUnitLabel = (u: ProductUnit) =>
    u.isBase
      ? t("inventory.batches.costUnitBase", { unit: u.name })
      : t("inventory.batches.costUnitPack", {
          unit: u.name,
          factor: u.factor.toString(),
          base: baseUnit?.name ?? "",
        });

  // Props for one sortable header; `by` is the server's sort_by column key.
  const sortable = (by: string) => ({
    active: sort?.by === by,
    desc: sort?.by === by && sort.desc,
    onClick: () => setSort((s) => cycleColumnSort(s, by)),
  });

  return (
    <>
      {showCostUnits && (
        <HStack justify="flex-end" gap={2} mb={2}>
          <Text fontSize="sm" color="fg.muted">
            {t("inventory.batches.costShowPer")}
          </Text>
          <EnumSelect
            size="sm"
            width="220px"
            ariaLabel={t("inventory.batches.costShowPer")}
            value={costUnit?.id ?? ""}
            onChange={setCostUnitId}
            items={costUnits}
            itemToValue={(u) => u.id}
            itemToString={costUnitLabel}
          />
        </HStack>
      )}
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
              {showCost && (
                <SortableColumnHeader
                  label={
                    showCostUnits && costUnit
                      ? t("inventory.batches.costInUnit", { unit: costUnit.name })
                      : t("inventory.batches.cost")
                  }
                  {...sortable("cost")}
                />
              )}
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
                {showCost && <Table.Cell>{formatMoney(b.costPrice * costFactor)}</Table.Cell>}
                <Table.Cell textAlign="end">
                  {b.currentQuantity.toString()}
                </Table.Cell>
              </Table.Row>
            ))}
            {q.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={showCost ? 5 : 3}>
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

// Base units per 1 of `u`. The base unit is 1 by definition, whatever the row
// stores.
function unitFactor(u: ProductUnit): bigint {
  return u.isBase ? 1n : u.factor;
}
