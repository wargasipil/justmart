import type { HttpHandler } from "msw";

import { ProductDiscountService } from "../../gen/inventory_iface/v1/product_discount_connect";
import { ProductDiscount } from "../../gen/inventory_iface/v1/product_discount_pb";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { discountsFor } from "../dev/productDetailFixtures";
import { mockRpc } from "../dev/storyMocks";

// A tiny in-memory discount table for the ProductDiscountTab stories, in the
// same spirit as screens/scenarios/{settingsShop,restockShop}.
//
// The tab is the ONE product-detail tab that writes, and its mutations
// invalidate the list key. Against a canned read that makes every write look
// broken: you delete a rule, the refetch serves the same fixture back, and the
// row you just removed is still sitting there. So the mocks hold the rows and
// answer from them — Add lands a row, Edit changes one, Delete removes one.
//
// It mirrors the two rules the server actually applies rather than waving
// writes through, because those are what the drawer's output means:
// PERCENT values arrive in basis points, and the min-qty unit is stored
// denormalized (name + factor) so the table can render "≥N box" without a
// second read.

export type DiscountShop = { rows: ProductDiscount[] };

/** A fresh table, seeded with the same two rules the page stories show. */
export function newDiscountShop(product: Product, seed = discountsFor(product)): DiscountShop {
  return { rows: seed.map((d) => d.clone()) };
}

/**
 * Everything the tab and its drawer read and write, over one shop.
 *
 * Every story calls this for its OWN copy: a rule deleted while reading one
 * story must not be missing from the next.
 */
export function discountShopHandlers(product: Product, shop: DiscountShop): HttpHandler[] {
  // The drawer's min-qty picker reads the product's units, so the same GetProduct
  // the page would serve has to be here for the drawer to open complete.
  const unitOf = (id: string) => product.units.find((u) => u.id === id);

  return [
    mockRpc(ProductService, "getProduct", { product }),
    mockRpc(ProductDiscountService, "listProductDiscounts", (req) => ({
      discounts: shop.rows.slice(req.offset, req.offset + (req.limit || 25)),
      total: shop.rows.length,
    })),
    mockRpc(ProductDiscountService, "createProductDiscount", (req) => {
      const u = unitOf(req.minQtyUnitId);
      const row = new ProductDiscount({
        id: `d-${shop.rows.length + 1}-${Date.now()}`,
        productId: product.id,
        discountType: req.discountType,
        perItem: req.perItem,
        value: req.value,
        minQty: req.minQty,
        minQtyUnitId: req.minQtyUnitId,
        minQtyUnitName: u?.name ?? "",
        minQtyUnitFactor: u?.factor ?? 1n,
        expiresAt: req.expiresAt,
        createdAt: BigInt(Math.floor(Date.now() / 1000)),
      });
      shop.rows = [row, ...shop.rows];
      return { discount: row };
    }),
    mockRpc(ProductDiscountService, "updateProductDiscount", (req) => {
      const i = shop.rows.findIndex((d) => d.id === req.id);
      if (i < 0) return { discount: undefined };
      const u = unitOf(req.minQtyUnitId);
      const row = shop.rows[i].clone();
      row.discountType = req.discountType;
      row.perItem = req.perItem;
      row.value = req.value;
      row.minQty = req.minQty;
      row.minQtyUnitId = req.minQtyUnitId;
      row.minQtyUnitName = u?.name ?? "";
      row.minQtyUnitFactor = u?.factor ?? 1n;
      row.expiresAt = req.expiresAt;
      shop.rows = shop.rows.map((d, j) => (j === i ? row : d));
      return { discount: row };
    }),
    mockRpc(ProductDiscountService, "deleteProductDiscount", (req) => {
      shop.rows = shop.rows.filter((d) => d.id !== req.id);
      return {};
    }),
  ];
}
