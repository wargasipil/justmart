import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";

import { BatchService } from "../../gen/inventory_iface/v1/batch_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import { PHARMACY_CATALOG, SUPPLIERS, dateIn } from "../dev/fixtures";
import { batchesFor, redactBatchesForTill } from "../dev/productDetailFixtures";
import { mockApi, mockRpc, mockRpcError } from "../dev/storyMocks";
import ProductBatchesTab from "./ProductBatchesTab";

// The Batch tab of the product detail page, on its own: in-stock lots for this
// product in the ACTIVE warehouse, soonest expiry first.
//
// It is the only one of the five tabs the TILL also sees, which is why it takes
// a `showCost` prop and the others do not — so the pair of stories below is not
// two skins of a table, it is the cost-visibility rule rendered twice.
//
// Data comes from dev/productDetailFixtures, the same rows the whole-page
// stories use (screens/**/ProductDetail), so this tab and the tab inside the
// page describe one product.
//
// Rendered by ProductBatchesTab.stories.tsx ("components/products/ProductBatchesTab").

const PRODUCT = PHARMACY_CATALOG[0];

const listBatches = (rows = batchesFor(PRODUCT)) =>
  mockRpc(BatchService, "listBatches", (req) => ({
    batches: rows.slice(req.offset, req.offset + (req.limit || 25)),
    total: rows.length,
  }));

// Manager-only, and the tab knows it: with showCost off it passes an empty id
// list and the hook never fires, so this handler is unreachable from the till
// story on purpose.
const resolveSuppliers = mockRpc(SupplierService, "resolveSuppliers", (req) => ({
  suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
}));

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: ProductBatchesTab,
  args: { productId: PRODUCT.id, showCost: true },
  parameters: { msw: mockApi(listBatches(), resolveSuppliers) },
};

type Story = StoryObj<typeof ProductBatchesTab>;

const story = (description: string, s: Story = {}): Story => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Manager: story(
    "Five columns: lot number, supplier, expiry, cost and quantity. The three lots sit at " +
      "different distances from their expiry date, so ExpiryBadge shows each of its states — " +
      "and the costs climb with each arrival, ending at the product's reference cost (the " +
      "newest batch IS what \"last cost\" means on the page above).",
  ),
  Till: story(
    "The same tab for a cashier: three columns. Supplier and cost are gone, and not merely " +
      "hidden — the server blanks both fields before answering (batch.redactCost), so these " +
      "rows are what a till genuinely receives. ResolveSuppliers is manager-only, and the tab " +
      "passes an empty id list rather than firing it and getting refused.",
    {
      args: { showCost: false },
      parameters: { msw: mockApi(listBatches(redactBatchesForTill(batchesFor(PRODUCT)))) },
    },
  ),
  Expiring: story(
    "Every lot inside the warning window, one of them already past it. Worth pinning on its " +
      "own because this is the state the tab exists for — a pharmacist opens it to find what " +
      "must be pulled off the shelf — and the badge is the only thing marking it.",
    {
      parameters: {
        msw: mockApi(
          listBatches(
            batchesFor(PRODUCT).map((b, i) => {
              const r = b.clone();
              r.expiryDate = dateIn([-6, 9, 25][i]);
              return r;
            }),
          ),
          resolveSuppliers,
        ),
      },
    },
  ),
  Paged: story(
    "Thirty lots — a product restocked in small, frequent deliveries. The pager appears under " +
      "the table (25 a page), and the table scrolls inside its own TableScroll rather than " +
      "growing the panel, which is what keeps the tab strip above it reachable.",
    {
      parameters: {
        msw: mockApi(
          listBatches(
            Array.from({ length: 30 }, (_, i) => {
              const b = batchesFor(PRODUCT)[i % 3].clone();
              b.id = `${PRODUCT.id}-bp${i}`;
              b.batchNumber = `PCT24${String(i + 10).padStart(2, "0")}X`;
              b.expiryDate = dateIn(30 + i * 20);
              b.currentQuantity = BigInt((i % 7) + 1) * 40n;
              return b;
            }),
          ),
          resolveSuppliers,
        ),
      },
    },
  ),
  Empty: story(
    "Nothing in stock here. The lot may exist in another warehouse — ListBatches is scoped to " +
      "the active one and asks for only_in_stock — so the honest reading is \"not on this " +
      "shelf\", not \"never bought\". The pager stays, reporting 0.",
    { parameters: { msw: mockApi(listBatches([])) } },
  ),
  Loading: story("ListBatches never answers: the empty table under a working pager.", {
    parameters: {
      msw: mockApi(mockRpc(BatchService, "listBatches", { batches: [], total: 0 }, { delay: "infinite" })),
    },
  }),
  LoadFailed: story(
    "ListBatches fails. The tab has no error branch of its own — it renders as empty and the " +
      "global QueryCache raises the toast. That is the app-wide rule working, and also the " +
      "reason an empty table here is ambiguous on its own.",
    {
      parameters: {
        msw: mockApi(mockRpcError(BatchService, "listBatches", Code.Unavailable, "backend down")),
      },
    },
  ),
};
