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
import { useRef } from "react";
import { useTranslation } from "react-i18next";

import DatePickerField from "../../components/DatePicker";
import ExpiryInput from "../../components/ExpiryInput";
import NumberInput from "../../components/NumberInput";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { ExpirySource } from "../../gen/inventory_iface/v1/batch_pb";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseOrder } from "../../gen/purchasing_iface/v1/order_pb";
import type { PurchaseReceipt } from "../../gen/purchasing_iface/v1/receipt_pb";
import { toast } from "../../lib/toaster";
import { useCreateReceiptMutation, useReplaceReceiptMutation } from "../../queries/purchasing";
import { useReceiveDraft } from "./useReceiveDraft";

// Record a delivery against a purchase order — or, with `replacing`, correct
// one already recorded ("ubah penerimaan"). Page-local — opened only from
// PurchaseOrderDetail and coupled to its PO shape, so it lives beside the route
// rather than in components/ (the shared vocabulary surfaced by /components).
// The draft (lines, header, what each line saves) lives in useReceiveDraft.
//
// Editing is ReplaceReceipt: the server cancels the delivery and records the
// corrected lines as a NEW one with its own RCV number, in one transaction, so
// it is offered exactly where Batalkan is and needs a reason like Batalkan does.
//
// Stays MOUNTED and is driven purely by its `open` prop — never
// `{open && <Dialog…>}` or an early `return null`. Unmounting an open
// Dialog.Root leaves Ark's body lock in place and freezes the whole page.

export function ReceiveDialog({
  open,
  onClose,
  po,
  productRefs,
  replacing,
}: {
  open: boolean;
  onClose: () => void;
  po: PurchaseOrder;
  productRefs: Map<string, ProductRef>;
  /** The delivery being edited; absent for a new delivery. */
  replacing?: PurchaseReceipt | null;
}) {
  const { t } = useTranslation();
  const createReceipt = useCreateReceiptMutation();
  const replaceReceipt = useReplaceReceiptMutation();
  const { isPharmacy, header, patchHeader, lines, updateLine, generation, lineExpiry, canSubmit, requestLines } =
    useReceiveDraft(open, po, replacing);
  const expiryRefs = useRef<(HTMLInputElement | null)[]>([]);
  const fieldMinW = replacing ? "150px" : undefined;

  const submit = async () => {
    const body = { receivedAt: header.receivedAt, note: header.note, lines: requestLines() };
    try {
      if (replacing) {
        await replaceReceipt.mutateAsync({
          ...body,
          id: replacing.id,
          reason: header.reason.trim(),
          invoiceNo: header.invoiceNo,
        });
        toast.success(t("purchasing.edit.done"));
      } else {
        await createReceipt.mutateAsync({ ...body, purchaseOrderId: po.id });
        toast.success(t("purchasing.receipt") + " ✓");
      }
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()} size="xl">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>
                {replacing
                  ? t("purchasing.edit.title", { no: replacing.receiptNo })
                  : t("purchasing.receiveTitle")}
              </Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={4}>
                {replacing && (
                  <Text fontSize="sm" color="fg.muted">
                    {t("purchasing.edit.warning")}
                  </Text>
                )}
                {/* Wraps only in edit mode, where a third field (faktur) joins. */}
                <HStack gap={3} wrap="wrap">
                  <Box flex="1" minW={fieldMinW}>
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.receivedAt")}
                    </Text>
                    <DatePickerField value={header.receivedAt} onChange={(v) => patchHeader({ receivedAt: v })} />
                  </Box>
                  {/* A new delivery inherits the order's faktur; an edit shows
                      the one it recorded, because a wrong faktur no is one of
                      the mistakes it exists to fix. */}
                  {replacing && (
                    <Box flex="1" minW={fieldMinW}>
                      <Text fontSize="xs" color="fg.muted">
                        {t("purchasing.invoiceNo")}
                      </Text>
                      <Input
                        value={header.invoiceNo}
                        onChange={(e) => patchHeader({ invoiceNo: e.target.value })}
                      />
                    </Box>
                  )}
                  <Box flex="2" minW={fieldMinW}>
                    <Text fontSize="xs" color="fg.muted">
                      {t("purchasing.note")}
                    </Text>
                    <Input value={header.note} onChange={(e) => patchHeader({ note: e.target.value })} />
                  </Box>
                </HStack>

                {lines.length > 1 && (
                  <Text fontSize="sm" color="fg.muted">
                    {t("purchasing.receiveHint")}
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
                      {lines.map((l, idx) => {
                        const name = productRefs.get(l.productId)?.name ?? "—";
                        const expiry = lineExpiry(l);
                        return (
                          <Table.Row key={`${generation}-${l.key}`} verticalAlign="top">
                            <Table.Cell>
                              {name}
                              <Text fontSize="xs" color="fg.muted">
                                {replacing ? t("purchasing.edit.max") : t("purchasing.remaining")}:{" "}
                                {l.remaining} {l.unitName}
                                {l.qty === 0 && <> · {t("purchasing.notInDelivery")}</>}
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
                            <Table.Cell minW="190px">
                              {expiry.source === ExpirySource.NONE ? (
                                // The product does not expire: nothing to type.
                                // Should this pack carry a date after all, it
                                // can still be entered.
                                <Stack gap={0} align="start">
                                  <Text fontSize="sm" color="fg.muted" py={1.5}>
                                    {t("inventory.batches.noExpiry")}
                                  </Text>
                                  <Button
                                    size="2xs"
                                    variant="plain"
                                    colorPalette="blue"
                                    px={0}
                                    onClick={() => updateLine(idx, { expiryTouched: true, expiryTyped: "" })}
                                  >
                                    {t("inventory.batches.expiryDialog.newDate")}
                                  </Button>
                                </Stack>
                              ) : (
                                <ExpiryInput
                                  size="sm"
                                  value={expiry.iso}
                                  isDefault={expiry.isDefault}
                                  blockExpired={isPharmacy}
                                  onChange={(v) => updateLine(idx, { expiryTyped: v, expiryTouched: true })}
                                  onEnter={() => expiryRefs.current[idx + 1]?.focus()}
                                  inputRef={(el) => {
                                    expiryRefs.current[idx] = el;
                                  }}
                                  aria-label={`${t("purchasing.expiryDate")} — ${name}`}
                                />
                              )}
                            </Table.Cell>
                          </Table.Row>
                        );
                      })}
                    </Table.Body>
                  </Table.Root>
                </TableScroll>

                {/* Required, like Batalkan's: the edit cancels a delivery, and
                    the cancelled row shows why. */}
                {replacing && (
                  <Box>
                    <Text fontSize="xs" color="fg.muted" mb={1}>
                      {t("purchasing.cancel.reason")} *
                    </Text>
                    <Input
                      value={header.reason}
                      onChange={(e) => patchHeader({ reason: e.target.value })}
                      placeholder={t("purchasing.edit.reasonPlaceholder")}
                      aria-label={t("purchasing.cancel.reason")}
                    />
                  </Box>
                )}
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
                  loading={createReceipt.isPending || replaceReceipt.isPending}
                  disabled={!canSubmit}
                >
                  {replacing ? t("purchasing.edit.confirm") : t("purchasing.actions.receive")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
