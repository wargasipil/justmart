import { Badge, Box, Button, Grid, HStack, Heading, Input, SimpleGrid, Spinner, Stack, Switch, Table, Text } from "@chakra-ui/react";
import { useMemo, useState } from "react";
import { Printer, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import { useCrumbLabel } from "../lib/breadcrumbs";
import BackButton from "../components/BackButton";
import ConfirmDialog from "../components/ConfirmDialog";
import PageHeader from "../components/PageHeader";
import { Role } from "../gen/auth_iface/v1/policy_pb";
import { SaleStatus } from "../gen/pos_iface/v1/sale_pb";
import { useAuth } from "../lib/auth";
import { formatMoney, formatUnix } from "../lib/format";
import { savedPrinterTarget } from "../lib/printerTarget";
import { toast } from "../lib/toaster";
import { useCustomerRefs, useProductRefs, useUserRefs } from "../queries/refs";
import { usePrintReceiptMutation, useRefundSaleMutation, useSaleQuery } from "../queries/sales";

const PAYMENT_KEY: Record<number, string> = {
  0: "unspecified",
  1: "cash",
  2: "nonCash",
};

const STATUS_BADGE: Record<number, string> = {
  [SaleStatus.UNSPECIFIED]: "gray",
  [SaleStatus.DRAFT]: "gray",
  [SaleStatus.COMPLETED]: "green",
  [SaleStatus.VOIDED]: "red",
  [SaleStatus.REFUNDED]: "orange",
};

function statusKey(s: SaleStatus): string {
  switch (s) {
    case SaleStatus.DRAFT:
      return "draft";
    case SaleStatus.COMPLETED:
      return "completed";
    case SaleStatus.VOIDED:
      return "voided";
    case SaleStatus.REFUNDED:
      return "refunded";
    default:
      return "unspecified";
  }
}

export default function OrderDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const { user } = useAuth();
  const saleQ = useSaleQuery(id);
  useCrumbLabel(saleQ.data ? saleQ.data.saleNo || saleQ.data.id.slice(0, 8) : undefined);
  const refund = useRefundSaleMutation();
  const printMut = usePrintReceiptMutation();

  const [refundOpen, setRefundOpen] = useState(false);
  const [refundReason, setRefundReason] = useState("");
  const [refundRestock, setRefundRestock] = useState(true);

  const sale = saleQ.data;
  const customerIds = useMemo(() => (sale?.customerId ? [sale.customerId] : []), [sale]);
  const cashierIds = useMemo(() => (sale?.cashierUserId ? [sale.cashierUserId] : []), [sale]);
  const productIds = useMemo(
    () => (sale?.items ?? []).map((i) => i.productId).filter(Boolean),
    [sale],
  );
  const customerRefs = useCustomerRefs(customerIds);
  const cashierRefs = useUserRefs(cashierIds);
  const productRefs = useProductRefs(productIds);

  if (saleQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  if (!sale) {
    return (
      <Box>
        <BackButton to="/orders" />
        <Box p={8}>
          <Text color="fg.muted">{t("common.noResults")}</Text>
        </Box>
      </Box>
    );
  }

  const saleNo = sale.saleNo || sale.id.slice(0, 8);
  const customer = sale.customerId ? customerRefs.get(sale.customerId)?.name ?? "—" : "—";
  const createdBy =
    cashierRefs.get(sale.cashierUserId)?.name ||
    cashierRefs.get(sale.cashierUserId)?.email ||
    "—";
  const paymentLabel = t(`orders.payments.${PAYMENT_KEY[sale.paymentSource] ?? "unspecified"}`);
  const change = Number(sale.paidAmount) - Number(sale.total);

  // Refunds are time-boxed to 1 day after completion (mirrors the backend guard).
  const withinRefundWindow =
    sale.completedAt > 0n && Date.now() / 1000 - Number(sale.completedAt) <= 86400;
  const canRefund =
    sale.status === SaleStatus.COMPLETED &&
    withinRefundWindow &&
    (user?.role === Role.OWNER || user?.role === Role.PHARMACIST);

  const onConfirmRefund = async () => {
    try {
      await refund.mutateAsync({ saleId: sale.id, reason: refundReason.trim(), restock: refundRestock });
      toast.success(t("orders.refund.success"));
      setRefundOpen(false);
      setRefundReason("");
    } catch {
      /* toast handled globally */
    }
  };

  // Reprint a completed order's receipt, reusing the printer POS already targets
  // (empty target → server resolves the saved default / sole connector / TCP).
  const onPrint = async () => {
    const target = savedPrinterTarget();
    try {
      await printMut.mutateAsync({
        saleId: sale.id,
        connectorDeviceId: target.deviceId,
        printerName: target.printerName,
      });
      toast.success(t("pos.printSent"));
    } catch {
      /* error surfaced by the global mutation toast (printing disabled / Unavailable) */
    }
  };

  return (
    <Box>
      <BackButton to="/orders" />
      <PageHeader
        title={saleNo}
        description={t("orders.detail.description")}
        actions={
          <HStack gap={3}>
            {sale.status === SaleStatus.COMPLETED && (
              <Button size="sm" variant="outline" onClick={onPrint} loading={printMut.isPending}>
                <Printer size={14} />
                {t("orders.printReceipt")}
              </Button>
            )}
            {canRefund && (
              <Button size="sm" variant="outline" colorPalette="orange" onClick={() => setRefundOpen(true)}>
                <RotateCcw size={14} />
                {t("orders.refund.button")}
              </Button>
            )}
            <Badge colorPalette={STATUS_BADGE[sale.status] ?? "gray"} size="lg">
              {t(`orders.states.${statusKey(sale.status)}`)}
            </Badge>
          </HStack>
        }
      />

      <Stack gap={6}>
        <Section title={t("orders.detail.info")}>
          <SimpleGrid columns={{ base: 1, md: 2, lg: 3 }} gap={4}>
            <Field label={t("orders.date")} value={formatUnix(sale.createdAt)} />
            <Field label={t("orders.createdBy")} value={createdBy} />
            <Field label={t("orders.customer")} value={customer} />
            <Field label={t("orders.payment")} value={paymentLabel} />
            {sale.completedAt > 0n && (
              <Field
                label={t("orders.detail.completedAt")}
                value={formatUnix(sale.completedAt)}
              />
            )}
          </SimpleGrid>
        </Section>

        <Section title={t("orders.detail.totals")}>
          <Grid templateColumns={{ base: "1fr 1fr", md: "repeat(6, 1fr)" }} gap={3}>
            <MoneyTile label={t("orders.detail.subtotal")} value={Number(sale.subtotal)} />
            <MoneyTile
              label={
                t("orders.detail.cartDiscount") +
                (sale.cartDiscountType === "PERCENT"
                  ? ` (${Number(sale.cartDiscountValue) / 100}%)`
                  : "")
              }
              value={Number(sale.cartDiscount)}
            />
            {Number(sale.biayaJasa) > 0 && (
              <MoneyTile label={t("prescriptions.biayaJasa")} value={Number(sale.biayaJasa)} />
            )}
            <MoneyTile label={t("orders.detail.total")} value={Number(sale.total)} accent />
            <MoneyTile label={t("orders.detail.paid")} value={Number(sale.paidAmount)} />
            <MoneyTile label={t("orders.detail.change")} value={change} />
          </Grid>
        </Section>

        {sale.status === SaleStatus.REFUNDED && (
          <Section title={t("orders.refund.infoTitle")}>
            <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} gap={4}>
              <Field label={t("orders.refund.refundedAt")} value={sale.refundedAt > 0n ? formatUnix(sale.refundedAt) : "—"} />
              <Box>
                <Text fontSize="xs" color="fg.muted" mb={1}>
                  {t("orders.refund.amount")}
                </Text>
                <Text fontFamily="mono">{formatMoney(Number(sale.refundAmount))}</Text>
              </Box>
              <Box>
                <Text fontSize="xs" color="fg.muted" mb={1}>
                  {t("orders.refund.restockLabel")}
                </Text>
                <Badge colorPalette={sale.refundRestocked ? "green" : "gray"}>
                  {sale.refundRestocked ? t("orders.refund.restockedYes") : t("orders.refund.restockedNo")}
                </Badge>
              </Box>
              <Field label={t("orders.refund.reasonLabel")} value={sale.refundReason || "—"} />
            </SimpleGrid>
          </Section>
        )}

        <Section title={t("orders.detail.items")}>
          <Box overflowX="auto">
            <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("orders.detail.product")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("orders.detail.qty")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("orders.detail.unitPrice")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("orders.detail.lineDiscount")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("orders.detail.lineTotal")}</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {sale.items.map((it) => {
                  const name = productRefs.get(it.productId)?.name || it.productName || "—";
                  return (
                    <Table.Row key={it.id}>
                      <Table.Cell>{name}</Table.Cell>
                      <Table.Cell>
                        {String(it.qty)}{it.unitName ? ` ${it.unitName}` : ""}
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        <Stack gap={0} align="flex-end">
                          <Text fontFamily="mono">{formatMoney(Number(it.unitPriceSnapshot))}</Text>
                          {/* Grosir: history must explain the price the customer
                              actually paid, same as POS showed at checkout. */}
                          {it.tierMinQty > 0 && (
                            <HStack gap={1}>
                              <Badge size="xs" colorPalette="purple">
                                {t("pos.grosir")}
                              </Badge>
                              <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
                                {t("pos.grosirThreshold", { qty: it.tierMinQty })}
                              </Text>
                            </HStack>
                          )}
                        </Stack>
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(Number(it.lineDiscount))}
                        {it.discountType === "PERCENT" && Number(it.discountValue) > 0 && (
                          <Text as="span" fontSize="xs" color="fg.muted">
                            {" "}({Number(it.discountValue) / 100}%)
                          </Text>
                        )}
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(Number(it.lineTotal))}
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
                {sale.items.length === 0 && (
                  <Table.Row>
                    <Table.Cell colSpan={5}>
                      <Text color="fg.muted" textAlign="center" py={4}>
                        {t("common.noResults")}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                )}
              </Table.Body>
            </Table.Root>
          </Box>
        </Section>
      </Stack>

      <ConfirmDialog
        open={refundOpen}
        title={t("orders.refund.dialogTitle")}
        confirmLabel={t("orders.refund.confirm")}
        confirmColorPalette="orange"
        loading={refund.isPending}
        onConfirm={onConfirmRefund}
        onCancel={() => setRefundOpen(false)}
        body={
          <Stack gap={4}>
            <Text fontSize="sm">{t("orders.refund.body", { total: formatMoney(Number(sale.total)) })}</Text>
            <Stack gap={1}>
              <Text fontSize="sm" fontWeight="medium">
                {t("orders.refund.reasonLabel")}
              </Text>
              <Input
                size="sm"
                value={refundReason}
                onChange={(e) => setRefundReason(e.target.value)}
                placeholder={t("orders.refund.reasonPlaceholder")}
              />
            </Stack>
            <Switch.Root checked={refundRestock} onCheckedChange={(d) => setRefundRestock(d.checked)}>
              <Switch.HiddenInput />
              <Switch.Control />
              <Switch.Label>{t("orders.refund.restockSwitch")}</Switch.Label>
            </Switch.Root>
          </Stack>
        }
      />
    </Box>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Box>
      <Heading size="sm" mb={3}>
        {title}
      </Heading>
      {children}
    </Box>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily={mono ? "mono" : undefined}>{value}</Text>
    </Box>
  );
}

function MoneyTile({ label, value, accent }: { label: string; value: number; accent?: boolean }) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderColor={accent ? "blue.300" : "border"} borderRadius="lg" p={3}>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily="mono" fontSize="md" fontWeight="semibold" color={accent ? "blue.600" : "fg"}>
        {formatMoney(value)}
      </Text>
    </Box>
  );
}
