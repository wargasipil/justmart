// Small cart-panel controls used only by the POS page (routes/Pos.tsx). Split
// out of Pos.tsx to keep the page to its own shell + cart logic; behaviour
// unchanged. Each is presentational — all state lives in the page.
import { useMemo, useState } from "react";
import {
  Button,
  Flex,
  HStack,
  IconButton,
  Popover,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";
import { FileText, Percent, UserRound } from "lucide-react";
import { useTranslation } from "react-i18next";

import DiscountField, { type DiscountType } from "../../components/DiscountField";
import type { Sale, SaleItem } from "../../gen/pos_iface/v1/sale_pb";
import { formatMoney } from "../../lib/format";
import { useCustomerRefs } from "../../queries/refs";

// LineDiscountPopover is the per-cart-line discount affordance: a small button
// (highlighted when a discount is set) that opens a popover with the shared
// <DiscountField> + Apply/Clear. Draft state is local; Apply commits via the
// passed handler. Kept always-mounted, controlled by `open` (Ark body-lock rule).
export function LineDiscountPopover({
  item,
  onApply,
  onClear,
}: {
  item: SaleItem;
  onApply: (type: DiscountType, human: number) => void | Promise<void>;
  onClear: () => void | Promise<void>;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const seed = (): { type: DiscountType; value: number } => {
    // Seed from a MANUAL discount only; an auto product discount starts blank so
    // the cashier types a fresh override.
    const type = (item.discountManual ? item.discountType : "FIXED") as DiscountType;
    const raw = item.discountManual ? Number(item.discountValue) : 0;
    return { type, value: type === "PERCENT" ? raw / 100 : raw };
  };
  const [draft, setDraft] = useState(seed);
  const hasDiscount = Number(item.lineDiscount) > 0;
  const isAuto = hasDiscount && !item.discountManual;

  return (
    <Popover.Root
      open={open}
      onOpenChange={(e) => {
        setOpen(e.open);
        if (e.open) setDraft(seed());
      }}
      positioning={{ placement: "bottom-end" }}
    >
      <Popover.Trigger asChild>
        <IconButton
          aria-label={t("pos.lineDiscount")}
          size="xs"
          variant={hasDiscount ? "subtle" : "ghost"}
          colorPalette={hasDiscount ? "blue" : undefined}
        >
          <Percent size={14} />
        </IconButton>
      </Popover.Trigger>
      <Portal>
        <Popover.Positioner>
          <Popover.Content width="auto">
            <Popover.Body>
              <Stack gap={2}>
                <Text fontSize="xs" color="fg.muted">
                  {t("pos.lineDiscount")}
                </Text>
                {isAuto && (
                  <Text fontSize="xs" color="green.fg">
                    {t("pos.autoDiscountHint")}
                  </Text>
                )}
                {item.tierMinQty > 0 && (
                  <Text fontSize="xs" color="purple.fg">
                    {t("pos.grosirDiscountHint")}
                  </Text>
                )}
                <DiscountField
                  type={draft.type}
                  value={draft.value}
                  onChange={(type, value) => setDraft({ type, value })}
                />
                <HStack justify="flex-end" gap={2}>
                  <Button
                    size="xs"
                    variant="ghost"
                    onClick={() => {
                      void onClear();
                      setOpen(false);
                    }}
                  >
                    {t("pos.discountClear")}
                  </Button>
                  <Button
                    size="xs"
                    colorPalette="blue"
                    onClick={() => {
                      void onApply(draft.type, draft.value);
                      setOpen(false);
                    }}
                  >
                    {t("pos.discountApply")}
                  </Button>
                </HStack>
              </Stack>
            </Popover.Body>
          </Popover.Content>
        </Popover.Positioner>
      </Portal>
    </Popover.Root>
  );
}

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
