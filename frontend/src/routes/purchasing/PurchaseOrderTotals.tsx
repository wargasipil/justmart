import { Box, HStack, Input, Stack, Switch, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import MoneyInput from "../../components/MoneyInput";
import { formatMoney } from "../../lib/format";

// The totals card under a restock order's lines: subtotal, cart discount, the
// PPN switch + rate, and the total. Presentational; the form owns the state
// and the arithmetic (it mirrors the server's, see lib/purchaseLine.ts).
export default function PurchaseOrderTotals({
  subtotal,
  cartDiscount,
  onCartDiscountChange,
  ppnEnabled,
  onPpnEnabledChange,
  ppnRate,
  onPpnRateChange,
  ppnAmount,
  total,
}: {
  subtotal: number;
  cartDiscount: number;
  onCartDiscountChange: (v: number) => void;
  ppnEnabled: boolean;
  onPpnEnabledChange: (v: boolean) => void;
  ppnRate: number;
  onPpnRateChange: (v: number) => void;
  ppnAmount: number;
  total: number;
}) {
  const { t } = useTranslation();
  return (
    <Box
      borderWidth="1px"
      borderRadius="md"
      p={4}
      maxW="420px"
      alignSelf="flex-end"
      w="full"
    >
      <Stack gap={2}>
        <HStack justify="space-between">
          <Text color="fg.muted">{t("purchasing.subtotal")}</Text>
          <Text fontFamily="mono">{formatMoney(subtotal)}</Text>
        </HStack>
        <HStack justify="space-between" align="center">
          <Text color="fg.muted">{t("purchasing.cartDiscount")}</Text>
          <MoneyInput
            size="sm"
            width="140px"
            value={cartDiscount}
            onChange={(raw) => onCartDiscountChange(Number(raw || 0))}
          />
        </HStack>
        <HStack justify="space-between" align="center">
          <HStack gap={2}>
            <Switch.Root
              checked={ppnEnabled}
              onCheckedChange={(e) => onPpnEnabledChange(e.checked)}
            >
              <Switch.HiddenInput />
              <Switch.Control />
            </Switch.Root>
            <Text color="fg.muted">{t("purchasing.ppn")}</Text>
            <Input
              size="xs"
              type="number"
              width="60px"
              value={ppnRate}
              onChange={(e) => onPpnRateChange(parseInt(e.target.value, 10) || 0)}
              disabled={!ppnEnabled}
              min={0}
              max={100}
              aria-label={t("purchasing.ppnRate")}
            />
            <Text color="fg.muted">%</Text>
          </HStack>
          <Text fontFamily="mono" color={ppnEnabled ? "fg" : "fg.muted"}>
            {formatMoney(ppnAmount)}
          </Text>
        </HStack>
        <HStack justify="space-between" pt={2} borderTopWidth="1px">
          <Text fontWeight="bold">{t("purchasing.total")}</Text>
          <Text fontWeight="bold" fontFamily="mono">
            {formatMoney(total)}
          </Text>
        </HStack>
      </Stack>
    </Box>
  );
}
