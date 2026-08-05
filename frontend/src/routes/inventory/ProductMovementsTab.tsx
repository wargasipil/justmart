import { Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useMovementsQuery } from "../../queries/stock";

function movementTypeKey(type: MovementType): string {
  switch (type) {
    case MovementType.PURCHASE:
      return "purchase";
    case MovementType.SALE:
      return "sale";
    case MovementType.ADJUSTMENT:
      return "adjustment";
    case MovementType.WRITE_OFF:
      return "writeOff";
    default:
      return "unspecified";
  }
}

// The stock ledger for this product, scoped to the active warehouse.
// Manager-only: ListMovements is OWNER+PHARMACIST, so this tab is only mounted
// when the caller may see it.
export default function ProductMovementsTab({
  productId,
}: {
  productId: string;
}) {
  const { t } = useTranslation();
  const page = usePageState(`movements:${productId}`);
  const q = useMovementsQuery({
    productId,
    page: page.page,
    pageSize: page.pageSize,
  });

  return (
    <>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>
                {t("inventory.movements.when")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>
                {t("inventory.movements.type")}
              </Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("inventory.movements.qty")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>
                {t("inventory.movements.reason")}
              </Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {q.rows.map((m) => (
              <Table.Row key={m.id}>
                <Table.Cell>{formatUnix(m.createdAt)}</Table.Cell>
                <Table.Cell>
                  {t(`inventory.movements.types.${movementTypeKey(m.type)}`)}
                </Table.Cell>
                <Table.Cell textAlign="end">
                  {m.qty > 0 ? `+${m.qty}` : m.qty}
                </Table.Cell>
                <Table.Cell>{m.reason}</Table.Cell>
              </Table.Row>
            ))}
            {q.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={4}>
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
