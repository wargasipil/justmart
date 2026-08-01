import {
  Badge,
  Box,
  Button,
  Dialog,
  Flex,
  Grid,
  HStack,
  Heading,
  IconButton,
  Input,
  Portal,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Ban, DollarSign, PackageCheck, Send, Undo2, X } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useCrumbLabel } from "../../lib/breadcrumbs";
import BackButton from "../../components/BackButton";
import Pagination from "../../components/Pagination";
import { usePageState } from "../../lib/pagination";
import DatePickerField from "../../components/DatePicker";
import MoneyInput from "../../components/MoneyInput";
import NumberInput from "../../components/NumberInput";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import {
  POStatus,
  type PurchaseOrder,
  type PurchaseOrderItem,
} from "../../gen/purchasing_iface/v1/order_pb";
import { formatDate, formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseReceipt } from "../../gen/purchasing_iface/v1/receipt_pb";
import { useProductRefs, useSupplierRefs } from "../../queries/refs";
import {
  useCreatePurchaseReturnMutation,
  useCreateReceiptMutation,
  usePayPurchaseMutation,
  usePurchaseOrderQuery,
  usePurchaseReturnsQuery,
  useReceiptsQuery,
  useSendPurchaseOrderMutation,
  useVoidPurchaseOrderMutation,
} from "../../queries/purchasing";

const STATUS_PALETTE: Record<POStatus, string> = {
  [POStatus.PO_STATUS_UNSPECIFIED]: "gray",
  [POStatus.PO_STATUS_DRAFT]: "gray",
  [POStatus.PO_STATUS_SENT]: "blue",
  [POStatus.PO_STATUS_PARTIALLY_RECEIVED]: "orange",
  [POStatus.PO_STATUS_RECEIVED]: "green",
  [POStatus.PO_STATUS_CLOSED]: "green",
  [POStatus.PO_STATUS_VOIDED]: "red",
};

export default function PurchaseOrderDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();

  const poQ = usePurchaseOrderQuery(id);
  useCrumbLabel(poQ.data?.poNo);
  const receiptsPage = usePageState(`receipts:${id}`);
  const returnsPage = usePageState(`returns:${id}`);
  const receiptsQ = useReceiptsQuery(
    { purchaseOrderId: id },
    { page: receiptsPage.page, pageSize: receiptsPage.pageSize },
  );
  const returnsQ = usePurchaseReturnsQuery(id, {
    page: returnsPage.page,
    pageSize: returnsPage.pageSize,
  });

  const sendMut = useSendPurchaseOrderMutation();
  const voidMut = useVoidPurchaseOrderMutation();

  const [receiveOpen, setReceiveOpen] = useState(false);
  const [payOpen, setPayOpen] = useState(false);
  const [returnOpen, setReturnOpen] = useState(false);

  // Resolve names for just this PO's supplier + line/receipt products
  // (resolve-by-IDs; hooks run unconditionally, before the loading early-return).
  const po = poQ.data;
  const supplierIds = useMemo(() => (po ? [po.supplierId] : []), [po]);
  const productIds = useMemo(() => {
    const ids = new Set<string>();
    po?.items.forEach((it) => ids.add(it.productId));
    receiptsQ.rows.forEach((r) => r.items.forEach((it) => ids.add(it.productId)));
    return Array.from(ids);
  }, [po, receiptsQ.rows]);
  const supplierRefs = useSupplierRefs(supplierIds);
  const productRefs = useProductRefs(productIds);

  if (poQ.isLoading || !po) {
    return (
      <Box p={6} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  const canSend = po.status === POStatus.PO_STATUS_DRAFT && po.items.length > 0;
  const canVoid = po.status === POStatus.PO_STATUS_DRAFT || po.status === POStatus.PO_STATUS_SENT;
  const canReceive =
    po.status === POStatus.PO_STATUS_SENT || po.status === POStatus.PO_STATUS_PARTIALLY_RECEIVED;
  const canPay =
    po.status !== POStatus.PO_STATUS_VOIDED &&
    po.status !== POStatus.PO_STATUS_DRAFT &&
    po.outstanding > 0n;
  // Returnable: a PO with received goods still in stock (a receipt line whose
  // batch on-hand > 0). VOIDED/DRAFT/SENT have nothing to return.
  const canReturn =
    (po.status === POStatus.PO_STATUS_PARTIALLY_RECEIVED ||
      po.status === POStatus.PO_STATUS_RECEIVED ||
      po.status === POStatus.PO_STATUS_CLOSED) &&
    receiptsQ.rows.some((r) => r.items.some((it) => it.returnableQty > 0n));

  const onSend = async () => {
    try {
      await sendMut.mutateAsync(id);
      toast.success(t("purchasing.actions.send") + " ✓");
    } catch {
      /* */
    }
  };
  const onVoid = async () => {
    try {
      await voidMut.mutateAsync(id);
      toast.success(t("purchasing.actions.void") + " ✓");
    } catch {
      /* */
    }
  };

  return (
    <Stack gap={6}>
      <BackButton to="/purchasing" />
      <Flex justify="space-between" align="center" wrap="wrap" gap={2}>
        <HStack gap={3}>
          <Heading size="md" fontFamily="mono">
            {po.poNo || po.id.slice(0, 8)}
          </Heading>
          <Badge colorPalette={STATUS_PALETTE[po.status]}>
            {t(`purchasing.states.${statusKey(po.status)}`)}
          </Badge>
        </HStack>
        <HStack gap={2}>
          {canSend && (
            <Button size="sm" colorPalette="blue" onClick={onSend} loading={sendMut.isPending}>
              <Send size={14} />
              {t("purchasing.actions.send")}
            </Button>
          )}
          {canReceive && (
            <Button size="sm" colorPalette="blue" onClick={() => setReceiveOpen(true)}>
              <PackageCheck size={14} />
              {t("purchasing.actions.receive")}
            </Button>
          )}
          {canPay && (
            <Button size="sm" variant="outline" onClick={() => setPayOpen(true)}>
              <DollarSign size={14} />
              {t("purchasing.actions.pay")}
            </Button>
          )}
          {canReturn && (
            <Button size="sm" variant="outline" colorPalette="orange" onClick={() => setReturnOpen(true)}>
              <Undo2 size={14} />
              {t("purchasing.actions.return")}
            </Button>
          )}
          {canVoid && (
            <Button size="sm" variant="outline" colorPalette="red" onClick={onVoid}>
              <Ban size={14} />
              {t("purchasing.actions.void")}
            </Button>
          )}
        </HStack>
      </Flex>

      {/* Header info */}
      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
        <Grid templateColumns={{ base: "1fr", md: "repeat(4, 1fr)" }} gap={4}>
          <Info label={t("purchasing.supplier")} value={supplierRefs.get(po.supplierId)?.name ?? "—"} />
          <Info label={t("purchasing.warehouse")} value={po.warehouseName || "—"} />
          <Info label={t("purchasing.invoiceNo")} value={po.invoiceNo || "—"} />
          <Info label={t("purchasing.invoiceDate")} value={po.invoiceDate ? formatDate(po.invoiceDate) : "—"} />
          <Info label={t("purchasing.dueAt")} value={po.dueAt ? formatDate(po.dueAt) : "—"} />
          <Info
            label={t("purchasing.totalOrdered")}
            value={formatMoney(Number(po.orderedTotal))}
          />
          {po.returnedAmount > 0n && (
            <Info
              label={t("purchasing.return.refundAmount")}
              value={formatMoney(Number(po.returnedAmount))}
            />
          )}
          <Info
            label={po.outstanding < 0n ? t("purchasing.return.credit") : t("purchasing.outstanding")}
            value={formatMoney(Number(po.outstanding))}
            highlight={po.outstanding > 0n}
          />
        </Grid>
        {po.note && (
          <Box mt={3} pt={3} borderTopWidth="1px">
            <Text fontSize="xs" color="fg.muted">
              {t("purchasing.note")}
            </Text>
            <Text fontSize="sm">{po.note}</Text>
          </Box>
        )}
      </Box>

      {/* Lines */}
      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
        <Heading size="sm" mb={3}>
          {t("purchasing.items")}
        </Heading>
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Table.Root size="sm" stickyHeader>
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>{t("purchasing.selectProduct")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.ordered")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.received")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.unitCost")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.lineDiscount")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.lineTotal")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {po.items.map((it) => (
                <Table.Row key={it.id}>
                  <Table.Cell>{productRefs.get(it.productId)?.name ?? "—"}</Table.Cell>
                  <Table.Cell>{fmtUnitQty(it.orderedQty, it.unitName, it.unitFactor)}</Table.Cell>
                  <Table.Cell>
                    {fmtUnitQty(it.receivedQty, it.unitName, it.unitFactor)} /{" "}
                    {fmtUnitQty(it.orderedQty, it.unitName, it.unitFactor)}
                  </Table.Cell>
                  <Table.Cell fontFamily="mono">{formatMoney(Number(it.unitCostPrice))}</Table.Cell>
                  <Table.Cell fontFamily="mono" color="fg.muted">
                    {it.discountValue > 0n
                      ? (it.discountType === "PERCENT"
                          ? `${Number(it.discountValue) / 100}%`
                          : `−${formatMoney(Number(it.discountValue))}`) +
                        (it.discountPerItem ? ` ${t("purchasing.perItemSuffix")}` : "")
                      : "—"}
                  </Table.Cell>
                  <Table.Cell fontFamily="mono">{formatMoney(Number(it.subtotal))}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </TableScroll>
        <Box mt={4} pt={4} borderTopWidth="1px" display="flex" justifyContent="flex-end">
          <Stack gap={1} maxW="320px" w="full">
            <HStack justify="space-between">
              <Text color="fg.muted">{t("purchasing.subtotal")}</Text>
              <Text fontFamily="mono">{formatMoney(Number(po.subtotal))}</Text>
            </HStack>
            {po.cartDiscount > 0n && (
              <HStack justify="space-between">
                <Text color="fg.muted">{t("purchasing.cartDiscount")}</Text>
                <Text fontFamily="mono">−{formatMoney(Number(po.cartDiscount))}</Text>
              </HStack>
            )}
            {po.ppnEnabled && (
              <HStack justify="space-between">
                <Text color="fg.muted">
                  {t("purchasing.ppn")} {po.ppnRate || 11}%
                </Text>
                <Text fontFamily="mono">+{formatMoney(Number(po.ppnAmount))}</Text>
              </HStack>
            )}
            <HStack justify="space-between" pt={2} borderTopWidth="1px">
              <Text fontWeight="bold">{t("purchasing.total")}</Text>
              <Text fontWeight="bold" fontFamily="mono">
                {formatMoney(Number(po.orderedTotal))}
              </Text>
            </HStack>
          </Stack>
        </Box>
      </Box>

      {/* Receipts */}
      <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
        <Heading size="sm" mb={3}>
          {t("purchasing.receipt")}s
        </Heading>
        {receiptsQ.isLoading ? (
          <Spinner size="sm" />
        ) : receiptsQ.rows.length === 0 ? (
          <Text fontSize="sm" color="fg.muted">
            {t("purchasing.noReceipts")}
          </Text>
        ) : (
          <Stack gap={3}>
            {receiptsQ.rows.map((r) => (
              <Box key={r.id} borderWidth="1px" borderRadius="md" p={3}>
                <HStack justify="space-between" mb={2}>
                  <HStack gap={3}>
                    <Text fontFamily="mono" fontWeight="medium">
                      {r.receiptNo}
                    </Text>
                    {r.invoiceNo && (
                      <Text fontSize="sm" color="fg.muted">
                        {t("purchasing.invoiceNo")}: {r.invoiceNo}
                      </Text>
                    )}
                  </HStack>
                  <Text fontSize="sm" color="fg.muted">
                    {formatDate(r.receivedAt)}
                  </Text>
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
            ))}
          </Stack>
        )}
        <Pagination
          page={receiptsPage.page}
          pageSize={receiptsPage.pageSize}
          total={receiptsQ.total}
          onPageChange={receiptsPage.setPage}
          onPageSizeChange={receiptsPage.setPageSize}
        />
      </Box>

      {/* Returns */}
      {returnsQ.rows.length > 0 && (
        <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
          <Heading size="sm" mb={3}>
            {t("purchasing.return.section")}
          </Heading>
          <Stack gap={3}>
            {returnsQ.rows.map((r) => (
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
            page={returnsPage.page}
            pageSize={returnsPage.pageSize}
            total={returnsQ.total}
            onPageChange={returnsPage.setPage}
            onPageSizeChange={returnsPage.setPageSize}
          />
        </Box>
      )}

      <ReceiveDialog
        open={receiveOpen}
        onClose={() => setReceiveOpen(false)}
        po={po}
        productRefs={productRefs}
      />
      <ReturnDialog
        open={returnOpen}
        onClose={() => setReturnOpen(false)}
        poId={po.id}
        receipts={receiptsQ.rows}
        productRefs={productRefs}
      />
      <PayDialog open={payOpen} onClose={() => setPayOpen(false)} poId={po.id} outstanding={Number(po.outstanding)} />

      <Button variant="ghost" alignSelf="flex-start" onClick={() => navigate("/purchasing/all")}>
        ← {t("purchasing.title")}
      </Button>
    </Stack>
  );
}

function Info({
  label,
  value,
  highlight,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted">
        {label}
      </Text>
      <Text fontSize="md" fontWeight="medium" color={highlight ? "fg.error" : "fg"} fontFamily="mono">
        {value}
      </Text>
    </Box>
  );
}

function ReceiveDialog({
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

  type LineDraft = {
    purchaseOrderItemId: string;
    qty: number; // in the PO line's purchasable unit
    unitCostPrice: number; // per BASE unit (override; default = PO line's)
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
          unitCostPrice: Number(it.unitCostPrice),
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
            unitCostPrice: BigInt(l.unitCostPrice),
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
                        <Table.ColumnHeader>{t("purchasing.unitCost")}</Table.ColumnHeader>
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
                              <MoneyInput
                                size="sm"
                                width="120px"
                                value={l.unitCostPrice}
                                onChange={(raw) =>
                                  updateLine(idx, { unitCostPrice: Number(raw || 0) })
                                }
                              />
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

function ReturnDialog({
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

function PayDialog({
  open,
  onClose,
  poId,
  outstanding,
}: {
  open: boolean;
  onClose: () => void;
  poId: string;
  outstanding: number;
}) {
  const { t } = useTranslation();
  const payMut = usePayPurchaseMutation();
  const [amount, setAmount] = useState(String(outstanding));
  const [note, setNote] = useState("");

  const submit = async () => {
    const n = parseInt(amount, 10);
    if (!n || n <= 0) return;
    try {
      await payMut.mutateAsync({
        purchaseOrderId: poId,
        amount: BigInt(n),
        note,
      });
      toast.success(t("purchasing.actions.pay") + " ✓");
      onClose();
    } catch {
      /* */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("purchasing.payTitle")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Box>
                  <Text fontSize="xs" color="fg.muted">
                    {t("purchasing.outstanding")}
                  </Text>
                  <Text fontSize="md" fontFamily="mono">
                    {formatMoney(outstanding)}
                  </Text>
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("purchasing.amount")}
                  </Text>
                  <MoneyInput value={amount} onChange={setAmount} />
                </Box>
                <Box>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("purchasing.note")}
                  </Text>
                  <Input value={note} onChange={(e) => setNote(e.target.value)} />
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" onClick={submit} loading={payMut.isPending}>
                  {t("purchasing.actions.pay")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

// fmtUnitQty renders a BASE-unit quantity in its purchasable unit, e.g.
// (500, "box", 100n) -> "5 box". Falls back to the bare number when no unit.
function fmtUnitQty(qty: number, unitName: string, factor: bigint): string {
  const f = Number(factor) || 1;
  const q = f > 1 ? qty / f : qty;
  return unitName ? `${q} ${unitName}` : String(q);
}

function statusKey(s: POStatus): string {
  switch (s) {
    case POStatus.PO_STATUS_DRAFT:
      return "draft";
    case POStatus.PO_STATUS_SENT:
      return "sent";
    case POStatus.PO_STATUS_PARTIALLY_RECEIVED:
      return "partiallyReceived";
    case POStatus.PO_STATUS_RECEIVED:
      return "received";
    case POStatus.PO_STATUS_CLOSED:
      return "closed";
    case POStatus.PO_STATUS_VOIDED:
      return "voided";
    default:
      return "draft";
  }
}

