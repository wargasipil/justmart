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
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import DatePickerField from "../../components/DatePicker";
import ExpiryInput from "../../components/ExpiryInput";
import NumberInput from "../../components/NumberInput";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { ExpirySource } from "../../gen/inventory_iface/v1/batch_pb";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseOrder, PurchaseOrderItem } from "../../gen/purchasing_iface/v1/order_pb";
import { formatDateOnly } from "../../lib/dateRange";
import { daysUntilExpiry, defaultExpiry } from "../../lib/expiry";
import { toast } from "../../lib/toaster";
import { useProductExpiryDefaultsQuery } from "../../queries/products";
import { useCreateReceiptMutation } from "../../queries/purchasing";
import { useBusinessMode } from "../../queries/settings";

// Record a delivery against a purchase order. Page-local — opened only from
// PurchaseOrderDetail and coupled to its PO shape, so it lives beside the route
// rather than in components/ (the shared vocabulary surfaced by /components).
//
// Stays MOUNTED and is driven purely by its `open` prop — never
// `{open && <Dialog…>}` or an early `return null`. Unmounting an open
// Dialog.Root leaves Ark's body lock in place and freezes the whole page.

// No unit cost here by design: it is fully determined by the PO line (net of
// its discount, plus PPN) and the backend derives it — see common.NetUnitCost.
// The request therefore omits unit_cost_price, whose 0 default IS the "derive
// it" signal. Re-adding an input would only let a receive contradict the order
// it is fulfilling; the proto field survives as an API-level escape hatch.
type LineDraft = {
  purchaseOrderItemId: string;
  productId: string;
  qty: number; // in the PO line's purchasable unit
  batchNumber: string;
  // What was TYPED. Until a person types, the line shows its product's default
  // instead (derived each render, so changing the received date moves it).
  expiryTyped: string;
  expiryTouched: boolean;
  remaining: number; // in the purchasable unit
  unitName: string;
  unitFactor: number;
  productUnitId: string;
};

function initialLines(items: PurchaseOrderItem[]): LineDraft[] {
  return items
    .filter((it) => it.receivedQty < it.orderedQty)
    .map((it) => {
      const factor = Number(it.unitFactor) || 1;
      const remainingUnit = (it.orderedQty - it.receivedQty) / factor;
      return {
        purchaseOrderItemId: it.id,
        productId: it.productId,
        qty: remainingUnit,
        batchNumber: "",
        expiryTyped: "",
        expiryTouched: false,
        remaining: remainingUnit,
        unitName: it.unitName,
        unitFactor: factor,
        productUnitId: it.productUnitId,
      };
    });
}

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
  const { isPharmacy } = useBusinessMode();
  // The LOCAL calendar date. toISOString() is UTC, which on a UTC+7 shop made
  // anything received before 07:00 default to yesterday.
  const [receivedAt, setReceivedAt] = useState(() => formatDateOnly(new Date()));
  const [note, setNote] = useState("");
  const [lines, setLines] = useState<LineDraft[]>(() => initialLines(po.items));
  // Bumped on every open so each line's <ExpiryInput> remounts with no
  // half-typed text left over from the last delivery.
  const [generation, setGeneration] = useState(0);
  const expiryRefs = useRef<(HTMLInputElement | null)[]>([]);

  // Each product's expiry setting, fetched in one read. The server only stores
  // the setting; the date is computed here (defaultExpiry) and seeded into the
  // line until somebody types over it.
  const defaultsQ = useProductExpiryDefaultsQuery(
    po.items.map((it) => it.productId),
    { enabled: open },
  );

  // Re-derive everything on each closed → open edge. The dialog stays mounted,
  // so without this a second delivery reopened on the previous one's draft:
  // the old "remaining", batch number and expiry, ready to be submitted again
  // as a new lot. Done during render (not in an effect) so the first open frame
  // already shows the fresh lines.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setLines(initialLines(po.items));
      setReceivedAt(formatDateOnly(new Date()));
      setNote("");
      setGeneration((g) => g + 1);
    }
  }

  // The expiry a line will be saved with, and where it came from.
  const lineExpiry = (l: LineDraft): { iso: string; source: ExpirySource; isDefault: boolean } => {
    if (l.expiryTouched) return { iso: l.expiryTyped, source: ExpirySource.ENTERED, isDefault: false };
    const seed = defaultExpiry(defaultsQ.byId.get(l.productId), receivedAt, isPharmacy);
    if (!seed) return { iso: "", source: ExpirySource.ENTERED, isDefault: false };
    return { ...seed, isDefault: seed.source === ExpirySource.DEFAULT };
  };
  // Ready to save: a "does not expire" line needs nothing; any other needs a
  // date, and in pharmacy mode one that has not already passed (the server
  // refuses it too — retail only warns, in the readout).
  const expiryReady = (l: LineDraft) => {
    const e = lineExpiry(l);
    if (e.source === ExpirySource.NONE) return true;
    if (!e.iso) return false;
    return !(isPharmacy && daysUntilExpiry(e.iso) < 0);
  };

  // A line at 0 is "not in this delivery" and is left off the request, so it
  // stays outstanding on the order. Only the lines that arrived need an expiry.
  const arriving = lines.filter((l) => l.qty > 0);
  const canSubmit =
    arriving.length > 0 && arriving.every((l) => l.qty <= l.remaining && expiryReady(l));

  const submit = async () => {
    try {
      await createReceipt.mutateAsync({
        purchaseOrderId: po.id,
        receivedAt,
        note,
        lines: arriving.map((l) => {
          const e = lineExpiry(l);
          return {
            purchaseOrderItemId: l.purchaseOrderItemId,
            qty: l.qty,
            batchNumber: l.batchNumber,
            expiryDate: e.iso,
            expirySource: e.source,
            productUnitId: l.productUnitId,
          };
        }),
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
                          <Table.Row key={`${generation}-${l.purchaseOrderItemId}`} verticalAlign="top">
                            <Table.Cell>
                              {name}
                              <Text fontSize="xs" color="fg.muted">
                                {t("purchasing.remaining")}: {l.remaining} {l.unitName}
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
