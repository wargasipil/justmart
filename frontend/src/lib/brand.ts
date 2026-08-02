import { Pill, Store, UtensilsCrossed, type LucideIcon } from "lucide-react";

import { BussinessType } from "../gen/settings_iface/v1/settings_pb";

// The brand mark for a business mode, in ONE place. Two surfaces render it (the
// Sidebar top-left brand and the Login heading) and they must never disagree —
// they previously each inlined `isPharmacy ? <Pill/> : <Store/>`, which is
// exactly the shape that silently forgets the third mode.
//
// UNSPECIFIED (never configured) shares the retail mark, matching the
// retail-by-default fallback in modeFlags().
export function brandIcon(mode: BussinessType): LucideIcon {
  switch (mode) {
    case BussinessType.PHARMACY_SHOP:
      return Pill;
    case BussinessType.RESTAURANT:
      return UtensilsCrossed;
    default:
      return Store;
  }
}
