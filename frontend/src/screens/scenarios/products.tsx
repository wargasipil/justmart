import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";

import { BatchService } from "../../gen/inventory_iface/v1/batch_connect";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { ProductDiscountService } from "../../gen/inventory_iface/v1/product_discount_connect";
import { ProductPriceTierService } from "../../gen/inventory_iface/v1/product_price_tier_connect";
import { StockMovementService } from "../../gen/inventory_iface/v1/stock_connect";
import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import { UnitService } from "../../gen/unit_iface/v1/unit_connect";
import {
  PHARMACY_CATALOG,
  RETAIL_CATALOG,
  MANUFACTURERS,
  SUPPLIERS,
  filterProducts,
  redactProductForTill,
  summarizeProducts,
  unitBasesFor,
} from "../../routes/dev/fixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import ProductDetail from "../../routes/inventory/ProductDetail";
import Products from "../../routes/inventory/Products";

// The catalog list, on the shared fixture catalog (routes/dev/fixtures.ts).
// Rendered by screens/{desktop,mobile}/pages/products/ProductList.stories.tsx — this
// module is the one place its data and states are written.
//
// ListProducts is mocked as a FUNCTION of the request, not a canned page: the
// fake applies the same predicate as the server's productFilters (archived tab,
// name/SKU search, "opname before X or never"), orders by name and pages — so
// search, the Active/Archived tabs, the opname filter and the pager all work
// live here. The summary tiles aggregate the SAME filtered set, which is the
// filter-parity rule the real GetProductsSummary is tested for.
//
// Clicking a row opens the real ProductDetail for that product (its tabs are
// mocked empty; the Product detail stories have them populated).

const DEFAULT_LIMIT = 25;

/** Everything the list reads, for either audience. */
export function listHandlers(catalog: Product[], { till = false } = {}) {
  const rows = till ? catalog.map(redactProductForTill) : catalog;
  const handlers = [
    mockRpc(ProductService, "listProducts", (req) => {
      const matched = filterProducts(rows, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { products: matched.slice(req.offset, req.offset + limit), total: matched.length };
    }),
    mockRpc(UnitService, "listUnitBases", () => {
      const bases = unitBasesFor(catalog);
      return { bases, total: bases.length };
    }),
    // Opening a row: the detail page, with the product and nothing in its tabs.
    mockRpc(ProductService, "getProduct", (req) => ({ product: rows.find((p) => p.id === req.id) })),
    mockRpc(BatchService, "listBatches", { batches: [], total: 0 }),
    // Opening a row shows the Pabrik field, which resolves one id. All four
    // roles may call this, so it sits before the till early-return.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
  ];
  if (till) return handlers;
  // Manager-only reads. Left unmocked for the till on purpose: if the page ever
  // fires one for a cashier, it hits mockApi's catch-all and fails by name.
  return [
    ...handlers,
    mockRpc(ProductService, "getProductsSummary", (req) => summarizeProducts(filterProducts(rows, req))),
    mockRpc(SupplierService, "resolveSuppliers", (req) => ({
      suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
    })),
    mockRpc(ProductPriceTierService, "listProductPriceTiers", { tiers: [], total: 0 }),
    mockRpc(ProductPriceTierService, "listProductTierPrices", { prices: [], total: 0 }),
    mockRpc(ProductService, "listProductUnitPrices", { prices: [], total: 0 }),
    mockRpc(ProductService, "listProductRestockLogs", { logs: [], total: 0 }),
    mockRpc(StockMovementService, "listMovements", { movements: [], total: 0 }),
    mockRpc(ProductDiscountService, "listProductDiscounts", { discounts: [], total: 0 }),
  ];
}

export const routes = (
  <>
    <Route path="/products" element={<Products />} />
    <Route path="/products/:id" element={<ProductDetail />} />
  </>
);

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: Products,
  parameters: {
    router: { initialEntries: ["/products"] },
    msw: mockApi(...listHandlers(RETAIL_CATALOG)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Owner: story(
    "A minimarket owner: 30 active products over two pages, the four summary tiles, and " +
      "Export / Import / Add. Try the search box, the Archived tab and \"Opname before\" — the " +
      "tiles re-aggregate with every filter. The restock column group is off by default; turn " +
      "it on from Columns.",
  ),
  Cashier: story(
    "The till's read-only view: no summary tiles (half are valuations at cost), no " +
      "Export/Import/Add, and the restock column group is gone from both the table and the " +
      "Columns popover. Fed redacted rows, like the server sends.",
    {
      parameters: {
        pageContext: { user: "cashier" },
        msw: mockApi(...listHandlers(RETAIL_CATALOG, { till: true })),
      },
    },
  ),
  Pharmacy: story("Pharmacy mode: the page is titled \"Obat\" and lists an apotek's shelf.", {
    parameters: {
      pageContext: { mode: "pharmacy" },
      msw: mockApi(...listHandlers(PHARMACY_CATALOG)),
    },
  }),
  Empty: story(
    "A fresh install with nothing in the catalog. The tiles read \"0\", not blank: that's the " +
      "formatCount-vs-formatThousands rule — a blank tile reads as \"failed to load\" when zero " +
      "is the real answer.",
    { parameters: { msw: mockApi(...listHandlers([])) } },
  ),
  Loading: story("ListProducts never answers: the table's spinner (the tiles still land).", {
    parameters: {
      msw: mockApi(
        mockRpc(ProductService, "listProducts", { products: [], total: 0 }, { delay: "infinite" }),
        ...listHandlers(RETAIL_CATALOG),
      ),
    },
  }),
  LoadFailed: story(
    "ListProducts fails. In the app the global QueryCache turns this into an error toast; the " +
      "page itself has no error state and renders \"no results\" — which is worth seeing, since " +
      "it's indistinguishable from a genuinely empty search.",
    {
      parameters: {
        msw: mockApi(
          mockRpcError(ProductService, "listProducts", Code.Unavailable, "database unavailable"),
          ...listHandlers(RETAIL_CATALOG),
        ),
      },
    },
  ),
};
