import type { ChangeEvent } from "react";
import { Input } from "@chakra-ui/react";

type Props = {
  value: number | bigint | string | null | undefined;
  onChange: (raw: string) => void;
  placeholder?: string;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  disabled?: boolean;
  autoFocus?: boolean;
  onBlur?: () => void;
  max?: number;
  "aria-label"?: string;
};

// toDigits reduces any value to canonical digits: keeps [0-9] and drops leading
// zeros. Returns "" for empty OR zero, so callers show a placeholder instead of a
// stuck "0" ("0" -> "", "09000" -> "9000"). Integer/quantity only (no separators,
// no sign, no decimal point).
function toDigits(input: string): string {
  return input.replace(/\D/g, "").replace(/^0+/, "");
}

// NumberInput — controlled digits-only integer/quantity input. Composes Chakra
// <Input> (type=text + inputMode=numeric). Display is the leading-zero-stripped
// digit string (empty -> placeholder). onChange emits that same UNFORMATTED digit
// string ("" for empty/zero), matching MoneyInput's contract, so z.coerce.bigint()
// and Number(raw || 0) consumers parse cleanly (BigInt("") === 0n, Number("") === 0).
// Unlike MoneyInput there is no thousands grouping, hence no caret-reflow effect:
// nothing is inserted into the string, so native caret handling stays correct.
export default function NumberInput({
  value,
  onChange,
  placeholder = "0",
  size,
  width,
  disabled,
  autoFocus,
  onBlur,
  max,
  "aria-label": ariaLabel,
}: Props) {
  const display = toDigits(value == null ? "" : String(value));

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    let raw = toDigits(e.target.value);
    // Clamp the emitted value (not mid-typing chars): if the typed number exceeds
    // max, snap to max. "" stays "" (zero). Number(raw) is safe for quantity-sized
    // values.
    if (max != null && raw !== "" && Number(raw) > max) {
      raw = String(max);
    }
    onChange(raw);
  };

  return (
    <Input
      type="text"
      inputMode="numeric"
      value={display}
      placeholder={placeholder}
      size={size}
      width={width}
      disabled={disabled}
      autoFocus={autoFocus}
      onChange={handleChange}
      onBlur={onBlur}
      aria-label={ariaLabel}
    />
  );
}
