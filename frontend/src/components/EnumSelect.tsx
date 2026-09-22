import { Portal, Select, createListCollection } from "@chakra-ui/react";
import { useMemo } from "react";

// Wrapper around Chakra v3's `Select` (popover, no search). Use for short
// fixed-enum option sets — status filters, role pickers, payment source,
// date-range presets, branch switcher. For dynamic / long lists, use
// <SearchableSelect> instead.
//
// Single-select by design; multi-select can land as a follow-up when a
// real call site needs it.
export type EnumSelectProps<T> = {
  value: string | null;
  onChange: (value: string) => void;
  items: readonly T[];
  itemToString: (item: T) => string;
  itemToValue: (item: T) => string;
  placeholder?: string;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  /**
   * Accessible name for the trigger, for a select that stands ALONE — one
   * inside a `<Field.Root>` is already named by its `Field.Label` and should
   * not pass this. Ark always writes an `aria-labelledby` pointing at a
   * `Select.Label` this wrapper does not render, so a standalone select
   * otherwise resolves to NO name at all and a screen reader announces a bare
   * "combobox". Pass a translated string.
   */
  ariaLabel?: string;
};

type Entry = { label: string; value: string };

export default function EnumSelect<T>({
  value,
  onChange,
  items,
  itemToString,
  itemToValue,
  placeholder,
  disabled,
  size = "md",
  width,
  ariaLabel,
}: EnumSelectProps<T>) {
  // Memoize the collection so identity stays stable across renders — Chakra's
  // select machine spins if the collection swaps every paint.
  const collection = useMemo(
    () =>
      createListCollection<Entry>({
        items: items.map((item) => ({
          label: itemToString(item),
          value: itemToValue(item),
        })),
        itemToString: (i) => i.label,
        itemToValue: (i) => i.value,
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [items],
  );

  const valueArr = value ? [value] : [];

  return (
    <Select.Root
      collection={collection}
      value={valueArr}
      onValueChange={(d) => onChange(d.value[0] ?? "")}
      disabled={disabled}
      size={size}
      width={width}
    >
      <Select.HiddenSelect />
      <Select.Control>
        {/* Ark's own aria-labelledby stays; it points at a Select.Label this
            wrapper never renders, so it resolves to the empty string and the
            accname algorithm falls through to aria-label. Verified in Chromium
            via Playwright's accessible-name computation. */}
        <Select.Trigger aria-label={ariaLabel}>
          <Select.ValueText placeholder={placeholder} />
        </Select.Trigger>
        <Select.IndicatorGroup>
          <Select.Indicator />
        </Select.IndicatorGroup>
      </Select.Control>
      <Portal>
        <Select.Positioner>
          <Select.Content>
            {collection.items.map((item) => (
              <Select.Item item={item} key={item.value}>
                <Select.ItemText>{item.label}</Select.ItemText>
                <Select.ItemIndicator />
              </Select.Item>
            ))}
          </Select.Content>
        </Select.Positioner>
      </Portal>
    </Select.Root>
  );
}
