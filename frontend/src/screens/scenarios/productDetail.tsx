import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";
import { userEvent, within } from "storybook/test";

import { BatchService } from "../../gen/inventory_iface/v1/batch_connect";
import { BatchExpiryChange, ExpirySource } from "../../gen/inventory_iface/v1/batch_pb";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { ExpiryDefault, type Product } from "../../gen/inventory_iface/v1/product_pb";
import { ProductDiscountService } from "../../gen/inventory_iface/v1/product_discount_connect";
import { ProductPriceTierService } from "../../gen/inventory_iface/v1/product_price_tier_connect";
import { StockMovementService } from "../../gen/inventory_iface/v1/stock_connect";
import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import { UserService } from "../../gen/user_iface/v1/users_connect";
import {
  MANUFACTURERS,
  PHARMACY_CATALOG,
  SUPPLIERS,
  filterManufacturers,
  redactProductForTill,
} from "../../routes/dev/fixtures";
import {
  batchesFor,
  discountsFor,
  movementsFor,
  redactBatchesForTill,
  restocksFor,
  tierPricesFor,
  tiersFor,
  unitPricesFor,
} from "../../routes/dev/productDetailFixtures";
import { STORY_USERS, withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError, rpcError } from "../../routes/dev/storyMocks";
import ProductDetail from "../../routes/inventory/ProductDetail";

// One product's detail page. Unlike a component story, a page is the real
// route: it reads `:id` from the router, the signed-in role from AuthContext,
// and ~10 RPCs from the network — all of which are faked here, so it renders
// with no backend. See routes/dev/storyMocks.ts for the Connect mocks and
// withPageContext for the role / business-mode wiring.
//
// Rendered by screens/{desktop,mobile}/pages/products/ProductDetail.stories.tsx.

// Fixtures come from routes/dev/ — the catalog (fixtures.ts) the Products list
// story pages through, so a row there and this page describe one product, and
// the tab data (productDetailFixtures.ts), which the per-tab component stories
// in routes/inventory read too.
const [PARACETAMOL, AMOXICILLIN] = PHARMACY_CATALOG;

// --- handlers ---------------------------------------------------------------

/** Everything the page can read for the till: the product + its batches. */
function tillHandlers(p: Product) {
  return [
    mockRpc(ProductService, "getProduct", { product: p }),
    mockRpc(BatchService, "listBatches", () => {
      const rows = redactBatchesForTill(batchesFor(p));
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
function managerHandlers(seed: Product) {
  // The handlers below WRITE (approved sources, the edit form, lot expiry), so
  // they work on a copy: writing into the shared catalog fixture would change
  // the same product in every other scenario that renders it.
  const p = seed.clone();
  const ladder = tiersFor(p);
  // This story's own lots and their expiry corrections: the pencil on the
  // Batches tab WRITES here, so a corrected date survives the refetch instead
  // of snapping back to the fixture.
  const lots = batchesFor(p);
  const changes: BatchExpiryChange[] = [];
  return [
    mockRpc(ProductService, "getProduct", () => {
      const withLadder = p.clone();
      withLadder.priceTiers = ladder;
      return { product: withLadder };
    }),
    mockRpc(BatchService, "listBatches", () => ({ batches: lots, total: lots.length })),
    mockRpc(BatchService, "setBatchExpiry", (req) => {
      const b = lots.find((l) => l.id === req.batchId);
      if (!b) return rpcError(Code.NotFound, "batch not found");
      if (!req.reason.trim()) return rpcError(Code.InvalidArgument, "batch.reason_required");
      const none = req.expirySource === ExpirySource.NONE;
      const date = none ? "2099-12-31" : req.expiryDate;
      changes.unshift(
        new BatchExpiryChange({
          id: "chg-" + (changes.length + 1),
          batchId: b.id,
          oldExpiryDate: b.expiryDate,
          oldExpirySource: b.expirySource || ExpirySource.ENTERED,
          newExpiryDate: date,
          newExpirySource: none ? ExpirySource.NONE : ExpirySource.ENTERED,
          reason: req.reason.trim(),
          changedBy: STORY_USERS.owner.id,
          changedAt: BigInt(Math.floor(Date.now() / 1000)),
        }),
      );
      b.expiryDate = date;
      b.expirySource = none ? ExpirySource.NONE : ExpirySource.ENTERED;
      return { batch: b };
    }),
    mockRpc(BatchService, "listBatchExpiryChanges", (req) => {
      const mine = changes.filter((c) => c.batchId === req.batchId);
      return { changes: mine.slice(req.offset, req.offset + (req.limit || 5)), total: mine.length };
    }),
    mockRpc(UserService, "resolveUsers", (req) => ({
      users: Object.values(STORY_USERS)
        .filter((u) => req.ids.includes(u.id))
        .map((u) => ({ id: u.id, name: u.name, email: u.email })),
    })),
    mockRpc(SupplierService, "resolveSuppliers", (req) => ({ suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)) })),
    // The Pabrik field resolves one id. Mocked for the TILL too, on purpose:
    // ResolveManufacturers is open to all four roles (a maker's name is not
    // cost), and this story is what asserts the cashier may call it.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
    // The approved-sources card adds a picker, which searches on mount, and an
    // editor. It WRITES into this story's copy of the product for the same
    // reason the settings panels do: the mutation invalidates the product key,
    // the refetch serves the fixture back, and a canned reply would make every
    // add and remove visibly snap back as though the card were broken.
    mockRpc(ManufacturerService, "searchManufacturers", (req) => ({
      manufacturers: filterManufacturers(MANUFACTURERS, {
        query: req.query,
        includeInactive: false,
      }).slice(0, req.limit || 20),
    })),
    // The edit form's Save. Applies what the form sends -- including the expiry
    // setting -- so the info section shows the change after the refetch rather
    // than snapping back. Units are left as they are (the unit editor is not
    // what these stories are about). Mirrors the server's month bound.
    mockRpc(ProductService, "updateProduct", (req) => {
      if (
        req.expiryDefault === ExpiryDefault.MONTHS &&
        (req.expiryDefaultMonths < 1 || req.expiryDefaultMonths > 120)
      ) {
        return rpcError(Code.InvalidArgument, "product.expiry_months_invalid");
      }
      if (req.sku) p.sku = req.sku;
      p.name = req.name;
      p.unit = req.unit;
      p.unitPrice = req.unitPrice;
      p.prescriptionRequired = req.prescriptionRequired;
      p.manufacturerId = req.manufacturerId;
      p.expiryDefault =
        req.expiryDefault === ExpiryDefault.UNSPECIFIED ? ExpiryDefault.MANUAL : req.expiryDefault;
      p.expiryDefaultMonths = req.expiryDefault === ExpiryDefault.MONTHS ? req.expiryDefaultMonths : 0;
      return { product: p };
    }),
    mockRpc(ProductService, "setProductManufacturers", (req) => {
      p.manufacturerIds = req.manufacturerIds;
      // Mirrors the server's invariant rather than trusting the request: the
      // primary is always a member of its own list, and clearing the list
      // clears the pointer instead of dangling it.
      p.manufacturerId = req.manufacturerIds.includes(req.primaryManufacturerId)
        ? req.primaryManufacturerId
        : (req.manufacturerIds[0] ?? "");
      return { product: p };
    }),
    mockRpc(ProductPriceTierService, "listProductPriceTiers", { tiers: ladder, total: ladder.length }),
    mockRpc(ProductService, "listProductUnitPrices", () => {
      const prices = unitPricesFor(p);
      return { prices, total: prices.length };
    }),
    mockRpc(ProductPriceTierService, "listProductTierPrices", () => {
      const prices = tierPricesFor(p);
      return { prices, total: prices.length };
    }),
    mockRpc(ProductService, "listProductRestockLogs", () => {
      const logs = restocksFor(p);
      return { logs, total: logs.length };
    }),
    mockRpc(StockMovementService, "listMovements", () => {
      const movements = movementsFor(p);
      return { movements, total: movements.length };
    }),
    mockRpc(ProductDiscountService, "listProductDiscounts", () => {
      const discounts = discountsFor(p);
      return { discounts, total: discounts.length };
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

/**
 * A story that opens on one of the five tabs.
 *
 * The tab is local state with no URL and no prop — `Tabs.Root defaultValue`
 * decides it — so a click is the only seam there is, and every story without
 * one lands on Batches. `name` matches both locales because the toolbar's
 * locale global is a real switch and these stories should survive it.
 *
 * Each tab also has its OWN story beside its component (components/products/*),
 * where its states — empty, loading, refused — are staged. These exist for the
 * thing a component story structurally cannot show: the tab in the page, under
 * the real strip, at the width the shell leaves it.
 */
const onTab = (name: RegExp, description: string): StoryObj =>
  story(description, {
    play: async ({ canvasElement }) => {
      const canvas = within(canvasElement);
      await userEvent.click(await canvas.findByRole("tab", { name }));
    },
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

  EditExpirySetting: story(
    "The edit form, opened, to show the expiry setting: how this product's lots get their expiry " +
      "on the Receive screen. Paracetamol pre-fills 24 months after the received date (end of that " +
      "month); switch it to \"Type it in from the pack\" or \"Does not expire\" to see the " +
      "months field come and go. The help text says the uncomfortable part: use the shortest life " +
      "deliveries usually arrive with, not the factory shelf life, because a default that runs late " +
      "keeps old stock looking fresh. Tick \"Requires prescription\" in pharmacy mode and the form " +
      "warns that such a product is always typed in, whatever is chosen here.",
    {
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await userEvent.click(await canvas.findByRole("button", { name: /^(edit|ubah)$/i }));
      },
    },
  ),

  // --- the five tabs, opened in place ---------------------------------------

  TabBatches: onTab(
    /^(batch|batches)$/i,
    "The tab every story lands on, stated explicitly so the set reads as five. In-stock lots " +
      "in the ACTIVE warehouse, soonest expiry first, with the manager's supplier and cost " +
      "columns — compare the Cashier story, where the same table is three columns wide.",
  ),
  TabPriceHistory: onTab(
    /riwayat harga jual|sold price history/i,
    "Sell-price history, in two flavours behind one segmented control: the per-unit catalog " +
      "price and the grosir ladder. They are separate tables keyed differently — a rung is " +
      "(unit, min_qty) — so each owns its own pager rather than sharing one.",
  ),
  TabRestockLog: onTab(
    /riwayat harga restok|restock price history/i,
    "What the shop PAID, to whom, per arrival — the append-only restock log. The price drifts " +
      "across the six rows and the two suppliers price differently, so \"is this one getting " +
      "dearer\" is a question the table can actually answer.",
  ),
  TabMovements: onTab(
    /mutasi|recent movements/i,
    "The stock ledger for this product in the active warehouse, one row per movement type the " +
      "column labels (sale · purchase · adjustment · write-off). Quantities are signed and the " +
      "reason column carries the document that caused it.",
  ),
  TabDiscounts: onTab(
    /^(diskon|discounts)$/i,
    "The one tab that WRITES: discount rules with Add / edit / delete. One rule is live and one " +
      "lapsed three days ago — the red Kedaluwarsa badge is the point, since an expired rule " +
      "stays listed rather than vanishing. The mutations are not mocked here; the tab's own " +
      "story (components/products/ProductDiscountTab) stages those over a shop that answers.",
  ),
};
