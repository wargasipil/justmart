import { ProductKind, type Product, type ProductUnit } from "../gen/inventory_iface/v1/product_pb";

/**
 * How many of `unit` POS may sell right now, and whether the number even means
 * anything.
 *
 * POS reads live stock from GetStockLevels (a per-warehouse ledger sum keyed by
 * product), which is the freshest source for an ordinary product. But that map
 * only knows about products that HAVE batches, and two of the three product
 * kinds don't:
 *
 *  - COMPOSITE (a menu item) holds no batches of its own; what it can sell is
 *    how many portions its recipe can build, which the server already computed
 *    into `readyStock`.
 *  - SERVICE (corkage, a delivery fee) holds no stock by definition and can
 *    never run out.
 *
 * Without this branch both read 0 from the ledger map, which POS renders as
 * out-of-stock: every menu item dimmed, "not-allowed", and refusing to be added.
 * That is the whole restaurant till, unusable.
 *
 * `unlimited` is kept separate from a large number on purpose — the caller
 * suppresses the quantity text entirely for a service rather than printing a
 * fake count.
 */
export function unitAvailability(
  product: Product,
  unit: ProductUnit,
  ledgerBaseQty: bigint | undefined,
): { available: number; unlimited: boolean } {
  if (product.kind === ProductKind.SERVICE) {
    return { available: 0, unlimited: true };
  }
  const base =
    product.kind === ProductKind.COMPOSITE
      ? Number(product.readyStock) // buildable portions, server-computed
      : Number(ledgerBaseQty ?? 0n);
  const factor = Number(unit.factor) || 1;
  return { available: Math.floor(base / factor), unlimited: false };
}

/** Whether a product may be added to the cart at all, in ANY unit. */
export function canSell(product: Product, ledgerBaseQty: bigint | undefined): boolean {
  if (product.kind === ProductKind.SERVICE) return true;
  const base =
    product.kind === ProductKind.COMPOSITE
      ? Number(product.readyStock)
      : Number(ledgerBaseQty ?? 0n);
  return base > 0;
}
