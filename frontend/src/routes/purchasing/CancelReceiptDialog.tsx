import {
  Box,
  Button,
  Dialog,
  HStack,
  IconButton,
  Input,
  Portal,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseReceipt } from "../../gen/purchasing_iface/v1/receipt_pb";
import { toast } from "../../lib/toaster";
import { useCancelReceiptMutation } from "../../queries/purchasing";

// Page-local, like the other purchase-order dialogs. Stays MOUNTED and is driven
// purely by its `open` prop — never `{open && <Dialog…>}` or an early
// `return null`, which would strand Ark's body lock and freeze the page.

export function CancelReceiptDialog({
  open,
  onClose,
  receipt,
  productRefs,
}: {
  open: boolean;
  onClose: () => void;
  receipt: PurchaseReceipt | null;
  productRefs: Map<string, ProductRef>;
}) {
  const { t } = useTranslation();
  const cancelMut = useCancelReceiptMutation();
  const [reason, setReason] = useState("");

  const handleClose = () => {
    setReason("");
    onClose();
  };

  const submit = async () => {
    if (!receipt || !reason.trim()) return;
    try {
      await cancelMut.mutateAsync({ id: receipt.id, reason: reason.trim() });
      toast.success(t("purchasing.cancel.done"));
      handleClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && handleClose()} size="lg">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.cancel.title")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              {receipt && (
                <Stack gap={4}>
                  <Text fontSize="sm" color="fg.muted">
                    {t("purchasing.cancel.warning")}
                  </Text>

                  {/* Spell out exactly which stock disappears — this is the last
                      screen before it does. */}
                  <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                    <Table.Root size="sm" stickyHeader>
                      <Table.Header>
                        <Table.Row>
                          <Table.ColumnHeader>{t("purchasing.selectProduct")}</Table.ColumnHeader>
                          <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                          <Table.ColumnHeader>{t("purchasing.batchNumber")}</Table.ColumnHeader>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {receipt.items.map((it) => (
                          <Table.Row key={it.id}>
                            <Table.Cell>{productRefs.get(it.productId)?.name ?? "—"}</Table.Cell>
                            <Table.Cell>
                              {Number(it.unitFactor) > 1
                                ? `${it.qty / Number(it.unitFactor)} ${it.unitName}`
                                : it.qty}
                            </Table.Cell>
                            <Table.Cell>{it.batchNumber || "—"}</Table.Cell>
                          </Table.Row>
                        ))}
                      </Table.Body>
                    </Table.Root>
                  </TableScroll>

                  <Box>
                    <Text fontSize="xs" color="fg.muted" mb={1}>
                      {t("purchasing.cancel.reason")} *
                    </Text>
                    <Input
                      value={reason}
                      onChange={(e) => setReason(e.target.value)}
                      placeholder={t("purchasing.cancel.reasonPlaceholder")}
                    />
                  </Box>
                </Stack>
              )}
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={handleClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  colorPalette="red"
                  onClick={submit}
                  loading={cancelMut.isPending}
                  disabled={!reason.trim()}
                >
                  {t("purchasing.cancel.confirm")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

