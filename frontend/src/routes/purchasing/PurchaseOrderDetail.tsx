import {
  Badge,
  Box,
  Button,
  Flex,
  Grid,
  HStack,
  Heading,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Ban, DollarSign, PackageCheck, Send, Undo2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useCrumbLabel } from "../../lib/breadcrumbs";
import BackButton from "../../components/BackButton";
import Pagination from "../../components/Pagination";
import { usePageState } from "../../lib/pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { POStatus } from "../../gen/purchasing_iface/v1/order_pb";
import { formatDate, formatMoney } from "../../lib/format";
import { fmtUnitQty, netUnitCostFrom } from "../../lib/purchaseLine";
import { toast } from "../../lib/toaster";
import { useProductRefs, useSupplierRefs } from "../../queries/refs";
import {
  usePurchaseOrderQuery,
  usePurchaseReturnsQuery,
  useReceiptsQuery,
  useSendPurchaseOrderMutation,
  useVoidPurchaseOrderMutation,
} from "../../queries/purchasing";
import { PayDialog } from "./PayDialog";
import PurchaseOrderReceipts from "./PurchaseOrderReceipts";
import { ReceiveDialog } from "./ReceiveDialog";
import { ReturnDialog } from "./ReturnDialog";

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

  // 0 when PPN is off — the derived per-base cost column then shows the plain
  // net cost and drops the "+ PPN" from its header.
  const ppnRate = po.ppnEnabled ? po.ppnRate || 11 : 0;

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
                <Table.ColumnHeader>
                  {ppnRate > 0
                    ? t("purchasing.unitCostDerivedPpn", { rate: ppnRate })
                    : t("purchasing.unitCostDerived")}
                </Table.ColumnHeader>
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
                  {/* Derived, not part of the invoice arithmetic: the per-base
                      cost net of this line's discount and inclusive of PPN —
                      i.e. what the batch's cost_price becomes on receive. Kept
                      separate so Unit cost x Ordered − Line discount = Line
                      total still holds and line totals still sum to the
                      PPN-exclusive Subtotal in the card below. */}
                  <Table.Cell fontFamily="mono" color="fg.muted">
                    {formatMoney(
                      netUnitCostFrom(Number(it.subtotal), Number(it.orderedQty), ppnRate),
                    )}
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

      <PurchaseOrderReceipts
        receipts={receiptsQ.rows}
        isLoading={receiptsQ.isLoading}
        total={receiptsQ.total}
        page={receiptsPage.page}
        pageSize={receiptsPage.pageSize}
        onPageChange={receiptsPage.setPage}
        onPageSizeChange={receiptsPage.setPageSize}
        productRefs={productRefs}
      />

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

