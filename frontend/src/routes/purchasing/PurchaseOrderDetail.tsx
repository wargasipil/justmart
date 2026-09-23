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
  Text,
} from "@chakra-ui/react";
import { Ban, DollarSign, PackageCheck, Send, Undo2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useCrumbLabel } from "../../lib/breadcrumbs";
import BackButton from "../../components/BackButton";
import { usePageState } from "../../lib/pagination";
import { POStatus } from "../../gen/purchasing_iface/v1/order_pb";
import { formatDate, formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useManufacturerRefs, useProductRefs, useSupplierRefs } from "../../queries/refs";
import {
  usePurchaseOrderQuery,
  usePurchaseReturnsQuery,
  useReceiptsQuery,
  useSendPurchaseOrderMutation,
  useVoidPurchaseOrderMutation,
} from "../../queries/purchasing";
import { PayDialog } from "./PayDialog";
import PurchaseOrderLines from "./PurchaseOrderLines";
import PurchaseOrderReceipts from "./PurchaseOrderReceipts";
import PurchaseOrderReturns from "./PurchaseOrderReturns";
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
  // Only the ORDERED lines name a pabrik. A delivery cannot disagree with the
  // line it fulfils — CreateReceipt stamps the lot from that same field, and
  // lines freeze once the order leaves DRAFT — so resolving the receipt items
  // too would cost a second set of ids to print the same names twice.
  const manufacturerIds = useMemo(
    () => Array.from(new Set(po?.items.map((it) => it.manufacturerId).filter(Boolean) ?? [])),
    [po],
  );
  const supplierRefs = useSupplierRefs(supplierIds);
  const productRefs = useProductRefs(productIds);
  const manufacturerRefs = useManufacturerRefs(manufacturerIds);

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

      <PurchaseOrderLines
        po={po}
        ppnRate={ppnRate}
        productRefs={productRefs}
        manufacturerRefs={manufacturerRefs}
      />
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

      <PurchaseOrderReturns
        returns={returnsQ.rows}
        total={returnsQ.total}
        page={returnsPage.page}
        pageSize={returnsPage.pageSize}
        isPlaceholderData={returnsQ.isPlaceholderData}
        onPageChange={returnsPage.setPage}
        onPageSizeChange={returnsPage.setPageSize}
        productRefs={productRefs}
      />

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

