import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";

import { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import { BatchService } from "../../gen/inventory_iface/v1/batch_connect";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { ProductPriceTier } from "../../gen/inventory_iface/v1/product_price_tier_pb";
import { ProductDiscountService } from "../../gen/inventory_iface/v1/product_discount_connect";
import { ProductPriceTierService } from "../../gen/inventory_iface/v1/product_price_tier_connect";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { StockMovementService } from "../../gen/inventory_iface/v1/stock_connect";
import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import { MANUFACTURERS, PHARMACY_CATALOG, SUPPLIERS, dateIn, daysAgo, redactProductForTill, restockLogsFor } from "../../routes/dev/fixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import ProductDetail from "../../routes/inventory/ProductDetail";

// One product's detail page. Unlike a component story, a page is the real
// route: it reads `:id` from the router, the signed-in role from AuthContext,
// and ~10 RPCs from the network — all of which are faked here, so it renders
// with no backend. See routes/dev/storyMocks.ts for the Connect mocks and
// withPageContext for the role / business-mode wiring.
//
// Rendered by screens/{desktop,mobile}/pages/products/ProductDetail.stories.tsx.

// Fixtures come from routes/dev/fixtures.ts — the same catalog the Products
// list story pages through, so a row there and this page describe one product.
const [PARACETAMOL, AMOXICILLIN] = PHARMACY_CATALOG;
const [SUPPLIER, SUPPLIER_2] = SUPPLIERS;

// Unit ids are `${productId}-u1|u2|u3` (base · strip · box) — see unitsFor.
const tiers = (p: Product) => [
  { id: "tier-1", productId: p.id, productUnitId: `${p.id}-u2`, unitName: "strip", unitFactor: 10n, minQty: 5, price: (p.units[1].sellPrice * 95n) / 100n, createdAt: daysAgo(40) },
  { id: "tier-2", productId: p.id, productUnitId: `${p.id}-u2`, unitName: "strip", unitFactor: 10n, minQty: 20, price: (p.units[1].sellPrice * 90n) / 100n, createdAt: daysAgo(40) },
  { id: "tier-3", productId: p.id, productUnitId: `${p.id}-u3`, unitName: "box", unitFactor: 100n, minQty: 3, price: (p.units[2].sellPrice * 94n) / 100n, createdAt: daysAgo(20) },
];

const batches = (p: Product) => [
  new Batch({ id: "b-1", productId: p.id, supplierId: SUPPLIER_2.id, batchNumber: `${p.sku.slice(0, 3)}2409A`, expiryDate: dateIn(21), costPrice: p.referenceCost - 20n, receivedAt: dateIn(-150), currentQuantity: 140n, productName: p.name }),
  new Batch({ id: "b-2", productId: p.id, supplierId: SUPPLIER.id, batchNumber: `${p.sku.slice(0, 3)}2503C`, expiryDate: dateIn(240), costPrice: p.referenceCost - 10n, receivedAt: dateIn(-60), currentQuantity: 600n, productName: p.name }),
  new Batch({ id: "b-3", productId: p.id, supplierId: SUPPLIER.id, batchNumber: `${p.sku.slice(0, 3)}2508B`, expiryDate: dateIn(640), costPrice: p.referenceCost, receivedAt: dateIn(-12), currentQuantity: 500n, productName: p.name }),
];

// --- handlers ---------------------------------------------------------------

/** Everything the page can read for the till: the product + its batches. */
function tillHandlers(p: Product) {
  return [
    mockRpc(ProductService, "getProduct", { product: p }),
    mockRpc(BatchService, "listBatches", () => {
      const rows = batches(p).map((b) => {
        b.supplierId = "";
        b.costPrice = 0n;
        return b;
      });
      return { batches: rows, total: rows.length };
    }),
    // The Pabrik field resolves one id. Mocked for the TILL too, on purpose:
    // ResolveManufacturers is open to all four roles (a maker's name is not
    // cost), and this story is what asserts the cashier may call it.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
  ];
}

/** The manager view adds the grosir card and the four cost-bearing tabs. */
function managerHandlers(p: Product) {
  const ladder = tiers(p);
  return [
    mockRpc(ProductService, "getProduct", () => {
      const withLadder = p.clone();
      withLadder.priceTiers = ladder.map((t) => new ProductPriceTier(t));
      return { product: withLadder };
    }),
    mockRpc(BatchService, "listBatches", () => {
      const rows = batches(p);
      return { batches: rows, total: rows.length };
    }),
    mockRpc(SupplierService, "resolveSuppliers", (req) => ({ suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)) })),
    // The Pabrik field resolves one id. Mocked for the TILL too, on purpose:
    // ResolveManufacturers is open to all four roles (a maker's name is not
    // cost), and this story is what asserts the cashier may call it.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
    mockRpc(ProductPriceTierService, "listProductPriceTiers", { tiers: ladder, total: ladder.length }),
    mockRpc(ProductService, "listProductUnitPrices", {
      prices: [
        { id: "up-1", productUnitId: `${p.id}-u3`, unitName: "box", unitSellPrice: p.units[2].sellPrice, effectiveFrom: daysAgo(35) },
        { id: "up-2", productUnitId: `${p.id}-u3`, unitName: "box", unitSellPrice: (p.units[2].sellPrice * 95n) / 100n, effectiveFrom: daysAgo(200), effectiveTo: daysAgo(35) },
        { id: "up-3", productUnitId: `${p.id}-u2`, unitName: "strip", unitSellPrice: p.units[1].sellPrice, effectiveFrom: daysAgo(200) },
        { id: "up-4", productUnitId: `${p.id}-u1`, unitName: p.unit, unitSellPrice: p.unitPrice, effectiveFrom: daysAgo(200) },
      ],
      total: 4,
    }),
    mockRpc(ProductPriceTierService, "listProductTierPrices", {
      // Newest first, like the server. The ≥5 strip rung was re-priced once.
      prices: [
        { id: "tp-1", productId: p.id, productUnitId: `${p.id}-u3`, unitName: "box", minQty: 3, price: ladder[2].price, effectiveFrom: daysAgo(20) },
        { id: "tp-2", productId: p.id, productUnitId: `${p.id}-u2`, unitName: "strip", minQty: 20, price: ladder[1].price, effectiveFrom: daysAgo(40) },
        { id: "tp-3", productId: p.id, productUnitId: `${p.id}-u2`, unitName: "strip", minQty: 5, price: ladder[0].price, effectiveFrom: daysAgo(15) },
        { id: "tp-4", productId: p.id, productUnitId: `${p.id}-u2`, unitName: "strip", minQty: 5, price: (ladder[0].price * 102n) / 100n, effectiveFrom: daysAgo(40), effectiveTo: daysAgo(15) },
      ],
      total: 4,
    }),
    mockRpc(ProductService, "listProductRestockLogs", () => {
      const logs = restockLogsFor(p, 6);
      return { logs, total: logs.length };
    }),
    mockRpc(StockMovementService, "listMovements", {
      movements: [
        { id: "m-1", batchId: "b-2", qty: -20, type: MovementType.SALE, reason: "INV-2026-0412", createdAt: daysAgo(0) },
        { id: "m-2", batchId: "b-3", qty: 500, type: MovementType.PURCHASE, reason: "RCV-2026-0088", createdAt: daysAgo(12) },
        { id: "m-3", batchId: "b-1", qty: -4, type: MovementType.ADJUSTMENT, reason: "Stocktake: Opname Agustus — selisih", createdAt: daysAgo(30) },
        { id: "m-4", batchId: "b-1", qty: -6, type: MovementType.WRITE_OFF, reason: "Kemasan rusak", createdAt: daysAgo(44) },
      ],
      total: 4,
    }),
    mockRpc(ProductDiscountService, "listProductDiscounts", {
      discounts: [
        { id: "d-1", productId: p.id, discountType: "PERCENT", perItem: true, value: 1_000n, minQty: 0, expiresAt: dateIn(10), createdAt: daysAgo(5) },
        { id: "d-2", productId: p.id, discountType: "FIXED", perItem: false, value: 2_000n, minQty: 100, minQtyUnitName: p.unit, minQtyUnitFactor: 1n, expiresAt: dateIn(-3), createdAt: daysAgo(60) },
      ],
      total: 2,
    }),
  ];
}

// --- stories ----------------------------------------------------------------

export const routes = <Route path="/products/:id" element={<ProductDetail />} />;

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: ProductDetail,
  parameters: {
    router: { initialEntries: [`/products/${PARACETAMOL.id}`] },
    msw: mockApi(...managerHandlers(PARACETAMOL)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Owner: story(
    "The manager view: cost fields, stock valuations, margin per unit, the grosir ladder, and " +
      "all five tabs (batches · price history · restock log · movements · discounts).",
  ),
  Cashier: story(
    "The till's read-only view — what a cashier sees when looking up a price or an expiry. No " +
      "cost anywhere, no Edit/Archive, no grosir card, and only the Batches tab (without " +
      "supplier or cost columns); Print barcode stays. Only the two RPCs the till may call are " +
      "mocked: if the page ever fires a manager-only read for a cashier, it hits the catch-all " +
      "and fails by name in the console — so this story doubles as a check on the " +
      "cost-visibility rule.",
    {
      parameters: {
        pageContext: { user: "cashier" },
        msw: mockApi(...tillHandlers(redactProductForTill(PARACETAMOL))),
      },
    },
  ),
  Pharmacy: story(
    "Pharmacy mode: the catalog noun swaps to \"Obat\" (page description and the breadcrumb " +
      "trail). The product is Rx-required — open Edit to see the \"Requires prescription\" " +
      "switch, which retail mode hides.",
    {
      parameters: {
        pageContext: { mode: "pharmacy" },
        router: { initialEntries: [`/products/${AMOXICILLIN.id}`] },
        msw: mockApi(...managerHandlers(AMOXICILLIN)),
      },
    },
  ),
  Loading: story("GetProduct never answers: the page's loading spinner.", {
    parameters: {
      msw: mockApi(mockRpc(ProductService, "getProduct", { product: PARACETAMOL }, { delay: "infinite" })),
    },
  }),
  NotFound: story(
    "GetProduct fails NotFound (a deleted or mistyped id): the empty state, and no crumb leaf.",
    {
      parameters: {
        msw: mockApi(mockRpcError(ProductService, "getProduct", Code.NotFound, "product not found")),
      },
    },
  ),
};
