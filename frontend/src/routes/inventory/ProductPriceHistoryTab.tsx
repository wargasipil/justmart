import { Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { formatMoney, formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useProductUnitPricesQuery } from "../../queries/products";

// Per-unit SELL-price history (one open row per unit; effective_to 0 = current).
// Manager-only: ListProductUnitPrices is OWNER+PHARMACIST, so this tab is only
// mounted when the caller may see it.
export default function ProductPriceHistoryTab({
  productId,
}: {
  productId: string;
}) {
  const { t } = useTranslation();
  const page = usePageState(`unitPrices:${productId}`);
  const q = useProductUnitPricesQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
    enabled: !!productId,
  });

  return (
    <>
      {q.rows.length === 0 ? (
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.products.priceHistoryEmpty")}
        </Text>
      ) : (
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>
                  {t("inventory.products.priceHistoryUnitCol")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.priceFrom")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.priceTo")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.pricePrice")}
                </Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {q.rows.map((p) => (
                <Table.Row key={p.id}>
                  <Table.Cell>{p.unitName}</Table.Cell>
                  <Table.Cell>{formatUnix(p.effectiveFrom)}</Table.Cell>
                  <Table.Cell>
                    {p.effectiveTo > 0n
                      ? formatUnix(p.effectiveTo)
                      : t("inventory.products.priceCurrent")}
                  </Table.Cell>
                  <Table.Cell>{formatMoney(p.unitSellPrice)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      )}
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
