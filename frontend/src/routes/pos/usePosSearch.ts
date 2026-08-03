// The POS product-search slice: query text, keyboard highlight, the
// out-of-stock filter, and the derived per-unit result rows. Split out of
// routes/Pos.tsx; behaviour unchanged.
//
// Adding to the cart is NOT owned here — the caller passes `onAdd`, so this
// hook stays a pure read/derive slice over the catalog and stock levels.
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";

import type { Product, ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import type { ProductPriceTier } from "../../gen/inventory_iface/v1/product_price_tier_pb";
import { loadShowOutOfStock, saveShowOutOfStock } from "../../lib/posStorage";
import { useAllProductsQuery } from "../../queries/products";
import { tiersForUnit } from "../../queries/productPriceTiers";
import { useStockLevelsQuery } from "../../queries/stock";

// Each sellable unit of each matching product is its own search row, so one
// click adds that exact unit. `available` = how many of that unit the current
// base stock can make (base ÷ factor). `tiers` is precomputed (not per render)
// so the list stays cheap; it drives the DISPLAY hint only — a cart line's
// applied-grosir state always comes from the server (SaleItem.tierMinQty).
export type UnitRow = {
  med: Product;
  unit: ProductUnit;
  available: number;
  tiers: ProductPriceTier[];
};

const MAX_ROWS = 40;

export function usePosSearch({
  onAdd,
}: {
  onAdd: (product: Product, unitId: string, available?: number) => void | Promise<void>;
}) {
  const productsQ = useAllProductsQuery();
  const stockQ = useStockLevelsQuery();
  const stockByProduct = useMemo(() => {
    const out = new Map<string, bigint>();
    for (const l of stockQ.data ?? []) {
      out.set(l.productId, (out.get(l.productId) ?? 0n) + l.currentQuantity);
    }
    return out;
  }, [stockQ.data]);

  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const searchRef = useRef<HTMLInputElement | null>(null);
  // Show/hide the zero-stock rows. Persisted per device — a till that only ever
  // sells from what's on the shelf keeps the list clean across shifts.
  const [showOutOfStock, setShowOutOfStock] = useState(loadShowOutOfStock);
  const onToggleOutOfStock = useCallback((next: boolean) => {
    setShowOutOfStock(next);
    saveShowOutOfStock(next);
  }, []);

  const unitRows = useMemo<UnitRow[]>(() => {
    const q = query.trim().toLowerCase();
    const meds = q
      ? productsQ.rows.filter((m) =>
          [m.sku, m.name].some((s) => s.toLowerCase().includes(q)),
        )
      : productsQ.rows;
    const out: UnitRow[] = [];
    for (const med of meds) {
      const base = Number(stockByProduct.get(med.id) ?? 0n);
      for (const unit of med.units.filter((u) => u.sellable && u.active)) {
        const factor = Number(unit.factor) || 1;
        const available = Math.floor(base / factor);
        // Filter BEFORE the cap: dropping unsellable rows after slicing to 40
        // would leave the list just as short, defeating the point of hiding them.
        if (!showOutOfStock && available < 1) continue;
        const tiers = tiersForUnit(med.priceTiers, unit.id).filter((t) => t.price < unit.sellPrice);
        out.push({ med, unit, available, tiers });
        if (out.length >= MAX_ROWS) return out;
      }
    }
    return out;
  }, [query, productsQ.rows, stockByProduct, showOutOfStock]);

  useEffect(() => {
    setHighlight(0);
  }, [query, showOutOfStock]);

  const onSearchKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => Math.min(unitRows.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      // Barcode scanner: exact SKU match adds the product at its base unit.
      // Matched against the WHOLE catalog, not the filtered rows, so a scan
      // still resolves when the out-of-stock filter is hiding that product.
      const skuExact = productsQ.rows.find(
        (m) => m.sku.toLowerCase() === query.trim().toLowerCase(),
      );
      if (skuExact) {
        const baseId = skuExact.units.find((u) => u.isBase)?.id ?? "";
        void onAdd(skuExact, baseId);
        return;
      }
      const row = unitRows[highlight];
      if (row) void onAdd(row.med, row.unit.id, row.available);
    } else if (e.key === "Escape") {
      setQuery("");
    }
  };

  return {
    /**
     * The full catalog (ALL_LIMIT). Exposed because the cart panel resolves each
     * line's product for its unit selector — sharing this query keeps POS on one
     * catalog fetch rather than two.
     */
    products: productsQ.rows,
    query,
    setQuery,
    highlight,
    setHighlight,
    searchRef,
    showOutOfStock,
    onToggleOutOfStock,
    unitRows,
    stockByProduct,
    onSearchKeyDown,
    /** Clear the box and return focus — used after a successful add. */
    resetAfterAdd: useCallback(() => {
      setQuery("");
      searchRef.current?.focus();
    }, []),
  };
}
