import type { ProductUnit } from "../gen/inventory_iface/v1/product_pb";

// Purchase cost ("modal") per unit of measure — tablet, strip, box.
//
// A lot stores its cost per BASE unit (Batch.costPrice) and, when it was
// received against an order line priced per pack, the exact NET cost of that
// pack too (costPackPrice for costPackFactor base units; Product carries the
// latest lot's pair as referenceCost / referenceCostPack / ...Factor). The pack
// figure is what the supplier's faktur says, and per-base × factor is not:
// a box of 30 bought at Rp 65.000 is 2.167 per tablet, and 2.167 × 30 = 65.010.
//
// Pure bigint math, no rendering, so the Batch tab and the product page's Last
// cost tile cannot disagree about what a box cost.

/**
 * The cost of ONE unit holding `unitFactor` base units.
 *
 * - The base unit (factor 1) is `costPrice` — the stored per-base figure,
 *   rounded once from the order line, and the one the server sorts and values
 *   stock by.
 * - The pack the lot was bought in is `packPrice`, exactly.
 * - Any other pack is scaled from the exact pack price (rounded half up once),
 *   which is closer than per-base × factor: a strip of 10 out of that box is
 *   21.667, not 21.670.
 * - A lot with no pack recorded (packFactor 0: older, manual or imported
 *   stock) falls back to per-base × factor.
 */
export function costForUnit(
  costPrice: bigint,
  packPrice: bigint,
  packFactor: bigint,
  unitFactor: bigint,
): bigint {
  if (unitFactor <= 1n) return costPrice;
  if (packFactor > 0n && unitFactor === packFactor) return packPrice;
  if (packFactor > 0n) return (packPrice * unitFactor * 2n + packFactor) / (packFactor * 2n);
  return costPrice * unitFactor;
}

/** Base units in one `u`. The base unit is 1 by definition, whatever the row stores. */
export function baseUnitsIn(u: ProductUnit): bigint {
  return u.isBase ? 1n : u.factor;
}

/**
 * The units a cost is shown per: the base unit first, then every ACTIVE pack
 * larger than it, smallest first (strip before box).
 */
export function costUnitsOf(units: ProductUnit[]): ProductUnit[] {
  return units
    .filter((u) => u.active && (u.isBase || u.factor > 1n))
    .sort((a, b) => Number(baseUnitsIn(a) - baseUnitsIn(b)));
}
