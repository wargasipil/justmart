import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";

import { BatchService } from "../../gen/inventory_iface/v1/batch_connect";
import type { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import { PHARMACY_CATALOG, SUPPLIERS, dateIn } from "../dev/fixtures";
import { batchesFor } from "../dev/productDetailFixtures";
import { mockApi, mockRpc, mockRpcError } from "../dev/storyMocks";
import ProductBatchesTab from "./ProductBatchesTab";

// The Batch tab of the product detail page, on its own: in-stock lots for this
// product in the ACTIVE warehouse, soonest expiry first.
//
// Every role reads it — cost included, since cost visibility was widened to the
// till (lib/roles.ts canSeeCost). What differs is the one WRITE it offers: the
// expiry correction (SetBatchExpiry, OWNER+PHARMACIST), which hangs off the
// `canManage` prop (canManageProducts). So the Manager/Till pair below is the
// write gate rendered twice, not two different views of cost.
//
// Data comes from dev/productDetailFixtures, the same rows the whole-page
// stories use (screens/**/ProductDetail), so this tab and the tab inside the
// page describe one product.
//
// Rendered by ProductBatchesTab.stories.tsx ("components/products/ProductBatchesTab").

const PRODUCT = PHARMACY_CATALOG[0];

// The column headers are a SERVER sort (ListBatches sort_by / sort_desc), so the
// mock re-orders the rows the way backend batch/list_batches.go does: its
// whitelist of columns, soonest expiry first by default, ties broken by id.
const SORT_KEY: Record<string, (b: Batch) => string | number> = {
  batch_number: (b) => b.batchNumber,
  supplier: (b) => SUPPLIERS.find((s) => s.id === b.supplierId)?.name ?? "",
  expiry: (b) => b.expiryDate,
  cost: (b) => Number(b.costPrice),
  qty: (b) => Number(b.currentQuantity),
};
const compare = (a: string | number, b: string | number) =>
  typeof a === "number" && typeof b === "number" ? a - b : String(a).localeCompare(String(b));

const sortLots = (rows: Batch[], sortBy: string, desc: boolean) => {
  const key = SORT_KEY[sortBy] ?? SORT_KEY.expiry;
  const dir = sortBy && desc ? -1 : 1;
  return [...rows].sort((a, b) => compare(key(a), key(b)) * dir || compare(a.id, b.id));
};

const listBatches = (rows = batchesFor(PRODUCT)) =>
  mockRpc(BatchService, "listBatches", (req) => ({
    batches: sortLots(rows, req.sortBy, req.sortDesc).slice(req.offset, req.offset + (req.limit || 25)),
    total: rows.length,
  }));

// Open to every role (it resolves ids the caller already holds). With showCost
// off — cost visibility re-narrowed — the tab passes an empty id list and the
// hook never fires.
const resolveSuppliers = mockRpc(SupplierService, "resolveSuppliers", (req) => ({
  suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
}));

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: ProductBatchesTab,
  args: { productId: PRODUCT.id, showCost: true, canManage: true },
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
    "The same tab for a cashier: the same five columns, supplier and cost included — the till " +
      "receives the manager's figures (batch.redactCost lets every role through). What is gone " +
      "is the expiry pencil: correcting a lot's expiry is SetBatchExpiry, OWNER+PHARMACIST.",
    { args: { canManage: false } },
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
