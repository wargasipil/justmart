import { useMemo } from "react";
import { Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import { manufacturerLabel } from "../../components/ManufacturerSelect";
import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { formatDiscount, formatMoney, formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useProductRestockLogsQuery } from "../../queries/products";
import { useManufacturerRefs, useSupplierRefs } from "../../queries/refs";

// Restock (purchase) history from the append-only log: what we paid, to whom —
// and, since the maker became a recorded fact, WHOSE goods arrived on each buy.
// Pemasok and Pabrik answer different questions (who sold it vs who made it),
// and a shop that buys one generic from three factories needs both columns to
// read its own price history.
// Manager-only — ListProductRestockLogs and ResolveSuppliers are both
// OWNER+PHARMACIST, so this tab is only mounted when the caller may see cost.
export default function ProductRestockTab({
  productId,
}: {
  productId: string;
}) {
  const { t } = useTranslation();
  const page = usePageState(`restocks:${productId}`);
  const q = useProductRestockLogsQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
    enabled: !!productId,
  });
  const supplierRefs = useSupplierRefs(
    useMemo(
      () =>
        Array.from(
          new Set((q.rows ?? []).map((r) => r.supplierId).filter(Boolean)),
        ),
      [q.rows],
    ),
  );
  // Resolve-by-IDs, batched over the page. Rows that predate the column carry
  // no id at all, so they cost nothing here and render as a dash below.
  const manufacturerRefs = useManufacturerRefs(
    useMemo(
      () =>
        Array.from(
          new Set((q.rows ?? []).map((r) => r.manufacturerId).filter(Boolean)),
        ),
      [q.rows],
    ),
  );

  return (
    <>
      {q.rows.length === 0 ? (
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.products.restockHistoryEmpty")}
        </Text>
      ) : (
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>
                  {t("inventory.products.restockSupplier")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.manufacturer")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.restockPrice")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.restockQty")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.restockDiscount")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.restockCreated")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.restockArrived")}
                </Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {q.rows.map((r) => (
                <Table.Row key={r.id}>
                  <Table.Cell>
                    {supplierRefs.get(r.supplierId)?.name ?? "—"}
                  </Table.Cell>
                  <Table.Cell>
                    {manufacturerLabel(manufacturerRefs.get(r.manufacturerId)) ??
                      "—"}
                  </Table.Cell>
                  <Table.Cell textAlign="end">
                    {formatMoney(r.price)}
                  </Table.Cell>
                  <Table.Cell textAlign="end">{r.qty.toString()}</Table.Cell>
                  <Table.Cell textAlign="end">
                    {formatDiscount(r.discountType, r.discountValue)}
                    {r.discountValue > 0n && r.discountPerItem
                      ? ` ${t("purchasing.perItemSuffix")}`
                      : ""}
                  </Table.Cell>
                  <Table.Cell>
                    {r.restockCreatedAt > 0n
                      ? formatUnix(r.restockCreatedAt)
                      : "—"}
                  </Table.Cell>
                  <Table.Cell>
                    {r.restockArrivedAt > 0n
                      ? formatUnix(r.restockArrivedAt)
                      : "—"}
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      )}
      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={q.total} loading={q.isPlaceholderData}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />
    </>
  );
}
