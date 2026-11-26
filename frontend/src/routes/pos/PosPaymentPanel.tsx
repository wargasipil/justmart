// The POS totals + payment block: subtotal, cart discount, service fee
// (pharmacy), total, payment source, cash paid + change, and Complete. Split out
// of routes/Pos.tsx; the page owns the values and the mutations.
import { Box, Button, Flex, HStack, RadioGroup, Stack, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import DiscountField, { type DiscountType } from "../../components/DiscountField";
import MoneyInput from "../../components/MoneyInput";
import { PaymentSource, type Sale } from "../../gen/pos_iface/v1/sale_pb";
import { formatMoney } from "../../lib/format";
import { QuickAmountRow } from "./posControls";

type CartDiscount = { type: DiscountType; value: number };

export type PosPaymentPanelProps = {
  sale: Sale | null;
  isPharmacy: boolean;
  /** The cart-discount inputs (human percent or Rp), mirrored from the sale. */
  cartDisc: CartDiscount;
  setCartDisc: (next: CartDiscount) => void;
  commitCartDiscount: (type: DiscountType, human: number) => void | Promise<void>;
  feeInput: string;
  setFeeInput: (v: string) => void;
  commitFee: () => void | Promise<void>;
  total: number;
  paymentSource: PaymentSource;
  setPaymentSource: (p: PaymentSource) => void;
  paidAmount: string;
  setPaidAmount: (v: string) => void;
  change: number;
  doComplete: () => void | Promise<void>;
  canComplete: boolean;
  completing: boolean;
};

export default function PosPaymentPanel({
  sale,
  isPharmacy,
  cartDisc,
  setCartDisc,
  commitCartDiscount,
  feeInput,
  setFeeInput,
  commitFee,
  total,
  paymentSource,
  setPaymentSource,
  paidAmount,
  setPaidAmount,
  change,
  doComplete,
  canComplete,
  completing,
}: PosPaymentPanelProps) {
  const { t } = useTranslation();
  return (
    <Box borderTopWidth="1px" px={4} py={3}>
      <Stack gap={2}>
        <Flex justify="space-between">
          <Text fontSize="sm" color="fg.muted">{t("pos.subtotal")}</Text>
          <Text fontSize="sm" fontFamily="mono">
            {formatMoney(Number(sale?.subtotal ?? 0n))}
          </Text>
        </Flex>
        <Flex justify="space-between" align="center">
          <Text fontSize="sm" color="fg.muted">{t("pos.discount")}</Text>
          <DiscountField
            type={cartDisc.type}
            value={cartDisc.value}
            onChange={(type, value) => {
              const typeChanged = type !== cartDisc.type;
              setCartDisc({ type, value });
              if (typeChanged) void commitCartDiscount(type, value);
            }}
            onBlur={() => void commitCartDiscount(cartDisc.type, cartDisc.value)}
            valueWidth="120px"
          />
        </Flex>
        {Number(sale?.cartDiscount ?? 0n) > 0 && (
          <Flex justify="flex-end">
            <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
              -{formatMoney(Number(sale?.cartDiscount ?? 0n))}
            </Text>
          </Flex>
        )}
        {isPharmacy && (
          <Flex justify="space-between" align="center">
            <Text fontSize="sm" color="fg.muted">{t("prescriptions.biayaJasa")}</Text>
            <MoneyInput
              size="sm"
              width="120px"
              value={feeInput}
              onChange={setFeeInput}
              onBlur={commitFee}
              aria-label={t("prescriptions.biayaJasa")}
            />
          </Flex>
        )}
        <Flex justify="space-between">
          <Text fontWeight="semibold">{t("pos.total")}</Text>
          <Text fontWeight="semibold" fontFamily="mono">
            {formatMoney(total)}
          </Text>
        </Flex>

        <Box pt={2}>
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("pos.payment")}
          </Text>
          <RadioGroup.Root
            value={String(paymentSource)}
            onValueChange={(d) => setPaymentSource(Number(d.value) as PaymentSource)}
          >
            <HStack gap={3}>
              <RadioGroup.Item value={String(PaymentSource.CASH)}>
                <RadioGroup.ItemHiddenInput />
                <RadioGroup.ItemIndicator />
                <RadioGroup.ItemText>{t("pos.paymentCash")}</RadioGroup.ItemText>
              </RadioGroup.Item>
              <RadioGroup.Item value={String(PaymentSource.NON_CASH)}>
                <RadioGroup.ItemHiddenInput />
                <RadioGroup.ItemIndicator />
                <RadioGroup.ItemText>{t("pos.paymentNonCash")}</RadioGroup.ItemText>
              </RadioGroup.Item>
            </HStack>
          </RadioGroup.Root>
        </Box>

        {paymentSource === PaymentSource.CASH && (
          <Stack gap={1}>
            <HStack>
              <Text fontSize="xs" color="fg.muted" minW="56px">{t("pos.paid")}</Text>
              <MoneyInput
                size="sm"
                value={paidAmount}
                onChange={setPaidAmount}
              />
            </HStack>
            <QuickAmountRow
              total={total}
              onPick={(n) => setPaidAmount(String(n))}
            />
            <Flex justify="space-between">
              <Text fontSize="xs" color="fg.muted">{t("pos.change")}</Text>
              <Text fontSize="sm" fontFamily="mono" color={change < 0 ? "fg.error" : "fg"}>
                {formatMoney(Math.max(0, change))}
              </Text>
            </Flex>
          </Stack>
        )}

        <HStack gap={2} pt={2}>
          <Button
            colorPalette="blue"
            flex="1"
            onClick={doComplete}
            disabled={!canComplete}
            loading={completing}
          >
            {t("pos.complete")}
          </Button>
        </HStack>
      </Stack>
    </Box>
  );
}
