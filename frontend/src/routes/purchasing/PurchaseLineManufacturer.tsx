import { HStack, IconButton, Text } from "@chakra-ui/react";
import { Factory, Pencil } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import EnumSelect from "../../components/EnumSelect";
import ManufacturerSelect, {
  manufacturerLabel,
  manufacturerPickerLabel,
} from "../../components/ManufacturerSelect";
import type { Line } from "../../lib/purchaseLine";

/** Minimal shape both label formats need — what the resolve refs return. */
export type MakerRef = { id: string; code: string; name: string };

type Props = {
  line: Line;
  /** Resolved pabrik by id, or undefined while it resolves. Batched by the page. */
  refFor: (id: string) => MakerRef | undefined;
  onChange: (manufacturerId: string) => void;
};

/** Sentinel for the "a pabrik that is not on the list" entry. */
const OTHER = "__other__";

/**
 * Which pabrik one restock line is sourced from.
 *
 * The product's APPROVED list constrains without refusing: when it holds any
 * makers they are offered as a short fixed set (an <EnumSelect>, per the Selects
 * rule) so the common case is a two-click pick with no typing, and a trailing
 * "pabrik lain" entry opens the full searchable picker. A buyer holding the
 * distributor's invoice knows more than the catalog does, so the list may not be
 * a gate — naming an unlisted maker is how the catalog learns one.
 *
 * Page-local on purpose: it is coupled to this form's Line shape. It renders
 * INSIDE the product cell rather than as its own column because a phone at
 * 390px cannot afford an eighth one.
 */
export default function PurchaseLineManufacturer({ line, refFor, onChange }: Props) {
  const { t } = useTranslation();
  // Open either because the buyer asked (pencil) or because there is nothing to
  // show yet — the missing case is the one worth asking about, and most products
  // carry no pabrik at all since the column shipped with no backfill.
  const [editing, setEditing] = useState(false);
  // Sticky for this row once chosen: going back to the short list after asking
  // for the full picker would throw away the search the buyer is mid-way through.
  const [searching, setSearching] = useState(false);

  const approved = line.approvedManufacturerIds;
  const open = editing || !line.manufacturerId;

  if (!open) {
    return (
      <HStack gap={1} color="fg.muted">
        <Factory size={12} />
        <Text fontSize="xs">{manufacturerLabel(refFor(line.manufacturerId)) ?? "—"}</Text>
        <IconButton
          aria-label={t("purchasing.changeManufacturer")}
          size="2xs"
          variant="ghost"
          onClick={() => setEditing(true)}
        >
          <Pencil size={11} />
        </IconButton>
      </HStack>
    );
  }

  const pick = (id: string) => {
    setEditing(false);
    setSearching(false);
    onChange(id);
  };

  if (approved.length > 0 && !searching) {
    // Only ids the page has already resolved can be labelled; an unresolved one
    // would render as a raw UUID, so it waits rather than lying.
    const items = approved.map((id) => refFor(id)).filter((m): m is MakerRef => !!m);
    return (
      <EnumSelect
        size="xs"
        width="100%"
        ariaLabel={t("inventory.products.manufacturer")}
        placeholder={t("common.selectManufacturer")}
        value={line.manufacturerId}
        items={[...items, { id: OTHER, code: "", name: t("purchasing.otherManufacturer") }]}
        itemToString={(m: MakerRef) => (m.id === OTHER ? m.name : manufacturerLabel(m)!)}
        itemToValue={(m: MakerRef) => m.id}
        onChange={(id) => (id === OTHER ? setSearching(true) : pick(id))}
      />
    );
  }

  return (
    <ManufacturerSelect
      size="xs"
      width="100%"
      ariaLabel={t("inventory.products.manufacturer")}
      value={line.manufacturerId}
      // The page resolved this in one batched call, so the trigger never shows a
      // raw id while the select's own resolve is still in flight. The picker's
      // own format, which must match its dropdown rows.
      selectedLabel={manufacturerPickerLabel(refFor(line.manufacturerId))}
      placeholder={t("common.selectManufacturer")}
      onChange={pick}
    />
  );
}
