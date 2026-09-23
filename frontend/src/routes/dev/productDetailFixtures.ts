import { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import { ProductDiscount } from "../../gen/inventory_iface/v1/product_discount_pb";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { ProductUnitPrice } from "../../gen/inventory_iface/v1/product_pb";
import {
  ProductPriceTier,
  ProductTierPrice,
} from "../../gen/inventory_iface/v1/product_price_tier_pb";
import { MovementType, StockMovement } from "../../gen/inventory_iface/v1/stock_pb";
import { SUPPLIERS, dateIn, daysAgo, restockLogsFor } from "./fixtures";

// What ONE product's detail page holds behind its five tabs — lots, sell-price
// history, the grosir ladder's history, the stock ledger, and the discount
// rules — plus the ladder the Grosir card on the page itself renders.
//
// It lives beside the catalog rather than inside a scenario because two sets of
// stories read it: the whole-screen ProductDetail stories (screens/**), and the
// per-tab component stories co-located with each tab in routes/inventory. A tab
// shown on its own and the same tab shown inside the page must describe the
// same Paracetamol, or one of them is quietly lying about the product.
//
// Same rules as restockFixtures.ts: it reads the catalog and nothing reads it
// back (the dependency runs one way), everything is built through the generated
// message classes, everything is DERIVED from the product — prices from its own
// units and referenceCost, so a lot cannot cost more than the shop sells it for
// — and every date is relative to now, because ExpiryBadge and the "expired"
// discount check read against today.

/**
 * The grosir ladder: two rungs on the middle pack and one on the largest, 5–10%
 * under that unit's own price (POS only applies a tier that is strictly
 * cheaper, so a ladder at or above list price would demo nothing).
 *
 * The unit ids, names and factors are read off the product rather than written
 * down — `unitsFor` numbers them `${productId}-u1|u2|u3` (base · strip · box on
 * Paracetamol), and a ladder naming a unit the product does not have would be a
 * row POS could never match.
 */
export function tiersFor(p: Product): ProductPriceTier[] {
  const [, mid, top] = p.units;
  return [
    new ProductPriceTier({ id: `${p.id}-tier1`, productId: p.id, productUnitId: mid.id, unitName: mid.name, unitFactor: mid.factor, minQty: 5, price: pct(mid.sellPrice, 95), createdAt: daysAgo(40) }),
    new ProductPriceTier({ id: `${p.id}-tier2`, productId: p.id, productUnitId: mid.id, unitName: mid.name, unitFactor: mid.factor, minQty: 20, price: pct(mid.sellPrice, 90), createdAt: daysAgo(40) }),
    new ProductPriceTier({ id: `${p.id}-tier3`, productId: p.id, productUnitId: top.id, unitName: top.name, unitFactor: top.factor, minQty: 3, price: pct(top.sellPrice, 94), createdAt: daysAgo(20) }),
  ];
}

/**
 * Three in-stock lots, chosen so the expiry column shows all three states the
 * badge has: one inside the warning window, one comfortable, one far out. Cost
 * climbs with each arrival, ending at the product's `referenceCost` — which IS
 * the newest batch's cost, so the page's "Last cost" agrees with this table.
 */
export function batchesFor(p: Product): Batch[] {
  const pre = p.sku.slice(0, 3).toUpperCase();
  return [
    new Batch({ id: `${p.id}-b1`, productId: p.id, supplierId: SUPPLIERS[1].id, batchNumber: `${pre}2409A`, expiryDate: dateIn(21), costPrice: p.referenceCost - 20n, receivedAt: dateIn(-150), currentQuantity: 140n, productName: p.name }),
    new Batch({ id: `${p.id}-b2`, productId: p.id, supplierId: SUPPLIERS[0].id, batchNumber: `${pre}2503C`, expiryDate: dateIn(240), costPrice: p.referenceCost - 10n, receivedAt: dateIn(-60), currentQuantity: 600n, productName: p.name }),
    new Batch({ id: `${p.id}-b3`, productId: p.id, supplierId: SUPPLIERS[0].id, batchNumber: `${pre}2508B`, expiryDate: dateIn(640), costPrice: p.referenceCost, receivedAt: dateIn(-12), currentQuantity: 500n, productName: p.name }),
  ];
}

/**
 * What a batch read looks like to the TILL: the server blanks supplier and cost
 * before responding (`batch.redactCost`), so a story that hands a cashier the
 * full rows would be demonstrating a leak the backend does not have.
 */
export function redactBatchesForTill(rows: Batch[]): Batch[] {
  return rows.map((b) => {
    const r = b.clone();
    r.supplierId = "";
    r.costPrice = 0n;
    return r;
  });
}

/**
 * Per-unit sell-price history, newest first. The box was re-priced once (so one
 * row is closed and one is open); strip and base have carried one price all
 * along — which is what makes "Sampai: sekarang" legible as a state rather than
 * a blank cell.
 */
export function unitPricesFor(p: Product): ProductUnitPrice[] {
  const [base, mid, top] = p.units;
  return [
    new ProductUnitPrice({ id: `${p.id}-up1`, productUnitId: top.id, unitName: top.name, unitSellPrice: top.sellPrice, effectiveFrom: daysAgo(35) }),
    new ProductUnitPrice({ id: `${p.id}-up2`, productUnitId: top.id, unitName: top.name, unitSellPrice: pct(top.sellPrice, 95), effectiveFrom: daysAgo(200), effectiveTo: daysAgo(35) }),
    new ProductUnitPrice({ id: `${p.id}-up3`, productUnitId: mid.id, unitName: mid.name, unitSellPrice: mid.sellPrice, effectiveFrom: daysAgo(200) }),
    new ProductUnitPrice({ id: `${p.id}-up4`, productUnitId: base.id, unitName: base.name, unitSellPrice: base.sellPrice, effectiveFrom: daysAgo(200) }),
  ];
}

/**
 * Grosir price history, newest first. A row is one price of one RUNG — "buy ≥ N
 * of this unit" — not of a tier row, because tiers are hard-deleted and a tier
 * id would dangle. The ≥5 strip rung was re-priced 15 days ago, so it appears
 * twice: once closed, once open. That pair is the whole reason this table is
 * separate from the one above.
 */
export function tierPricesFor(p: Product): ProductTierPrice[] {
  const [t1, t2, t3] = tiersFor(p);
  const row = (n: number, t: ProductPriceTier, price: bigint, from: bigint, to?: bigint) =>
    new ProductTierPrice({ id: `${p.id}-tp${n}`, productId: p.id, productUnitId: t.productUnitId, unitName: t.unitName, minQty: t.minQty, price, effectiveFrom: from, effectiveTo: to });
  return [
    row(1, t3, t3.price, daysAgo(20)),
    row(2, t2, t2.price, daysAgo(40)),
    row(3, t1, t1.price, daysAgo(15)),
    row(4, t1, pct(t1.price, 102), daysAgo(40), daysAgo(15)),
  ];
}

/**
 * The stock ledger, newest first — one row per movement type the tab labels, so
 * the type column is exercised end to end rather than showing four sales. The
 * batch ids are this product's own lots, and each reason reads like the
 * document that caused it (an invoice no, a receipt no, an opname name).
 */
export function movementsFor(p: Product): StockMovement[] {
  const b = batchesFor(p);
  return [
    new StockMovement({ id: `${p.id}-m1`, batchId: b[1].id, qty: -20, type: MovementType.SALE, reason: "INV-2026-0412", createdAt: daysAgo(0) }),
    new StockMovement({ id: `${p.id}-m2`, batchId: b[2].id, qty: 500, type: MovementType.PURCHASE, reason: "RCV-2026-0088", createdAt: daysAgo(12) }),
    new StockMovement({ id: `${p.id}-m3`, batchId: b[0].id, qty: -4, type: MovementType.ADJUSTMENT, reason: "Stocktake: Opname Agustus — selisih", createdAt: daysAgo(30) }),
    new StockMovement({ id: `${p.id}-m4`, batchId: b[0].id, qty: -6, type: MovementType.WRITE_OFF, reason: "Kemasan rusak", createdAt: daysAgo(44) }),
  ];
}

/**
 * Two discount rules, one live and one lapsed: a 10% per-item cut with no qty
 * rule, and a flat per-line amount gated on 100 base units that expired three
 * days ago. Between them they cover every cell the table derives — mode, value
 * formatting, the qty rule vs "no rule", and the red Kedaluwarsa badge.
 */
export function discountsFor(p: Product): ProductDiscount[] {
  return [
    new ProductDiscount({ id: `${p.id}-d1`, productId: p.id, discountType: "PERCENT", perItem: true, value: 1_000n, minQty: 0, expiresAt: dateIn(10), createdAt: daysAgo(5) }),
    new ProductDiscount({ id: `${p.id}-d2`, productId: p.id, discountType: "FIXED", perItem: false, value: 2_000n, minQty: 100, minQtyUnitName: p.unit, minQtyUnitFactor: 1n, expiresAt: dateIn(-3), createdAt: daysAgo(60) }),
  ];
}

/** Restock log rows for the tab — six arrivals, ~12 days apart. */
export function restocksFor(p: Product) {
  return restockLogsFor(p, 6);
}

/** `n` percent of an amount, in minor units. */
function pct(amount: bigint, percent: number): bigint {
  return (amount * BigInt(percent)) / 100n;
}
