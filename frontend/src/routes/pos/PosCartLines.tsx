// The POS cart's line list: name, grosir/price hints, qty stepper, unit picker,
// line discount, line total, remove. Split out of routes/Pos.tsx; the page owns
// the sale and every mutation, this file only renders the lines.
import { Badge, Box, Flex, HStack, IconButton, Stack, Text } from "@chakra-ui/react";
import { Minus, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { DiscountType } from "../../components/DiscountField";
import EnumSelect from "../../components/EnumSelect";
import NumberInput from "../../components/NumberInput";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import type { SaleItem } from "../../gen/pos_iface/v1/sale_pb";
import { formatMoney } from "../../lib/format";
import { nextTier } from "../../queries/productPriceTiers";
import { LineDiscountPopover } from "./posControls";

export type PosCartLinesProps = {
  items: SaleItem[];
  /** The POS catalog (usePosSearch.products) — resolves each line's name + units. */
  products: Product[];
  onChangeQty: (itemId: string, qty: number) => void;
  onChangeUnit: (item: SaleItem, unitId: string) => void;
  commitLineDiscount: (itemId: string, type: DiscountType, human: number) => void | Promise<void>;
  commitClearLineDiscount: (itemId: string) => void | Promise<void>;
  onRemove: (itemId: string) => void;
};

export default function PosCartLines({
  items,
  products,
  onChangeQty,
  onChangeUnit,
  commitLineDiscount,
  commitClearLineDiscount,
  onRemove,
}: PosCartLinesProps) {
  const { t } = useTranslation();
  return (
    <Box flex="1" overflowY="auto" px={4} py={2}>
      {items.length === 0 && (
        <Text color="fg.muted" fontSize="sm" textAlign="center" py={8}>
          {t("pos.empty")}
        </Text>
      )}
      <Stack gap={2}>
        {items.map((it) => {
          const med = products.find((m) => m.id === it.productId);
          return (
            <Flex key={it.id} align="center" gap={2}>
              <Stack gap={0} flex="1">
                <Text fontSize="sm" fontWeight="medium">
                  {med?.name ?? it.productId.slice(0, 8)}
                </Text>
                {/* Grosir: struck-through normal price + the rung earned,
                    right beside the qty input the cashier is looking at.
                    All values are server fields — no client price math. */}
                <HStack gap={1.5}>
                  {it.tierMinQty > 0 && it.listPriceSnapshot > it.unitPriceSnapshot && (
                    <Text
                      fontSize="2xs"
                      color="fg.muted"
                      fontFamily="mono"
                      textDecoration="line-through"
                    >
                      {formatMoney(it.listPriceSnapshot)}
                    </Text>
                  )}
                  <Text fontSize="xs" color="fg.muted" fontFamily="mono">
                    {formatMoney(it.unitPriceSnapshot)}
                  </Text>
                  {it.tierMinQty > 0 && (
                    <Text fontSize="2xs" color="purple.fg" fontFamily="mono">
                      {t("pos.grosirThreshold", { qty: it.tierMinQty })}
                    </Text>
                  )}
                </HStack>
                {/* "Buy N more" nudge, deliberately narrow: only when the
                    gap is a plausible ask, and never a click target — it's
                    for the cashier to relay, not to bump the qty. */}
                {(() => {
                  const unit = med?.units.find((u) => u.id === it.productUnitId);
                  if (!unit) return null;
                  const nt = nextTier(med?.priceTiers, it.productUnitId, it.qty, unit.sellPrice);
                  if (!nt) return null;
                  const remaining = nt.minQty - it.qty;
                  if (remaining <= 0 || remaining > Math.max(2, Math.ceil(nt.minQty * 0.2))) {
                    return null;
                  }
                  return (
                    <Text fontSize="2xs" color="orange.fg">
                      {t("pos.grosirNudge", { n: remaining, price: formatMoney(nt.price) })}
                    </Text>
                  );
                })()}
              </Stack>
              <IconButton
                aria-label="decrease quantity"
                size="xs"
                variant="outline"
                onClick={() => onChangeQty(it.id, it.qty - 1)}
              >
                <Minus size={14} />
              </IconButton>
              <NumberInput
                size="sm"
                width="48px"
                aria-label="line quantity"
                value={it.qty}
                onChange={(raw) => onChangeQty(it.id, Number(raw || 0))}
              />
              <IconButton
                aria-label="increase quantity"
                size="xs"
                variant="outline"
                onClick={() => onChangeQty(it.id, it.qty + 1)}
              >
                <Plus size={14} />
              </IconButton>
              {med && med.units.filter((u) => u.sellable && u.active).length > 1 ? (
                <EnumSelect
                  size="sm"
                  width="84px"
                  value={it.productUnitId}
                  onChange={(v) => onChangeUnit(it, v)}
                  items={med.units.filter((u) => u.sellable && u.active)}
                  itemToString={(u) => u.name}
                  itemToValue={(u) => u.id}
                />
              ) : (
                <Text fontSize="xs" color="fg.muted" w="84px">
                  {it.unitName || med?.unit}
                </Text>
              )}
              <LineDiscountPopover
                item={it}
                onApply={(type, human) => commitLineDiscount(it.id, type, human)}
                onClear={() => commitClearLineDiscount(it.id)}
              />
              <Stack gap={0} w="80px" align="flex-end">
                <Text fontSize="sm" fontFamily="mono">
                  {formatMoney(it.lineTotal)}
                </Text>
                {it.tierMinQty > 0 && (
                  <Badge size="xs" colorPalette="purple">
                    {t("pos.grosir")}
                  </Badge>
                )}
                {Number(it.lineDiscount) > 0 && (
                  <HStack gap={1}>
                    {/* Grosir and Promo are mutually exclusive: a tiered
                        line suppresses the auto discount, so any discount
                        shown here is the cashier's manual one. Guarded
                        explicitly rather than trusting lineDiscount === 0. */}
                    {!it.discountManual && it.tierMinQty === 0 && (
                      <Badge size="xs" colorPalette="green">
                        {t("pos.promo")}
                      </Badge>
                    )}
                    <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
                      -{formatMoney(Number(it.lineDiscount))}
                      {it.discountType === "PERCENT"
                        ? ` (${Number(it.discountValue) / 100}%)`
                        : ""}
                    </Text>
                  </HStack>
                )}
              </Stack>
              <IconButton
                aria-label="remove"
                size="xs"
                variant="ghost"
                onClick={() => onRemove(it.id)}
              >
                <Trash2 size={14} />
              </IconButton>
            </Flex>
          );
        })}
      </Stack>
    </Box>
  );
}
