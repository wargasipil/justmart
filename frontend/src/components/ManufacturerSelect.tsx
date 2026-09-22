import { useMemo } from "react";
import { Flex, HStack, Stack, Text } from "@chakra-ui/react";
import { Factory } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Manufacturer } from "../gen/inventory_iface/v1/manufacturer_pb";
import { searchManufacturers } from "../queries/manufacturers";
import { useManufacturerRefs } from "../queries/refs";
import SearchableSelect from "./SearchableSelect";

type ManufacturerLike = { code: string; name: string };

// The ONE manufacturer label format for TABLE CELLS: "MFR-0001 · Kalbe Farma".
// Code first, because a Pabrik column is read by scanning straight down it and
// the fixed-width code is what makes that scan work. Cells that print a
// manufacturer import this rather than re-inlining the template. Mirrors
// `supplierLabel` deliberately — the two columns sit side by side on a product.
export function manufacturerLabel(m: ManufacturerLike | undefined): string | undefined {
  return m ? `${m.code} · ${m.name}` : undefined;
}

// The picker's own format: "Kalbe Farma · MFR-0001". Deliberately the reverse —
// a trigger is read on its own, right after clicking a dropdown row whose name
// led, so echoing that order is what makes the selection feel like the thing
// that was picked. It must be used for BOTH `itemToString` and the resolved
// `selectedLabel`, or the trigger would flip format when the list rotates.
function pickerLabel(m: ManufacturerLike | undefined): string | undefined {
  return m ? `${m.name} · ${m.code}` : undefined;
}

type Props = {
  value: string;
  /** Omit for a read-only display (pair with `disabled`). */
  onChange?: (id: string) => void;
  /** Fires with the full picked manufacturer alongside onChange. */
  onSelectItem?: (manufacturer: Manufacturer | undefined) => void;
  /**
   * Trigger label override for a value whose manufacturer the caller already has
   * in hand. Leave it off and the component resolves the label itself — that is
   * the point of this wrapper.
   */
  selectedLabel?: string;
  placeholder?: string;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
};

// Pabrik picker: the shared <SearchableSelect> in async mode (HARD RULE for
// dynamic option sources) pre-wired to `searchManufacturers` with the one label
// format. Use this everywhere a manufacturer is chosen.
//
// It OWNS its trigger label, like <SupplierSelect>. A pre-set `value` (the
// product edit form is the main one) would otherwise render as a raw UUID until
// the first search fires. Resolution is internal and bounded to the single
// selected id; pass `selectedLabel` only when the caller genuinely already has
// the manufacturer loaded, which skips the fetch.
//
// Factory is the same glyph the Inventaris nav uses for /inventory/manufacturers,
// so the field carries the mark of the page these rows come from — and is what
// distinguishes it at a glance from the Building2 supplier field beside it.
export default function ManufacturerSelect({
  value,
  onChange,
  onSelectItem,
  selectedLabel,
  placeholder,
  disabled,
  size,
  width,
}: Props) {
  const { t } = useTranslation();
  // Resolve-by-IDs for the selected row only — and not at all when the caller
  // supplied the label or nothing is selected (`useManufacturerRefs` is disabled
  // on an empty id list, so an unset field costs zero requests).
  const refs = useManufacturerRefs(
    useMemo(() => (value && !selectedLabel ? [value] : []), [value, selectedLabel]),
  );
  const label = selectedLabel ?? pickerLabel(refs.get(value));

  return (
    <SearchableSelect
      value={value}
      onChange={onChange ?? (() => {})}
      onSelectItem={onSelectItem}
      loadOptions={searchManufacturers}
      // Trigger stays one line — the two-line layout below is the open list only.
      itemToString={(m) => pickerLabel(m)!}
      itemToValue={(m) => m.id}
      renderItem={(m) => <ManufacturerOption manufacturer={m} />}
      selectedLabel={label}
      startElement={<Factory size={14} />}
      placeholder={placeholder ?? t("common.selectManufacturer")}
      disabled={disabled}
      size={size}
      width={width}
    />
  );
}

// One dropdown row: pabrik glyph + name over a muted code. The name leads
// because that is what staff actually scan for; the code drops to the second
// line where it still disambiguates without competing for attention. The 32px
// slot matches <SupplierOption>, keeping row heights consistent between the two
// pickers a product form shows together.
function ManufacturerOption({ manufacturer }: { manufacturer: Manufacturer }) {
  return (
    <HStack gap={3} flex="1" minW={0}>
      <Flex
        align="center"
        justify="center"
        boxSize={8}
        flexShrink={0}
        borderRadius="md"
        bg="bg.muted"
        color="fg.muted"
      >
        <Factory size={16} />
      </Flex>
      <Stack gap={0} flex="1" minW={0}>
        <Text fontWeight="medium" truncate>
          {manufacturer.name}
        </Text>
        {manufacturer.code && (
          <Text fontSize="xs" color="fg.muted" truncate>
            {manufacturer.code}
          </Text>
        )}
      </Stack>
    </HStack>
  );
}
