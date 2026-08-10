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

import DatePickerField from "../../components/DatePicker";
import NumberInput from "../../components/NumberInput";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseOrder, PurchaseOrderItem } from "../../gen/purchasing_iface/v1/order_pb";
import { toast } from "../../lib/toaster";
import { useCreateReceiptMutation } from "../../queries/purchasing";

// Record a delivery against a purchase order. Page-local — opened only from
// PurchaseOrderDetail and coupled to its PO shape, so it lives beside the route
// rather than in components/ (the shared vocabulary surfaced by /components).
//
// Stays MOUNTED and is driven purely by its `open` prop — never
// `{open && <Dialog…>}` or an early `return null`. Unmounting an open
// Dialog.Root leaves Ark's body lock in place and freezes the whole page.

export function ReceiveDialog({
  open,
  onClose,
  po,
  productRefs,
}: {
  open: boolean;
  onClose: () => void;
  po: PurchaseOrder;
  productRefs: Map<string, ProductRef>;
}) {
  const { t } = useTranslation();
  const createReceipt = useCreateReceiptMutation();
  const today = new Date().toISOString().slice(0, 10);
  const [receivedAt, setReceivedAt] = useState(today);
  const [note, setNote] = useState("");

  // No unit cost here by design: it is fully determined by the PO line (net of
  // its discount, plus PPN) and the backend derives it — see common.NetUnitCost.
  // The request therefore omits unit_cost_price, whose 0 default IS the "derive
  // it" signal. Re-adding an input would only let a receive contradict the order
  // it is fulfilling; the proto field survives as an API-level escape hatch.
  type LineDraft = {
    purchaseOrderItemId: string;
    qty: number; // in the PO line's purchasable unit
    batchNumber: string;
    expiryDate: string;
    remaining: number; // in the purchasable unit
    unitName: string;
    unitFactor: number;
    productUnitId: string;
  };
  const initialLines = (items: PurchaseOrderItem[]): LineDraft[] =>
    items
      .filter((it) => it.receivedQty < it.orderedQty)
      .map((it) => {
        const factor = Number(it.unitFactor) || 1;
        const remainingUnit = (it.orderedQty - it.receivedQty) / factor;
        return {
          purchaseOrderItemId: it.id,
          qty: remainingUnit,
          batchNumber: "",
          expiryDate: "",
          remaining: remainingUnit,
          unitName: it.unitName,
          unitFactor: factor,
          productUnitId: it.productUnitId,
        };
      });
  const [lines, setLines] = useState<LineDraft[]>(() => initialLines(po.items));

  // Reset when opening for a different PO.
  if (open && lines.length === 0 && po.items.some((it) => it.receivedQty < it.orderedQty)) {
    setLines(initialLines(po.items));
  }

  const canSubmit =
    lines.length > 0 &&
    lines.every((l) => l.qty > 0 && l.qty <= l.remaining && l.expiryDate);

  const submit = async () => {
    try {
      await createReceipt.mutateAsync({
        purchaseOrderId: po.id,
        receivedAt,
        note,
        lines: lines
          .filter((l) => l.qty > 0)
          .map((l) => ({
            purchaseOrderItemId: l.purchaseOrderItemId,
            qty: l.qty,
            batchNumber: l.batchNumber,
            expiryDate: l.expiryDate,
            productUnitId: l.productUnitId,
          })),
      });
      toast.success(t("purchasing.receipt") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  const updateLine = (idx: number, patch: Partial<LineDraft>) =>
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()} size="xl">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.receiveTitle")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={4}>
                <HStack gap={3}>
                  <Box flex="1">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.receivedAt")}
                    </Text>
                    <DatePickerField value={receivedAt} onChange={setReceivedAt} />
                  </Box>
                  <Box flex="2">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.note")}
                    </Text>
                    <Input value={note} onChange={(e) => setNote(e.target.value)} />
                  </Box>
                </HStack>

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
                      {lines.map((l, idx) => {
                        const poItem = po.items.find((it) => it.id === l.purchaseOrderItemId);
                        return (
                          <Table.Row key={l.purchaseOrderItemId}>
                            <Table.Cell>
                              {poItem ? productRefs.get(poItem.productId)?.name ?? "—" : "—"}
                              <Text fontSize="xs" color="fg.muted">
                                {t("purchasing.remaining")}: {l.remaining} {l.unitName}
                              </Text>
                            </Table.Cell>
                            <Table.Cell>
                              <HStack gap={1}>
                                <NumberInput
                                  size="sm"
                                  width="70px"
                                  value={l.qty}
                                  onChange={(raw) =>
                                    updateLine(idx, { qty: Number(raw || 0) })
                                  }
                                  max={l.remaining}
                                />
                                {l.unitFactor > 1 && (
                                  <Text fontSize="xs" color="fg.muted">
                                    {l.unitName}
                                  </Text>
                                )}
                              </HStack>
                            </Table.Cell>
                            <Table.Cell>
                              <Input
                                size="sm"
                                value={l.batchNumber}
                                onChange={(e) => updateLine(idx, { batchNumber: e.target.value })}
                                w="120px"
                              />
                            </Table.Cell>
                            <Table.Cell>
                              <DatePickerField
                                size="sm"
                                value={l.expiryDate}
                                onChange={(v) => updateLine(idx, { expiryDate: v })}
                              />
                            </Table.Cell>
                          </Table.Row>
                        );
                      })}
                    </Table.Body>
                  </Table.Root>
                </TableScroll>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  colorPalette="blue"
                  onClick={submit}
                  loading={createReceipt.isPending}
                  disabled={!canSubmit}
                >
                  {t("purchasing.actions.receive")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

// Cancel an accepted restock entered in error ("batal terima"). A text reason is
// required, so this is a real Dialog with a Field rather than <ConfirmDialog> —
// the app has no native prompt().
//
// The receipt is passed in and may be null while the dialog animates closed;
// Dialog.Root therefore stays mounted and the CONTENT is guarded on the data.
