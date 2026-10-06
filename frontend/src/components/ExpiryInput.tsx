import { useState, type Ref } from "react";
import { DatePicker, IconButton, Input, InputGroup, Portal, Stack, Text, parseDate } from "@chakra-ui/react";
import { CalendarRange } from "lucide-react";
import { useTranslation } from "react-i18next";

import { CalendarViews } from "./DatePicker";
import {
  EXPIRY_SOON_DAYS,
  daysUntilExpiry,
  endOfMonthIso,
  formatExpiryText,
  isoToLocalDate,
  parseExpiryInput,
} from "../lib/expiry";

export type ExpiryInputProps = {
  /** "YYYY-MM-DD", or "" when empty. */
  value: string;
  /** Emits "YYYY-MM-DD" once the text reads as a date, "" while empty or incomplete. */
  onChange: (value: string) => void;
  /** The value is the product's default, not yet typed over — said in the readout. */
  isDefault?: boolean;
  /** A date that has already passed is an error (pharmacy), not just a warning. */
  blockExpired?: boolean;
  /** Enter was pressed — e.g. jump to the next line's expiry. */
  onEnter?: () => void;
  inputRef?: Ref<HTMLInputElement>;
  size?: "xs" | "sm" | "md";
  width?: string | number;
  disabled?: boolean;
  "aria-label"?: string;
};

/**
 * Expiry entry that reads what the pack says. Type `0327` and it is 31 Mar 2027
 * (a month-only expiry means the end of that month); `05/03/27` is the exact
 * day. Day-first in every UI language, because it follows the Indonesian pack,
 * not the screen. The line under the field says what will be SAVED, so a
 * misread is visible before submit rather than silent after. The calendar
 * button opens on a month grid — picking a month is two clicks.
 *
 * The typed text is kept as typed; the value it emits is the parsed date. A
 * value set from outside (a pre-filled default, a reset) replaces the text.
 */
export default function ExpiryInput({
  value,
  onChange,
  isDefault,
  blockExpired,
  onEnter,
  inputRef,
  size = "md",
  width,
  disabled,
  "aria-label": ariaLabel,
}: ExpiryInputProps) {
  const { t, i18n } = useTranslation();
  const [draft, setDraft] = useState(() => (value ? formatExpiryText(value) : ""));

  // Keep the text in step with a value changed from OUTSIDE. Our own emits
  // match what the draft parses to, so they never overwrite what is being typed
  // (an incomplete "03" emits "" and stays "03").
  const [seen, setSeen] = useState(value);
  if (value !== seen) {
    setSeen(value);
    if (value !== (parseExpiryInput(draft)?.iso ?? "")) {
      setDraft(value ? formatExpiryText(value) : "");
    }
  }

  const type = (text: string) => {
    setDraft(text);
    onChange(parseExpiryInput(text)?.iso ?? "");
  };
  const pickMonth = (year: number, month: number) => {
    const iso = endOfMonthIso(year, month);
    setDraft(formatExpiryText(iso));
    onChange(iso);
  };

  const parsed = parseExpiryInput(draft);
  const dateLabel = (iso: string) =>
    new Intl.DateTimeFormat(i18n.language, { day: "numeric", month: "short", year: "numeric" }).format(
      isoToLocalDate(iso),
    );

  let readout: { text: string; tone: "muted" | "warn" | "error" };
  if (!draft.trim()) {
    readout = { text: t("expiryInput.hint"), tone: "muted" };
  } else if (!parsed) {
    readout = { text: t("expiryInput.invalid"), tone: "error" };
  } else {
    const parts = [`→ ${dateLabel(parsed.iso)}`];
    if (parsed.monthOnly) parts.push(`(${t("expiryInput.endOfMonth")})`);
    if (isDefault) parts.push(`· ${t("expiryInput.productDefault")}`);
    const days = daysUntilExpiry(parsed.iso);
    let tone: "muted" | "warn" | "error" = "muted";
    if (days < 0) {
      parts.push(`· ${t("expiryInput.expired")}`);
      tone = blockExpired ? "error" : "warn";
    } else if (days <= EXPIRY_SOON_DAYS) {
      parts.push(`· ${days === 0 ? t("expiryInput.expiresToday") : t("expiryInput.expiresInDays", { count: days })}`);
      tone = "warn";
    }
    readout = { text: parts.join(" "), tone };
  }

  const selected = (() => {
    if (!value) return [];
    try {
      return [parseDate(value)];
    } catch {
      return [];
    }
  })();

  return (
    <Stack gap={0.5} width={width}>
      <InputGroup
        endElement={
          <DatePicker.Root
            value={selected}
            onValueChange={(d) => {
              const v = d.value[0];
              if (v) pickMonth(v.year, v.month);
            }}
            minView="month"
            defaultView="month"
            locale={i18n.language}
            disabled={disabled}
            closeOnSelect
          >
            <DatePicker.Trigger asChild>
              <IconButton aria-label={t("expiryInput.pickMonth")} variant="ghost" size="xs" me="-1">
                <CalendarRange size={14} />
              </IconButton>
            </DatePicker.Trigger>
            <Portal>
              <DatePicker.Positioner>
                <DatePicker.Content>
                  <CalendarViews />
                </DatePicker.Content>
              </DatePicker.Positioner>
            </Portal>
          </DatePicker.Root>
        }
      >
        <Input
          ref={inputRef}
          size={size}
          value={draft}
          onChange={(e) => type(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && onEnter) {
              e.preventDefault();
              onEnter();
            }
          }}
          inputMode="numeric"
          placeholder={t("expiryInput.placeholder")}
          aria-label={ariaLabel}
          aria-invalid={readout.tone === "error" || undefined}
          disabled={disabled}
          fontFamily="mono"
        />
      </InputGroup>
      <Text
        fontSize="xs"
        lineHeight="short"
        color={readout.tone === "error" ? "fg.error" : readout.tone === "warn" ? "fg.warning" : "fg.muted"}
      >
        {readout.text}
      </Text>
    </Stack>
  );
}
