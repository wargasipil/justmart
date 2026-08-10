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
import type { PurchaseReceipt } from "../../gen/purchasing_iface/v1/receipt_pb";
import { toast } from "../../lib/toaster";
import { useCreatePurchaseReturnMutation } from "../../queries/purchasing";

// Send received goods back to the supplier ("retur pembelian"). Page-local, and
// stays MOUNTED driven by its `open` prop — never `{open && <Dialog…>}` or an
// early `return null`, which would strand Ark's body lock and freeze the page.

export function ReturnDialog({
  open,
  onClose,
  poId,
  receipts,
  productRefs,
}: {
  open: boolean;
  onClose: () => void;
  poId: string;
  receipts: PurchaseReceipt[];
  productRefs: Map<string, ProductRef>;
}) {
  const { t } = useTranslation();
  const createReturn = useCreatePurchaseReturnMutation();
  const today = new Date().toISOString().slice(0, 10);
  const [returnedAt, setReturnedAt] = useState(today);
  const [reason, setReason] = useState("");

  type ReturnRow = {
    purchaseReceiptItemId: string;
    productId: string;
    batchNumber: string;
    qty: number; // in the receipt line's purchasable unit
    max: number; // returnable, in the purchasable unit
    unitName: string;
    unitFactor: number;
  };
  // Flatten every returnable receipt line (on-hand > 0) across receipts.
  const buildRows = (): ReturnRow[] => {
    const rows: ReturnRow[] = [];
    for (const r of receipts) {
      for (const it of r.items) {
        if (it.returnableQty <= 0n) continue;
        const factor = Number(it.unitFactor) || 1;
        rows.push({
          purchaseReceiptItemId: it.id,
          productId: it.productId,
          batchNumber: it.batchNumber,
          qty: 0,
          max: Number(it.returnableQty) / factor,
          unitName: it.unitName,
          unitFactor: factor,
        });
      }
    }
    return rows;
  };
  const [rows, setRows] = useState<ReturnRow[]>(() => buildRows());

  // Rebuild rows when opening (receipts/returnable may have changed since the
  // last open). handleClose empties rows, so every fresh open re-derives them.
  if (open && rows.length === 0 && receipts.some((r) => r.items.some((it) => it.returnableQty > 0n))) {
    setRows(buildRows());
  }

  const handleClose = () => {
    setRows([]);
    setReason("");
    onClose();
  };

  const updateRow = (idx: number, patch: Partial<ReturnRow>) =>
    setRows((cur) => cur.map((r, i) => (i === idx ? { ...r, ...patch } : r)));

  const canSubmit =
    reason.trim().length > 0 &&
    rows.some((r) => r.qty > 0) &&
    rows.every((r) => r.qty >= 0 && r.qty <= r.max);

  const submit = async () => {
    try {
      await createReturn.mutateAsync({
        purchaseOrderId: poId,
        returnedAt,
        reason: reason.trim(),
        lines: rows
          .filter((r) => r.qty > 0)
          .map((r) => ({ purchaseReceiptItemId: r.purchaseReceiptItemId, qty: r.qty })),
      });
      toast.success(t("purchasing.return.title") + " ✓");
      handleClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && handleClose()} size="xl">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.return.title")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={4}>
                <HStack gap={3} align="flex-start">
                  <Box flex="1">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.return.returnedAt")}
                    </Text>
                    <DatePickerField value={returnedAt} onChange={setReturnedAt} />
                  </Box>
                  <Box flex="2">
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.return.reason")} *
                    </Text>
                    <Input
                      value={reason}
                      onChange={(e) => setReason(e.target.value)}
                      placeholder={t("purchasing.return.reasonPlaceholder")}
                    />
                  </Box>
                </HStack>

                {rows.length === 0 ? (
                  <Text fontSize="sm" color="fg.muted">
                    {t("purchasing.return.nothingReturnable")}
                  </Text>
                ) : (
                  <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                    <Table.Root size="sm" stickyHeader>
                      <Table.Header>
                        <Table.Row>
                          <Table.ColumnHeader>{t("purchasing.selectProduct")}</Table.ColumnHeader>
                          <Table.ColumnHeader>{t("purchasing.batchNumber")}</Table.ColumnHeader>
                          <Table.ColumnHeader>{t("purchasing.return.returnable")}</Table.ColumnHeader>
                          <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {rows.map((r, idx) => (
                          <Table.Row key={r.purchaseReceiptItemId}>
                            <Table.Cell>{productRefs.get(r.productId)?.name ?? "—"}</Table.Cell>
                            <Table.Cell>{r.batchNumber || "—"}</Table.Cell>
                            <Table.Cell color="fg.muted">
                              {r.max} {r.unitName}
                            </Table.Cell>
                            <Table.Cell>
                              <HStack gap={1}>
                                <NumberInput
                                  size="sm"
                                  width="70px"
                                  value={r.qty}
                                  onChange={(raw) => updateRow(idx, { qty: Number(raw || 0) })}
                                  max={r.max}
                                />
                                {r.unitFactor > 1 && (
                                  <Text fontSize="xs" color="fg.muted">
                                    {r.unitName}
                                  </Text>
                                )}
                              </HStack>
                            </Table.Cell>
                          </Table.Row>
                        ))}
                      </Table.Body>
                    </Table.Root>
                  </TableScroll>
                )}
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={handleClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  colorPalette="orange"
                  onClick={submit}
                  loading={createReturn.isPending}
                  disabled={!canSubmit}
                >
                  {t("purchasing.actions.return")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

