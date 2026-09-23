import { Box, HStack, Heading, Stack, Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseReturn } from "../../gen/purchasing_iface/v1/return_pb";
import { formatDate, formatMoney } from "../../lib/format";
import { fmtUnitQty } from "../../lib/purchaseLine";

// The returns recorded against a purchase order — goods that physically went
// back to the supplier, each accruing a refund.
//
// The twin of PurchaseOrderReceipts beside it, and page-local for the same
// reason: the returns QUERY stays in the parent because its rows feed the
// product-ref resolution, while this section only renders what it is handed.
//
// It renders NOTHING when there are no returns, rather than an empty card. A
// return is the exception, not a stage of the order, so a permanent empty
// section would imply every order is missing one.
export default function PurchaseOrderReturns({
  returns,
  total,
  page,
  pageSize,
  isPlaceholderData,
  onPageChange,
  onPageSizeChange,
  productRefs,
}: {
  returns: PurchaseReturn[];
  total: number;
  page: number;
  pageSize: number;
  isPlaceholderData: boolean;
  onPageChange: (p: number) => void;
  onPageSizeChange: (n: number) => void;
  productRefs: Map<string, ProductRef>;
}) {
  const { t } = useTranslation();
  if (returns.length === 0) return null;

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
        loading={isPlaceholderData}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />
    </Box>
  );
}
