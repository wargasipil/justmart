import { HStack, IconButton, Input, Stack, Table, Text } from "@chakra-ui/react";
import { AlertTriangle, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import EnumSelect from "../../components/EnumSelect";
import MoneyInput from "../../components/MoneyInput";
import NumberInput from "../../components/NumberInput";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { PriceAgreement } from "../../gen/inventory_iface/v1/price_agreement_pb";
import { formatMoney } from "../../lib/format";
import {
  DISCOUNT_MODES,
  type DiscountMode,
  type Line,
  factorOf,
  lineNet,
  modeOf,
  modeToParts,
  netUnitCostOf,
  unitNameOf,
} from "../../lib/purchaseLine";

export type PurchaseLinesTableProps = {
  lines: Line[];
  onChange: (idx: number, patch: Partial<Line>) => void;
  onRemove: (idx: number) => void;
  /** Active agreement for a line's product+unit, if any — reference only. */
  agreementFor: (l: Line) => PriceAgreement | undefined;
  /** True when the entered cost is above the agreed price (warns, never blocks). */
  isAboveAgreement: (l: Line) => boolean;
};

// The editable line table of the restock (purchase order) form. Page-local: it
// is coupled to this form's Line shape and its price-agreement warnings, so it
// lives next to the route rather than in components/.
//
// Products are added/removed through <ProductPickerDialog> on the parent — the
// product cell here is a read-only label, and the row owns only unit / qty /
// cost / discount.
export default function PurchaseLinesTable({
  lines,
  onChange,
  onRemove,
  agreementFor,
  isAboveAgreement,
}: PurchaseLinesTableProps) {
  const { t } = useTranslation();
  return (
    <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
      <Table.Root size="sm" stickyHeader>
        <Table.Header>
          <Table.Row>
            <Table.ColumnHeader minW="240px">{t("purchasing.selectProduct")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("purchasing.unit")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("purchasing.qty")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("purchasing.costPerItemInput")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("purchasing.lineDiscount")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("purchasing.unitCostDerived")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("purchasing.subtotal")}</Table.ColumnHeader>
            <Table.ColumnHeader />
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {lines.length === 0 && (
            <Table.Row>
              <Table.Cell colSpan={8} textAlign="center" color="fg.muted" py={6}>
                {t("purchasing.noLines")}
              </Table.Cell>
            </Table.Row>
          )}
          {lines.map((l, idx) => {
            const agreement = agreementFor(l);
            const above = isAboveAgreement(l);
            return (
              <Table.Row key={idx} bg={above ? "red.subtle" : undefined}>
                <Table.Cell>
                  <Text fontSize="sm">{l.productName}</Text>
                  <Text fontSize="xs" color="fg.muted">
                    {l.productSku}
                  </Text>
                  {agreement && (
                    <Stack gap={0.5} mt={1}>
                      <Text fontSize="xs" color="fg.muted">
                        {t("purchasing.agreedPrice", {
                          price: formatMoney(Number(agreement.price)),
                        })}
                      </Text>
                      {above && (
                        <HStack gap={1} color="red.500">
                          <AlertTriangle size={12} />
                          <Text fontSize="xs">{t("purchasing.aboveAgreement")}</Text>
                        </HStack>
                      )}
                    </Stack>
                  )}
                </Table.Cell>
                <Table.Cell>
                  {l.units.length > 1 ? (
                    <EnumSelect
                      size="sm"
                      width="110px"
                      value={l.productUnitId}
                      onChange={(v) => onChange(idx, { productUnitId: v })}
                      items={l.units}
                      itemToString={(u) => u.name}
                      itemToValue={(u) => u.id}
                    />
                  ) : (
                    <Text fontSize="sm" color="fg.muted">
                      {unitNameOf(l) || "—"}
                    </Text>
                  )}
                </Table.Cell>
                <Table.Cell>
                  <NumberInput
                    size="sm"
                    width="80px"
                    value={l.orderedQty}
                    onChange={(raw) => onChange(idx, { orderedQty: Number(raw || 0) })}
                  />
                </Table.Cell>
                <Table.Cell>
                  <MoneyInput
                    size="sm"
                    width="140px"
                    value={l.costPerItem}
                    onChange={(raw) => onChange(idx, { costPerItem: Number(raw || 0) })}
                  />
                </Table.Cell>
                <Table.Cell>
                  <HStack gap={1}>
                    <EnumSelect
                      size="sm"
                      width="150px"
                      value={modeOf(l)}
                      onChange={(v) =>
                        onChange(idx, { ...modeToParts(v as DiscountMode), discountValue: 0 })
                      }
                      items={DISCOUNT_MODES}
                      itemToString={(m) =>
                        t(
                          m === "FIXED"
                            ? "purchasing.fixed"
                            : m === "PERCENT"
                              ? "purchasing.percent"
                              : m === "FIXED_ITEM"
                                ? "purchasing.fixedPerItem"
                                : "purchasing.percentPerItem",
                        )
                      }
                      itemToValue={(m) => m}
                    />
                    {l.discountType === "PERCENT" ? (
                      <Input
                        size="sm"
                        type="number"
                        step="0.01"
                        min={0}
                        max={100}
                        width="74px"
                        value={l.discountValue || ""}
                        onChange={(e) =>
                          onChange(idx, {
                            discountValue: Math.min(100, Math.max(0, Number(e.target.value) || 0)),
                          })
                        }
                        aria-label={t("purchasing.lineDiscount")}
                      />
                    ) : (
                      <MoneyInput
                        size="sm"
                        width="110px"
                        value={l.discountValue}
                        onChange={(raw) => onChange(idx, { discountValue: Number(raw || 0) })}
                      />
                    )}
                  </HStack>
                </Table.Cell>
                <Table.Cell fontFamily="mono" color="fg.muted">
                  {formatMoney(netUnitCostOf(l))}
                  {factorOf(l) > 1 && (
                    <Text fontSize="xs">/{t("inventory.products.baseUnit").toLowerCase()}</Text>
                  )}
                </Table.Cell>
                <Table.Cell fontFamily="mono" fontWeight="medium">
                  {formatMoney(lineNet(l))}
                </Table.Cell>
                <Table.Cell>
                  <IconButton
                    aria-label="remove line"
                    size="xs"
                    variant="ghost"
                    onClick={() => onRemove(idx)}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                </Table.Cell>
              </Table.Row>
            );
          })}
        </Table.Body>
      </Table.Root>
    </TableScroll>
  );
}
