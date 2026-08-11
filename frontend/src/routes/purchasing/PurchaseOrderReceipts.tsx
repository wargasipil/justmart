import { Badge, Box, Button, HStack, Heading, Spinner, Stack, Table, Text } from "@chakra-ui/react";
import { Ban } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseReceipt } from "../../gen/purchasing_iface/v1/receipt_pb";
import { formatDate } from "../../lib/format";
import { fmtUnitQty } from "../../lib/purchaseLine";
import { SERVER_ERRORS } from "../../lib/serverErrors";
import { CancelReceiptDialog } from "./CancelReceiptDialog";

// The deliveries recorded against a purchase order, plus the cancel affordance.
// Page-local to PurchaseOrderDetail; the receipts QUERY stays in the parent
// because it also feeds the returnable check, the ReturnDialog and the
// product-ref resolution — this section only renders what it is handed and owns
// the cancel dialog, which nothing else opens.

// cancel_blocked_reason is the same stable token CancelReceipt would have failed
// with, so it translates through the shared server-error catalog rather than a
// second copy of the wording.
function blockedReasonKey(token: string): string {
  return SERVER_ERRORS[token]?.i18nKey ?? "errors.generic";
}

export default function PurchaseOrderReceipts({
  receipts,
  isLoading,
  total,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  productRefs,
}: {
  receipts: PurchaseReceipt[];
  isLoading: boolean;
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (p: number) => void;
  onPageSizeChange: (n: number) => void;
  productRefs: Map<string, ProductRef>;
}) {
  const { t } = useTranslation();
  // The receipt being cancelled. The dialog stays mounted and reads `open` from
  // this being non-null — never `{cancelling && <Dialog…>}`, which would strand
  // Ark's body lock and freeze the page.
  const [cancelling, setCancelling] = useState<PurchaseReceipt | null>(null);

  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
      <Heading size="sm" mb={3}>
        {t("purchasing.receipts")}
      </Heading>
      {isLoading ? (
        <Spinner size="sm" />
      ) : receipts.length === 0 ? (
        <Text fontSize="sm" color="fg.muted">
          {t("purchasing.noReceipts")}
        </Text>
      ) : (
        <Stack gap={3}>
          {receipts.map((r) => {
            const voided = r.voidedAt !== 0n;
            return (
              <Box
                key={r.id}
                borderWidth="1px"
                borderRadius="md"
                p={3}
                // A cancelled receipt keeps its place in the list (its RCV number
                // is already burned from the counter — a gap would read as a
                // missing document), but reads as inert.
                opacity={voided ? 0.6 : 1}
              >
                <HStack justify="space-between" mb={2}>
                  <HStack gap={3}>
                    <Text
                      fontFamily="mono"
                      fontWeight="medium"
                      textDecoration={voided ? "line-through" : undefined}
                    >
                      {r.receiptNo}
                    </Text>
                    {voided && <Badge colorPalette="red">{t("purchasing.cancel.voided")}</Badge>}
                    {r.invoiceNo && (
                      <Text fontSize="sm" color="fg.muted">
                        {t("purchasing.invoiceNo")}: {r.invoiceNo}
                      </Text>
                    )}
                  </HStack>
                  <HStack gap={3}>
                    <Text fontSize="sm" color="fg.muted">
                      {formatDate(r.receivedAt)}
                    </Text>
                    {!voided &&
                      (r.cancellable ? (
                        <Button
                          size="xs"
                          variant="ghost"
                          colorPalette="red"
                          onClick={() => setCancelling(r)}
                        >
                          <Ban size={14} />
                          {t("purchasing.cancel.action")}
                        </Button>
                      ) : (
                        // Say WHY inline rather than offering a dead button: the
                        // reason (already sold, open stocktake, settled PO) is
                        // what tells the operator to reach for a return instead.
                        <Text fontSize="xs" color="fg.muted" maxW="320px" textAlign="right">
                          {t(blockedReasonKey(r.cancelBlockedReason))}
                        </Text>
                      ))}
                  </HStack>
                </HStack>
                {voided && r.voidReason && (
                  <Text fontSize="xs" color="fg.muted" mb={2}>
                    {t("purchasing.cancel.reason")}: {r.voidReason}
                  </Text>
                )}
                <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                  <Table.Root size="sm" stickyHeader>
                    <Table.Header>
                      <Table.Row>
                        <Table.ColumnHeader>{t("purchasing.selectProduct")}</Table.ColumnHeader>
                        <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                        <Table.ColumnHeader>{t("purchasing.batchNumber")}</Table.ColumnHeader>
                        <Table.ColumnHeader>{t("purchasing.expiryDate")}</Table.ColumnHeader>
                      </Table.Row>
                    </Table.Header>
                    <Table.Body>
                      {r.items.map((it) => (
                        <Table.Row key={it.id}>
                          <Table.Cell>{productRefs.get(it.productId)?.name ?? "—"}</Table.Cell>
                          <Table.Cell>{fmtUnitQty(it.qty, it.unitName, it.unitFactor)}</Table.Cell>
                          <Table.Cell>{it.batchNumber || "—"}</Table.Cell>
                          <Table.Cell>{formatDate(it.expiryDate)}</Table.Cell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </Table.Root>
                </TableScroll>
              </Box>
            );
          })}
        </Stack>
      )}
      <Pagination
        page={page}
        pageSize={pageSize}
        total={total}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />
      <CancelReceiptDialog
        open={cancelling != null}
        onClose={() => setCancelling(null)}
        receipt={cancelling}
        productRefs={productRefs}
      />
    </Box>
  );
}
