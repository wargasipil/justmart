import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";

import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import {
  MANUFACTURERS,
  PHARMACY_CATALOG,
  SUPPLIERS,
  restockLogsFor,
} from "../dev/fixtures";
import { restocksFor } from "../dev/productDetailFixtures";
import { mockApi, mockRpc, mockRpcError } from "../dev/storyMocks";
import ProductRestockTab from "./ProductRestockTab";

// The "Riwayat Harga Restok" tab: what the shop PAID for this product and to
// whom, straight off the append-only restock log.
//
// Two columns that look alike and are not: PEMASOK is who sold this delivery,
// PABRIK is who made it. A shop buying one generic from several factories can
// only read that off here, because the order line is where the fact was
// captured and this log is its per-product history.
//
// This is the most cost-bearing surface on the page — price per base unit,
// supplier, the discount the distributor gave — so it is manager-only twice
// over: ListProductRestockLogs and ResolveSuppliers are both OWNER+PHARMACIST.
// There is deliberately no till story; ProductDetail never mounts it for one.
//
// Rendered by ProductRestockTab.stories.tsx
// ("components/products/ProductRestockTab").

const PRODUCT = PHARMACY_CATALOG[0];

const restockLogs = (rows = restocksFor(PRODUCT)) =>
  mockRpc(ProductService, "listProductRestockLogs", (req) => ({
    logs: rows.slice(req.offset, req.offset + (req.limit || 25)),
    total: rows.length,
  }));

const resolveSuppliers = mockRpc(SupplierService, "resolveSuppliers", (req) => ({
  suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
}));

// Resolve-by-IDs for the Pabrik column. Includes ARCHIVED rows, like the server:
// a delivery made by a factory the shop has since retired must still print a
// name, or history would start blanking itself out.
const resolveManufacturers = mockRpc(
  ManufacturerService,
  "resolveManufacturers",
  (req) => ({ manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)) }),
);

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: ProductRestockTab,
  args: { productId: PRODUCT.id },
  parameters: { msw: mockApi(restockLogs(), resolveSuppliers, resolveManufacturers) },
};

type Story = StoryObj<typeof ProductRestockTab>;

const story = (description: string, s: Story = {}): Story => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Manager: story(
    "Six arrivals, newest first, about twelve days apart. The price drifts — each step back is " +
      "a little cheaper — and the three suppliers price differently, so \"is this one getting " +
      "dearer, and who is cheapest\" is a question the table can actually answer rather than " +
      "six identical rows. The discount column rotates through all four shapes the log " +
      "records: none, percent per item, a flat amount on the line, and percent on the line.",
  ),
  MultipleManufacturers: story(
    "The same product, the same distributor, different FACTORIES — the state the old single " +
      "manufacturer column on the product could not record at all, and the reason the maker " +
      "moved onto the order line and then onto the lot. Pemasok and Pabrik disagree row by row " +
      "here, which is exactly the point: who sold it and who made it are separate questions, " +
      "and only this table answers the second one over time. The two oldest arrivals show a " +
      "dash — they predate the column, and that blank is permanent: filling it in from the " +
      "product's usual maker would invent provenance.",
  ),
  SingleSupplier: story(
    "One distributor, every time. The supplier column stops being a comparison and becomes a " +
      "constant — which is the honest look of most products in a small shop, and the reason " +
      "the price trend beside it is the column that carries the story.",
    { parameters: { msw: mockApi(restockLogs(restockLogsFor(PRODUCT, 5, [SUPPLIERS[0]])), resolveSuppliers, resolveManufacturers) } },
  ),
  Paged: story(
    "A fast-moving product with 40 arrivals behind it: the pager under the table, 25 to a page, " +
      "and the rows scrolling inside TableScroll instead of stretching the tab panel.",
    { parameters: { msw: mockApi(restockLogs(restockLogsFor(PRODUCT, 40)), resolveSuppliers, resolveManufacturers) } },
  ),
  Empty: story(
    "Never restocked — a product added by hand, or one whose stock arrived before the log " +
      "existed. The tab drops the table entirely for a one-line message rather than showing " +
      "empty headers, since a header row with nothing under it reads as a failed load.",
    { parameters: { msw: mockApi(restockLogs([])) } },
  ),
  Loading: story("ListProductRestockLogs never answers: the same one-line empty state, then rows.", {
    parameters: {
      msw: mockApi(
        mockRpc(ProductService, "listProductRestockLogs", { logs: [], total: 0 }, { delay: "infinite" }),
      ),
    },
  }),
  LoadFailed: story(
    "The read fails: the empty message, and the toast from the global QueryCache. Same shape " +
      "as Empty on screen — which is why the toast is the part doing the work here.",
    {
      parameters: {
        msw: mockApi(
          mockRpcError(ProductService, "listProductRestockLogs", Code.Unavailable, "backend down"),
        ),
      },
    },
  ),
};
