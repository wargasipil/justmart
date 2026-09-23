import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Navigate, Route } from "react-router-dom";

import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import type { PurchaseOrder } from "../../gen/purchasing_iface/v1/order_pb";
import { PurchaseOrderService } from "../../gen/purchasing_iface/v1/order_connect";
import { PurchasePaymentService } from "../../gen/purchasing_iface/v1/payment_connect";
import {
  MANUFACTURERS,
  SUPPLIERS,
  filterManufacturers,
} from "../../routes/dev/fixtures";
import {
  PURCHASE_ORDERS,
  filterPurchaseOrders,
  filterSuppliers,
  summarizePurchaseOrders,
  supplierBalances,
} from "../../routes/dev/restockFixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import PurchaseOrderDetail from "../../routes/purchasing/PurchaseOrderDetail";
import PurchaseOrdersList from "../../routes/purchasing/PurchaseOrdersList";
import Purchasing from "../../routes/purchasing/Purchasing";
import SuppliersLedger from "../../routes/purchasing/SuppliersLedger";
import { shopHandlers } from "./restockShop";

// Restock (/purchasing) — the order list under its status tabs, the stat row
// above them, and the suppliers ledger. Rendered by
// screens/{desktop,mobile}/pages/purchasing/RestockList.stories.tsx; this module
// is the one place its data and states are written.
//
// ListPurchaseOrders is mocked as a FUNCTION of the request rather than as a
// canned page, so the whole toolbar works live: the status tabs, the search
// box, the supplier picker, the created-vs-received date filter, the
// outstanding toggle and the pager. GetPurchaseOrdersSummary aggregates the
// SAME filtered set — that is the filter-parity rule the two handlers share
// applyPOFilters for, and mocking them from one predicate is what lets a story
// demonstrate it instead of merely claiming it.
//
// There is no cashier story: /purchasing is OWNER + PHARMACIST in the proto and
// the sidebar never offers it to a till, so a cashier story would stage a page
// the app does not route them to.

const DEFAULT_LIMIT = 25;

/** Everything the list, its stat row and its toolbar read. */
export function listHandlers(rows: PurchaseOrder[]) {
  return [
    mockRpc(PurchaseOrderService, "listPurchaseOrders", (req) => {
      const matched = filterPurchaseOrders(rows, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { orders: matched.slice(req.offset, req.offset + limit), total: matched.length };
    }),
    mockRpc(PurchaseOrderService, "getPurchaseOrdersSummary", (req) =>
      summarizePurchaseOrders(filterPurchaseOrders(rows, req)),
    ),
    // Supplier names for the page's rows (resolve-by-IDs), and the toolbar's
    // picker, which is a server search rather than a preloaded list.
    mockRpc(SupplierService, "resolveSuppliers", (req) => ({
      suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
    })),
    mockRpc(SupplierService, "searchSuppliers", (req) => ({
      suppliers: filterSuppliers(req.query).slice(0, req.limit || 20),
    })),
    // The Pabrik filter beside the pemasok one. Same two RPCs, the same
    // asymmetry the server has: search is active-only (you cannot filter by a
    // factory the shop retired), resolve includes archived rows so an order
    // that already names one still renders its name.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
    mockRpc(ManufacturerService, "searchManufacturers", (req) => ({
      manufacturers: filterManufacturers(MANUFACTURERS, {
        query: req.query,
        includeInactive: false,
      }).slice(0, req.limit || 20),
    })),
    // The Pemasok tab.
    mockRpc(PurchasePaymentService, "getSupplierBalances", (req) => ({
      balances: supplierBalances(req.onlyOutstanding),
    })),
    // Product names, for a row opened into the detail page.
    mockRpc(ProductService, "resolveProducts", (req) => ({
      products: PURCHASE_ORDERS.flatMap((po) => po.items)
        .filter((it) => req.ids.includes(it.productId))
        .map((it) => ({ id: it.productId, name: it.productName, sku: it.productSku })),
    })),
  ];
}

export const routes = (
  <Route path="/purchasing" element={<Purchasing />}>
    <Route index element={<Navigate to="all" replace />} />
    {/* One route per status, exactly as main.tsx registers them: the active
        status is derived from the URL segment in the Purchasing shell and
        shared with the stat row through the filter context. */}
    <Route path="all" element={<PurchaseOrdersList />} />
    <Route path="draft" element={<PurchaseOrdersList />} />
    <Route path="sent" element={<PurchaseOrdersList />} />
    <Route path="partial" element={<PurchaseOrdersList />} />
    <Route path="received" element={<PurchaseOrdersList />} />
    <Route path="closed" element={<PurchaseOrdersList />} />
    <Route path="voided" element={<PurchaseOrdersList />} />
    <Route path="suppliers" element={<SuppliersLedger />} />
    {/* Clicking a row opens the REAL detail page, so the list story is also the
        check that the two agree. Its own states live in
        scenarios/purchaseOrderDetail.tsx. */}
    <Route path=":id" element={<PurchaseOrderDetail />} />
  </Route>
);

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: PurchaseOrdersList,
  parameters: {
    router: { initialEntries: ["/purchasing/all"] },
    // The detail page a row opens is served by the same writeable ledger the
    // detail stories use, so clicking through from the list lands on a working
    // order rather than on an Unimplemented catch-all.
    msw: mockApi(...listHandlers(PURCHASE_ORDERS), ...shopHandlers()),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Owner: story(
    "28 restock orders over two pages. The whole toolbar is live: search by PO number, supplier " +
      "or product; filter to one pemasok; pick \"Dibuat\" or \"Diterima\" in the date picker " +
      "(\"Any date\" is its off state and sends no bounds at all); flip \"Outstanding\" to hide " +
      "what is settled. Every one of those re-counts the four tiles as well as the table, " +
      "because the stat row sends the same filter object the list does. Click a row to open the " +
      "real order.",
  ),
  Admin: story(
    "The same page for the manager tier (PHARMACIST, shown as \"Admin\"). Restock is manager " +
      "work end to end — costs, supplier balances, what the shop owes — so the two roles that " +
      "can reach it see the same thing, and a cashier is not routed here at all.",
    { parameters: { pageContext: { user: "admin" } } },
  ),
  Sent: story(
    "The \"Terkirim\" tab: orders placed and not yet delivered — the working queue a shop opens " +
      "this page for. Each tab is its own URL, so it deep-links, and the tiles above summarize " +
      "that tab rather than the whole ledger.",
    { parameters: { router: { initialEntries: ["/purchasing/sent"] } } },
  ),
  Voided: story(
    "The \"Batal\" tab. A voided order keeps its PO number and its lines — the number is already " +
      "burned from the counter, so hiding the row would leave an unexplained gap — and it is " +
      "excluded from the outstanding filter and from every supplier balance.",
    { parameters: { router: { initialEntries: ["/purchasing/voided"] } } },
  ),
  ByManufacturer: story(
    "The Pabrik filter, live. Pick a factory in the toolbar and the table narrows to orders " +
      "containing at least one line from it — a LINE-level match, unlike Pemasok beside it, " +
      "which is a property of the order as a whole. That is the question a recall asks: not " +
      "who sold us this, but which orders brought in goods this factory made. The four " +
      "tiles re-count with it, because the stat row sends the same filter object the list " +
      "does and the server runs both through one shared predicate; the fixture mirrors that " +
      "predicate rather than hard-coding a result, so the filter genuinely works here. Where " +
      "it LANDS is the batches page: an order records the pabrik a line was bought from, and " +
      "receiving stamps it onto the lot, which is the only row that records who made " +
      "physical stock. Try PT Kimia Farma or PT SOHO rather than PT Kalbe Farma: this shop, " +
      "like most, has one dominant maker whose goods are in nearly every order, so picking " +
      "the big one narrows almost nothing and reads as a filter that does not work.",
  ),
  SuppliersLedger: story(
    "The Pemasok tab: what the shop owes each distributor, aggregated server-side over all " +
      "non-voided orders. It defaults to showing only suppliers with something outstanding — " +
      "turn the switch off to see the settled ones at zero. The figures are derived from the " +
      "same orders the list pages through, so the two tabs cannot disagree.",
    { parameters: { router: { initialEntries: ["/purchasing/suppliers"] } } },
  ),
  Empty: story(
    "A shop that has never ordered anything. The table falls through to the generic \"no " +
      "results\" cell, and the tiles read 0 rather than blank — a blank tile reads as \"failed " +
      "to load\" when zero is the real answer (the formatCount-vs-formatThousands rule; the " +
      "three count tiles here still use formatThousands, so they blank at zero — visible in " +
      "this story, and the reason it exists).",
    { parameters: { msw: mockApi(...listHandlers([])) } },
  ),
  Loading: story(
    "ListPurchaseOrders never answers: the table's spinner. The tiles still land, which is the " +
      "point of them being a separate query rather than a sum of the rows below.",
    {
      parameters: {
        msw: mockApi(
          mockRpc(
            PurchaseOrderService,
            "listPurchaseOrders",
            { orders: [], total: 0 },
            { delay: "infinite" },
          ),
          ...listHandlers(PURCHASE_ORDERS),
        ),
      },
    },
  ),
  LoadFailed: story(
    "ListPurchaseOrders fails. The global QueryCache turns this into an error toast; the page " +
      "itself has no error state and falls through to the empty cell, which is worth seeing " +
      "precisely because it is indistinguishable from a shop with no orders.",
    {
      parameters: {
        msw: mockApi(
          mockRpcError(
            PurchaseOrderService,
            "listPurchaseOrders",
            Code.Unavailable,
            "database unavailable",
          ),
          ...listHandlers(PURCHASE_ORDERS),
        ),
      },
    },
  ),
};
