import { HStack, Input } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import EnumSelect from "./EnumSelect";
import MoneyInput from "./MoneyInput";

export type DiscountType = "FIXED" | "PERCENT";

type Props = {
  type: DiscountType;
  /** FIXED: minor units (Rp). PERCENT: human percent (e.g. 10 or 12.5). */
  value: number;
  onChange: (type: DiscountType, value: number) => void;
  onBlur?: () => void;
  size?: "xs" | "sm" | "md" | "lg";
  disabled?: boolean;
  valueWidth?: string;
};

// DiscountField is a Rp/% toggle + a value input — the shared affordance for
// per-line and cart-level POS discounts. value is in HUMAN units (minor units
// for FIXED, plain percent for PERCENT); callers convert to basis points (×100)
// when sending PERCENT to the backend. Mirrors the purchasing inline discount.
export default function DiscountField({
  type,
  value,
  onChange,
  onBlur,
  size = "sm",
  disabled,
  valueWidth = "110px",
}: Props) {
  const { t } = useTranslation();
  return (
    <HStack gap={1}>
      <EnumSelect
        size={size}
        width="92px"
        disabled={disabled}
        value={type}
        onChange={(v) => onChange(v as DiscountType, 0)}
        items={["FIXED", "PERCENT"] as const}
        itemToString={(d) => t(d === "FIXED" ? "purchasing.fixed" : "purchasing.percent")}
        itemToValue={(d) => d}
      />
      {type === "PERCENT" ? (
        <Input
          size={size}
          type="number"
          step="0.01"
          min={0}
          max={100}
          width="74px"
          disabled={disabled}
          value={value || ""}
          onChange={(e) =>
            onChange("PERCENT", Math.min(100, Math.max(0, Number(e.target.value) || 0)))
          }
          onBlur={onBlur}
          aria-label={t("pos.discount")}
        />
      ) : (
        <MoneyInput
          size={size}
          width={valueWidth}
          disabled={disabled}
          value={value}
          onChange={(raw) => onChange("FIXED", Number(raw || 0))}
          onBlur={onBlur}
          aria-label={t("pos.discount")}
        />
      )}
    </HStack>
  );
}
