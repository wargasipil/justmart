import type { ProductUnit } from "../gen/inventory_iface/v1/product_pb";

// Money + unit math for one editable purchase-order (restock) line.
//
// Pure derivation, deliberately kept out of the route: every function here
// mirrors the backend's own arithmetic (see lineNetSubtotal in
// service/purchasing) so the on-screen preview and the stored order agree to
// the rupiah. Change one side and you must change the other.

export type DiscountType = "FIXED" | "PERCENT";

// The 4 effective discount modes = (discountType, discountPerItem). The "_ITEM"
// modes apply the discount to each item's cost (× qty) instead of the whole line.
export type DiscountMode = "FIXED" | "PERCENT" | "FIXED_ITEM" | "PERCENT_ITEM";

export const DISCOUNT_MODES: DiscountMode[] = [
  "FIXED",
  "PERCENT",
  "FIXED_ITEM",
  "PERCENT_ITEM",
];

export type Line = {
  productId: string;
  // Snapshotted at pick time so the row renders a name without a second fetch —
  // the product is chosen in the picker dialog, not on the row.
  productName: string;
  productSku: string;
  productUnitId: string; // chosen purchasable unit ("" => base)
  units: ProductUnit[]; // purchasable + active units of the picked product
  orderedQty: number; // in the chosen unit
  costPerItem: number; // GROSS cost per chosen purchasable unit (entered); line total is derived
  discountType: DiscountType;
  discountPerItem: boolean;
  discountValue: number; // FIXED: minor units; PERCENT: human decimal percent (e.g. 12.5)
};

export const emptyLine = (): Line => ({
  productId: "",
  productName: "",
  productSku: "",
  productUnitId: "",
  units: [],
  orderedQty: 1,
  costPerItem: 0,
  discountType: "FIXED",
  discountPerItem: false,
  discountValue: 0,
});

export const modeOf = (l: Line): DiscountMode =>
  l.discountPerItem
    ? l.discountType === "PERCENT"
      ? "PERCENT_ITEM"
      : "FIXED_ITEM"
    : l.discountType;

export const modeToParts = (
  m: DiscountMode,
): { discountType: DiscountType; discountPerItem: boolean } => ({
  discountType: m === "PERCENT" || m === "PERCENT_ITEM" ? "PERCENT" : "FIXED",
  discountPerItem: m === "FIXED_ITEM" || m === "PERCENT_ITEM",
});

export const factorOf = (l: Line): number => {
  const u = l.units.find((x) => x.id === l.productUnitId);
  return u ? Number(u.factor) : 1;
};

export const baseQtyOf = (l: Line): number => l.orderedQty * factorOf(l);

// GROSS cost per BASE unit — derived from the entered cost-per-item / factor.
// Sent to the backend as unit_cost_price; the preview uses this same rounded
// integer so the displayed totals agree with what the server stores.
export const unitCostBaseOf = (l: Line): number =>
  Math.round(l.costPerItem / factorOf(l));

// GROSS extended line amount = base qty × per-base cost.
export const grossOf = (l: Line): number => baseQtyOf(l) * unitCostBaseOf(l);

// Per-line discount amount — mirrors the backend lineNetSubtotal EXACTLY so the
// preview matches: per-item rounds each item then × qty; per-line rounds the
// whole line. PERCENT value is converted to basis points first (×100), like submit.
export const lineDiscountAmount = (l: Line): number => {
  const gross = grossOf(l);
  const chosenQty = l.orderedQty;
  const isPct = l.discountType === "PERCENT";
  const val = isPct ? Math.round(l.discountValue * 100) : l.discountValue; // bp | rupiah
  let disc: number;
  if (l.discountPerItem && chosenQty > 0) {
    const perItemGross = Math.floor(gross / chosenQty); // exact (gross is a multiple of qty)
    let perItemDisc = isPct ? Math.floor((perItemGross * val + 5000) / 10000) : val;
    if (perItemDisc > perItemGross) perItemDisc = perItemGross;
    disc = perItemDisc * chosenQty;
  } else if (!isPct) {
    disc = val;
  } else {
    disc = Math.floor((gross * val + 5000) / 10000);
  }
  return Math.max(0, Math.min(disc, gross));
};

export const lineNet = (l: Line): number => grossOf(l) - lineDiscountAmount(l);

// NET cost per base unit — what flows to the received batch's cost_price.
//
// Net of the line discount and INCLUSIVE of PPN, because PPN is capitalized
// into inventory cost (unrecoverable for a non-PKP shop). Pass the PO's rate as
// a whole percent, or 0 when the PPN switch is off.
//
// Mirrors common.NetUnitCost on the backend — scale first, round ONCE, so the
// preview and the stored batch agree to the rupiah. Change one, change both.
// Takes the raw net + base qty rather than a Line so the saved-PO detail page
// (which reads a PurchaseOrderItem, not an editable Line) shares it.
export const netUnitCostFrom = (net: number, baseQty: number, ppnRate = 0): number => {
  if (baseQty <= 0) return 0;
  const rate = Math.max(0, Math.min(100, ppnRate));
  return Math.round((net * (100 + rate)) / (baseQty * 100));
};

export const netUnitCostOf = (l: Line, ppnRate = 0): number =>
  netUnitCostFrom(lineNet(l), baseQtyOf(l), ppnRate);

// fmtUnitQty renders a BASE-unit quantity in its purchasable unit, e.g.
// (500, "box", 100n) -> "5 box". Falls back to the bare number when no unit.
// Lives here rather than beside a route because both the PO list and the PO
// detail page render purchase quantities and each had its own copy.
export const fmtUnitQty = (qty: number, unitName: string, factor: bigint): string => {
  const f = Number(factor) || 1;
  const q = f > 1 ? qty / f : qty;
  return unitName ? `${q} ${unitName}` : String(q);
};

export const unitNameOf = (l: Line): string =>
  l.units.find((x) => x.id === l.productUnitId)?.name ?? "";

// The entered cost per chosen unit — the basis for the price-agreement compare.
export const perChosenUnitGross = (l: Line): number => l.costPerItem;
