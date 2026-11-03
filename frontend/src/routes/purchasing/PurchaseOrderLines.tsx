import { Box, HStack, Heading, Stack, Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import { manufacturerLabel } from "../../components/ManufacturerSelect";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { ManufacturerRef } from "../../gen/inventory_iface/v1/manufacturer_pb";
import type { ProductRef } from "../../gen/inventory_iface/v1/product_pb";
import type { PurchaseOrder } from "../../gen/purchasing_iface/v1/order_pb";
import { formatMoney } from "../../lib/format";
import { fmtUnitQty, netUnitCostFrom } from "../../lib/purchaseLine";

// What was ordered, and what it adds up to — one card, because the totals are
// the sum of the rows above them and reading either without the other invites
// the arithmetic questions this layout exists to answer.
//
// Page-local to PurchaseOrderDetail, like its PurchaseOrderReceipts /
// PurchaseOrderReturns siblings: it renders what it is handed and owns no query.
export default function PurchaseOrderLines({
  po,
  ppnRate,
  productRefs,
  manufacturerRefs,
}: {
  po: PurchaseOrder;
  /** 0 when PPN is off — the derived column then drops "+ PPN" from its header. */
  ppnRate: number;
  productRefs: Map<string, ProductRef>;
  manufacturerRefs: Map<string, ManufacturerRef>;
}) {
  const { t } = useTranslation();
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
      <Heading size="sm" mb={3}>
        {t("purchasing.items")}
      </Heading>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header>
            <Table.Row>
              <Table.ColumnHeader>{t("purchasing.selectProduct")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.products.manufacturer")}</Table.ColumnHeader>
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
                {/* Who MADE it, as the buyer recorded it off the invoice —
                    the value CreateReceipt stamps onto the lot. A dash is a
                    line that named none, which is what every order placed
                    before the field existed looks like. */}
                <Table.Cell color="fg.muted">
                  {manufacturerLabel(manufacturerRefs.get(it.manufacturerId)) ?? "—"}
                </Table.Cell>
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
  );
}
