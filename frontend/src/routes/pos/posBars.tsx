import { Button, Flex, Text } from "@chakra-ui/react";
import { FileText, UserRound } from "lucide-react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import type { Sale } from "../../gen/pos_iface/v1/sale_pb";
import { formatMoney } from "../../lib/format";
import { useCustomerRefs } from "../../queries/refs";

// The one-line strips in the POS cart panel: who the sale is for, which resep is
// attached, and the quick-amount chips under the paid field. Moved out of
// Pos.tsx verbatim; RestaurantBar (the restaurant-mode strip) is its own file
// beside this one.
// QuickAmountRow: one-tap fill of the paid input. Renders below the Dibayar
// field for Cash payments. Includes an "Exact" chip (paid = total), an
// optional round-up-to-next-10k chip, and standard IDR banknote denominations
// (5k/10k/20k/50k/100k) filtered to amounts >= total.
export function QuickAmountRow({
  total,
  onPick,
}: {
  total: number;
  onPick: (n: number) => void;
}) {
  const { t } = useTranslation();
  if (total <= 0) return null;
  const DENOMS = [5_000, 10_000, 20_000, 50_000, 100_000];
  const above = DENOMS.filter((d) => d >= total);
  const roundedUp = Math.ceil(total / 10_000) * 10_000;
  const showRoundUp = roundedUp !== total && !above.includes(roundedUp);
  return (
    <Flex wrap="wrap" gap={1} mt={1}>
      <Button
        size="xs"
        variant="outline"
        colorPalette="blue"
        onClick={() => onPick(total)}
      >
        {t("pos.exactAmount")}
      </Button>
      {showRoundUp && (
        <Button
          size="xs"
          variant="outline"
          colorPalette="blue"
          onClick={() => onPick(roundedUp)}
        >
          {formatMoney(roundedUp)}
        </Button>
      )}
      {above.map((d) => (
        <Button
          key={d}
          size="xs"
          variant="outline"
          colorPalette="blue"
          onClick={() => onPick(d)}
        >
          {formatMoney(d)}
        </Button>
      ))}
    </Flex>
  );
}

export function CustomerBar({
  sale,
  onAttach,
  onClear,
}: {
  sale: Sale | null;
  onAttach: () => void;
  onClear: () => void;
}) {
  const { t } = useTranslation();
  const customerId = sale?.customerId ?? "";
  const hasCustomer = !!customerId;
  // Resolve the attached customer's name (manual pick OR auto-filled from an
  // attached resep) so the bar shows the name, not the raw UUID.
  const refs = useCustomerRefs(useMemo(() => (customerId ? [customerId] : []), [customerId]));
  return (
    <Flex mt={2} align="center" gap={2}>
      <UserRound size={14} />
      <Text fontSize="xs" color="fg.muted" flex="1">
        {hasCustomer
          ? (refs.get(customerId)?.name ?? customerId.slice(0, 8))
          : t("pos.customer")}
      </Text>
      {hasCustomer ? (
        <Button size="xs" variant="ghost" onClick={onClear}>
          {t("pos.clearCustomer")}
        </Button>
      ) : (
        <Button size="xs" variant="ghost" onClick={onAttach}>
          {t("pos.attachCustomer")}
        </Button>
      )}
    </Flex>
  );
}

// PrescriptionBar — pharmacy mode only. Shows the attached resep (Rx number) or
// an "attach" affordance (F5). Sits under the CustomerBar in the cart panel.
export function PrescriptionBar({
  sale,
  onAttach,
  onDetach,
}: {
  sale: Sale | null;
  onAttach: () => void;
  onDetach: () => void;
}) {
  const { t } = useTranslation();
  const attached = !!sale?.prescriptionId;
  return (
    <Flex mt={2} align="center" gap={2}>
      <FileText size={14} />
      <Text fontSize="xs" color="fg.muted" flex="1">
        {attached ? t("prescriptions.attached") : t("prescriptions.attach")}
      </Text>
      {attached ? (
        <Button size="xs" variant="ghost" onClick={onDetach}>
          {t("prescriptions.detach")}
        </Button>
      ) : (
        <Button size="xs" variant="ghost" onClick={onAttach}>
          {t("prescriptions.attach")}
        </Button>
      )}
    </Flex>
  );
}
