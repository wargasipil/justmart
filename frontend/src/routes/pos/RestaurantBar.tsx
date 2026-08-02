import { Badge, Button, Flex, HStack, Text } from "@chakra-ui/react";
import { ChefHat, Grid2X2, Utensils } from "lucide-react";
import { useTranslation } from "react-i18next";

import EnumSelect from "../../components/EnumSelect";
import type { Sale } from "../../gen/pos_iface/v1/sale_pb";

// The order types a COUNTER order may declare. DINE_IN is absent on purpose: a
// seated order is opened from the floor plan (TableService.OpenTable), the only
// path that binds a table — offering it here would produce a dine-in bill that
// no table shows.
export const COUNTER_ORDER_TYPES = ["", "TAKEAWAY", "DELIVERY"] as const;

// RestaurantBar — restaurant mode only. Two jobs, in one strip under the
// customer bar:
//
//  - say WHERE this order is going: the bound table ("Meja T4", read-only —
//    changing it is a floor action, not a cart edit), or an order-type picker
//    for a counter order.
//  - fire the new lines to the kitchen.
//
// The fire button carries the count of unfired lines rather than being a bare
// "Fire": the number is the whole question a waiter has ("is anything waiting to
// go?"), and it disables at zero so a double tap can't be mistaken for a
// reprint. Firing is incremental server-side anyway, so a stray tap is harmless.
export default function RestaurantBar({
  sale,
  pendingCount,
  isFiring,
  onFire,
  onOrderTypeChange,
}: {
  sale: Sale | null;
  pendingCount: number;
  isFiring: boolean;
  onFire: () => void;
  onOrderTypeChange: (orderType: string) => void;
}) {
  const { t } = useTranslation();
  const tableCode = sale?.tableCode ?? "";
  const isDineIn = !!sale?.tableId;

  const typeItems = COUNTER_ORDER_TYPES.map((v) => ({
    value: v,
    label: v === "" ? t("pos.orderTypes.counter") : t(`pos.orderTypes.${v.toLowerCase()}`),
  }));

  return (
    <Flex mt={2} align="center" gap={2} wrap="wrap">
      {isDineIn ? (
        <HStack gap={2} flex="1" minW="140px">
          <Grid2X2 size={14} />
          <Badge colorPalette="orange">{t("pos.atTable", { code: tableCode })}</Badge>
          {(sale?.guestCount ?? 0) > 0 && (
            <Text fontSize="xs" color="fg.muted">
              {t("tables.guests", { count: sale?.guestCount ?? 0 })}
            </Text>
          )}
        </HStack>
      ) : (
        <HStack gap={2} flex="1" minW="180px">
          <Utensils size={14} />
          <EnumSelect
            value={sale?.orderType ?? ""}
            onChange={onOrderTypeChange}
            items={typeItems}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
            size="xs"
            width="160px"
          />
        </HStack>
      )}

      <Button
        size="xs"
        variant={pendingCount > 0 ? "solid" : "outline"}
        colorPalette="orange"
        disabled={pendingCount === 0 || !sale}
        loading={isFiring}
        onClick={onFire}
      >
        <ChefHat size={14} />
        {pendingCount > 0 ? t("pos.fireCount", { count: pendingCount }) : t("pos.fired")}
      </Button>
    </Flex>
  );
}
