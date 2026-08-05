import { useMemo } from "react";
import { HStack, Table, Text } from "@chakra-ui/react";

import ExpiryBadge from "../../components/ExpiryBadge";
import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useBatchesQuery } from "../../queries/batches";
import { useSupplierRefs } from "../../queries/refs";
import { useTranslation } from "react-i18next";

// In-stock lots for this product in the ACTIVE warehouse. The one detail tab the
// till also sees — it answers "what's actually on the shelf and when does it
// expire" — so the supplier and cost columns are dropped when showCost is off
// (the backend redacts both fields for those roles anyway).
export default function ProductBatchesTab({
  productId,
  showCost,
}: {
  productId: string;
  showCost: boolean;
}) {
  const { t } = useTranslation();
  // Keyed by product id so navigating to a different product resets the pager
  // rather than landing on page 3 of a shorter list.
  const page = usePageState(`batches:${productId}`);
  const q = useBatchesQuery({
    productId,
    onlyInStock: true,
    page: page.page,
    pageSize: page.pageSize,
  });
  // Supplier ids are redacted out of batch reads for the till, and
  // ResolveSuppliers is manager-only — an empty id list keeps the hook from
  // firing at all.
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

  return (
    <>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>
                {t("inventory.batches.batchNumber")}
              </Table.ColumnHeader>
              {showCost && (
                <Table.ColumnHeader>
                  {t("inventory.batches.supplier")}
                </Table.ColumnHeader>
              )}
              <Table.ColumnHeader>
                {t("inventory.batches.expiry")}
              </Table.ColumnHeader>
              {showCost && (
                <Table.ColumnHeader>
                  {t("inventory.batches.cost")}
                </Table.ColumnHeader>
              )}
              <Table.ColumnHeader textAlign="end">
                {t("inventory.batches.qty")}
              </Table.ColumnHeader>
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
                  <HStack gap={2}>
                    <Text>{b.expiryDate}</Text>
                    <ExpiryBadge expiry={b.expiryDate} />
                  </HStack>
                </Table.Cell>
                {showCost && (
                  <Table.Cell>{formatMoney(b.costPrice)}</Table.Cell>
                )}
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
        total={q.total}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />
    </>
  );
}
