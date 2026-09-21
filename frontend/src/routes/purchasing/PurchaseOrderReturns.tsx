import { Box, HStack, Heading, Stack, Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseReturn } from "../../gen/purchasing_iface/v1/return_pb";
import { formatDate, formatMoney } from "../../lib/format";
import { fmtUnitQty } from "../../lib/purchaseLine";

// The goods sent back to the supplier against a purchase order. Page-local to
// PurchaseOrderDetail and shaped exactly like PurchaseOrderReturns' sibling
// PurchaseOrderReceipts: the QUERY stays in the parent (it also feeds the PO's
// returned_amount context and shares the product refs), and this section only
// renders what it is handed.
//
// The parent renders nothing at all when there are no returns — an empty
// section on a PO that was never returned reads as a missing feature — so the
// caller keeps that guard rather than this file rendering an empty state.
export default function PurchaseOrderReturns({
  returns,
  total,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  productRefs,
}: {
  returns: PurchaseReturn[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (p: number) => void;
  onPageSizeChange: (n: number) => void;
  productRefs: Map<string, ProductRef>;
}) {
  const { t } = useTranslation();
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
      <Heading size="sm" mb={3}>
        {t("purchasing.return.section")}
      </Heading>
      <Stack gap={3}>
        {returns.map((r) => (
          <Box key={r.id} borderWidth="1px" borderRadius="md" p={3}>
            <HStack justify="space-between" mb={2} wrap="wrap" gap={2}>
              <HStack gap={3}>
                <Text fontFamily="mono" fontWeight="medium">
                  {r.returnNo}
                </Text>
                <Text fontSize="sm" color="fg.muted">
                  {r.reason}
                </Text>
              </HStack>
              <HStack gap={3}>
                <Text fontFamily="mono" fontSize="sm">
                  −{formatMoney(Number(r.refundAmount))}
                </Text>
                <Text fontSize="sm" color="fg.muted">
                  {formatDate(r.returnedAt)}
                </Text>
              </HStack>
            </HStack>
            <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
              <Table.Root size="sm" stickyHeader>
                <Table.Body>
                  {r.items.map((it) => (
                    <Table.Row key={it.id}>
                      <Table.Cell>{productRefs.get(it.productId)?.name ?? "—"}</Table.Cell>
                      <Table.Cell>{fmtUnitQty(it.qty, it.unitName, it.unitFactor)}</Table.Cell>
                      <Table.Cell fontFamily="mono" color="fg.muted">
                        {formatMoney(Number(it.unitCostPrice))}
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Root>
            </TableScroll>
          </Box>
        ))}
      </Stack>
      <Pagination
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />
    </Box>
  );
}
