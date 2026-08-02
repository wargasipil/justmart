import { useMemo } from "react";
import { Flex, HStack, Stack, Text } from "@chakra-ui/react";
import { Building2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Supplier } from "../gen/inventory_iface/v1/supplier_pb";
import { useSupplierRefs } from "../queries/refs";
import { searchSuppliers } from "../queries/suppliers";
import SearchableSelect from "./SearchableSelect";

type SupplierLike = { code: string; name: string };

// The ONE supplier label format for TABLE CELLS: "SUP-0001 · Kimia Farma". Code
// first, because a Supplier column is read by scanning straight down it and the
// fixed-width code is what makes that scan work. Cells that print a supplier
// should import this rather than re-inlining the template.
export function supplierLabel(s: SupplierLike | undefined): string | undefined {
  return s ? `${s.code} · ${s.name}` : undefined;
}

// The picker's own format: "Kimia Farma · SUP-0001". Deliberately the reverse —
// a trigger is read on its own, right after clicking a dropdown row whose name
// led, so echoing that order is what makes the selection feel like the thing
// that was picked. It must be used for BOTH `itemToString` and the resolved
// `selectedLabel`, or the trigger would flip format when the list rotates.
function pickerLabel(s: SupplierLike | undefined): string | undefined {
  return s ? `${s.name} · ${s.code}` : undefined;
}

type Props = {
  value: string;
  /** Omit for a read-only display (pair with `disabled`). */
  onChange?: (id: string) => void;
  /** Fires with the full picked supplier alongside onChange. */
  onSelectItem?: (supplier: Supplier | undefined) => void;
  /**
   * Trigger label override for a value whose supplier the caller already has in
   * hand. Leave it off and the component resolves the label itself — that is
   * the point of this wrapper.
   */
  selectedLabel?: string;
  placeholder?: string;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
};

// Supplier picker: the shared <SearchableSelect> in async mode (HARD RULE for
// dynamic option sources) pre-wired to `searchSuppliers` with the one label
// format. Use this everywhere a supplier is chosen — filter toolbars, create
// forms, locked read-only rows.
//
// It also OWNS its trigger label. A pre-set `value` (edit drawer, filter
// restored from a URL, supplier locked via a query param) would otherwise render
// as a raw UUID until the first search fires, and every call site was hand-
// rolling the same `useSupplierRefs(...)` + IIFE to avoid that. Resolution is
// internal and bounded to the single selected id; pass `selectedLabel` only when
// the caller genuinely already has the supplier loaded, which skips the fetch.
export default function SupplierSelect({
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
  // supplied the label or nothing is selected (`useSupplierRefs` is disabled on
  // an empty id list, so this costs zero requests in the common empty-filter case).
  const refs = useSupplierRefs(
    useMemo(() => (value && !selectedLabel ? [value] : []), [value, selectedLabel]),
  );
  const label = selectedLabel ?? pickerLabel(refs.get(value));

  return (
    <SearchableSelect
      value={value}
      onChange={onChange ?? (() => {})}
      onSelectItem={onSelectItem}
      loadOptions={searchSuppliers}
      // Trigger stays one line — the two-line layout below is the open list only.
      itemToString={(s) => pickerLabel(s)!}
      itemToValue={(s) => s.id}
      renderItem={(s) => <SupplierOption supplier={s} />}
      selectedLabel={label}
      // Same glyph as the rows, so the field reads as "a supplier goes here"
      // before anything is picked and as the picked supplier after.
      startElement={<Building2 size={14} />}
      placeholder={placeholder ?? t("common.selectSupplier")}
      disabled={disabled}
      size={size}
      width={width}
    />
  );
}

// One dropdown row: supplier glyph + name over a muted code. The name leads
// because that is what staff actually scan for; the code drops to the second
// line where it still disambiguates two similarly-named distributors without
// competing with the name for attention.
//
// Building2 is the icon the sidebar already uses for /inventory/suppliers, so
// the row carries the same mark as the nav entry these suppliers come from.
// The 32px slot matches <UserAvatar size="sm"> in CashierFilterSelect, keeping
// row heights consistent between the two pickers.
function SupplierOption({ supplier }: { supplier: Supplier }) {
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
        <Building2 size={16} />
      </Flex>
      <Stack gap={0} flex="1" minW={0}>
        <Text fontWeight="medium" truncate>
          {supplier.name}
        </Text>
        {supplier.code && (
          <Text fontSize="xs" color="fg.muted" truncate>
            {supplier.code}
          </Text>
        )}
      </Stack>
    </HStack>
  );
}
