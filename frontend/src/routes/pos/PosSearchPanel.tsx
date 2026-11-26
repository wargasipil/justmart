// The POS search panel: the search box, the out-of-stock toggle and the
// per-unit result rows. Split out of routes/Pos.tsx; the page owns the state
// (usePosSearch) and the add, this file only renders it.
//
// Stock on a row is what checkout can SELL. In pharmacy mode that leaves out
// lots whose expiry date has passed (CompleteSale never takes from them), so a
// product whose remaining stock is all expired reads 0 and behaves exactly
// like an out-of-stock one: dimmed, hidden by the toggle, not addable. The
// "N kedaluwarsa" hint says why, before the cashier gets to checkout.
import type { KeyboardEvent, RefObject } from "react";
import { Badge, Box, Flex, HStack, Input, Stack, Switch, Text } from "@chakra-ui/react";
import { Lock, Search } from "lucide-react";
import { useTranslation } from "react-i18next";

import ProductImage from "../../components/ProductImage";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../../lib/format";
import type { UnitRow } from "./usePosSearch";

export type PosSearchPanelProps = {
  searchRef: RefObject<HTMLInputElement>;
  query: string;
  setQuery: (q: string) => void;
  onSearchKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void;
  showOutOfStock: boolean;
  onToggleOutOfStock: (next: boolean) => void;
  unitRows: UnitRow[];
  highlight: number;
  setHighlight: (i: number) => void;
  isPharmacy: boolean;
  /** The draft already carries a prescription (clears the Rx lock cue). */
  hasPrescription: boolean;
  onAdd: (product: Product, unitId: string, available: number) => void;
};

export default function PosSearchPanel({
  searchRef,
  query,
  setQuery,
  onSearchKeyDown,
  showOutOfStock,
  onToggleOutOfStock,
  unitRows,
  highlight,
  setHighlight,
  isPharmacy,
  hasPrescription,
  onAdd,
}: PosSearchPanelProps) {
  const { t } = useTranslation();
  return (
    <Box flex="3" borderRightWidth="1px" overflowY="auto" p={4}>
      <Box position="relative" mb={3}>
        <Box position="absolute" left={3} top="50%" transform="translateY(-50%)" color="fg.muted">
          <Search size={16} />
        </Box>
        <Input
          ref={searchRef}
          pl={10}
          placeholder={t("pos.searchPlaceholder")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onSearchKeyDown}
          autoFocus
        />
      </Box>
      <Switch.Root
        size="sm"
        mb={3}
        checked={showOutOfStock}
        onCheckedChange={(d) => onToggleOutOfStock(d.checked)}
      >
        <Switch.HiddenInput />
        <Switch.Control />
        <Switch.Label fontSize="sm" color="fg.muted">
          {t("pos.showOutOfStock")}
        </Switch.Label>
      </Switch.Root>
      <Stack gap={1}>
        {unitRows.map((row, i) => {
          const { med: m, unit, available, expired, tiers } = row;
          const out = available < 1;
          const active = i === highlight;
          // Pharmacy cue: Rx-required product with no covering Rx yet — dimmed
          // + lock, but still clickable (the click opens the Rx picker).
          const needsRx = isPharmacy && m.prescriptionRequired && !hasPrescription;
          return (
            <Flex
              key={`${m.id}:${unit.id}`}
              px={3}
              py={2}
              borderRadius="md"
              bg={active ? "bg.muted" : "transparent"}
              borderWidth="1px"
              borderColor={active ? "border" : "transparent"}
              align="center"
              justify="space-between"
              cursor={out ? "not-allowed" : "pointer"}
              opacity={out ? 0.5 : needsRx ? 0.7 : 1}
              onMouseEnter={() => setHighlight(i)}
              onClick={() => !out && onAdd(m, unit.id, available)}
            >
              {/* Thumbnail first: a cashier scans this list by sight, and
                  the picture is the fastest thing to match against. THUMB
                  rendition; products without one cost zero requests. */}
              <ProductImage
                productId={m.id}
                name={m.name}
                version={Number(m.imageUpdatedAt)}
                size={36}
              />
              <Stack gap={0} flex="1" ml={3}>
                <HStack gap={2}>
                  {needsRx && <Lock size={12} />}
                  <Text fontSize="sm" fontWeight="medium">
                    {m.name}
                  </Text>
                  <Text fontSize="xs" color="fg.muted">
                    · {unit.name}
                  </Text>
                  {needsRx && (
                    <Text fontSize="xs" color="orange.fg">
                      {t("pos.needsRx")}
                    </Text>
                  )}
                </HStack>
                <Text fontSize="xs" color="fg.muted">
                  {m.sku} · {available} {unit.name}
                  {/* Pharmacy: stock left out of `available` because its lot
                      expired. Always 0 in retail, which sells every lot. */}
                  {expired > 0 && (
                    <Text as="span" color="fg.subtle">
                      {" · "}
                      {t("pos.expiredHint", { count: expired })}
                    </Text>
                  )}
                </Text>
              </Stack>
              <Stack gap={0} align="flex-end">
                <Text fontSize="sm" fontFamily="mono">
                  {formatMoney(unit.sellPrice)}
                </Text>
                {/* Grosir hint: only the FIRST rung — the one crossed most
                    often — plus a +N counter. The full ladder is back-office
                    detail and would be noise in a 40-row scan list. */}
                {tiers.length > 0 && (
                  <HStack gap={1}>
                    <Badge size="xs" variant="subtle" colorPalette="purple">
                      {t("pos.grosir")}
                    </Badge>
                    <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
                      {tiers.length > 1
                        ? t("pos.grosirTierHintMore", {
                            qty: tiers[0].minQty,
                            price: formatMoney(tiers[0].price),
                            n: tiers.length - 1,
                          })
                        : t("pos.grosirTierHint", {
                            qty: tiers[0].minQty,
                            price: formatMoney(tiers[0].price),
                          })}
                    </Text>
                  </HStack>
                )}
              </Stack>
            </Flex>
          );
        })}
        {unitRows.length === 0 && (
          <Text color="fg.muted" fontSize="sm" textAlign="center" py={6}>
            {/* With the filter on, an empty list means "nothing in stock
                matches", not "no such product" — say so, or the cashier
                concludes the product isn't in the catalog at all. */}
            {showOutOfStock ? t("common.noResults") : t("pos.noResultsHiddenOutOfStock")}
          </Text>
        )}
      </Stack>
    </Box>
  );
}
