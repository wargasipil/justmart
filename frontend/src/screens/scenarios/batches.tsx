import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";

import { BatchService } from "../../gen/inventory_iface/v1/batch_connect";
import type { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import {
  MANUFACTURERS,
  RETAIL_CATALOG,
  SUPPLIERS,
  filterManufacturers,
} from "../../routes/dev/fixtures";
import { filterSuppliers } from "../../routes/dev/restockFixtures";
import { batchesFor } from "../../routes/dev/productDetailFixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc } from "../../routes/dev/storyMocks";
import Inventory from "../../routes/Inventory";
import Batches from "../../routes/inventory/Batches";

// Batch (lot) list — every lot on the shelf, and the one surface that can
// answer "who MADE this stock".
//
// Rendered by screens/{desktop,mobile}/pages/inventory/BatchList.stories.tsx;
// this module is the one place its data and states are written.
//
// A lot is where the manufacturer stops being intent and becomes fact. The
// product carries a list of pabrik it MAY be sourced from; only the batch
// records which one a given delivery actually came from, stamped at receive
// from the purchase-order line. That is why the Pabrik filter lives here as
// well as on the restock list: the restock list finds the ORDERS, this finds
// the STOCK, and in a recall it is the stock you have to pull off the shelf.

const DEFAULT_LIMIT = 25;

/**
 * Lots across several products, each stamped with the pabrik that made it.
 *
 * `batchesFor` builds three lots per product with no maker (it predates the
 * column), so they are stamped HERE, on clones. Two things are deliberate: one
 * product's lots come from DIFFERENT factories -- the whole point of the model,
 * and impossible to represent before the split -- and some lots carry no maker
 * at all, which is what every batch received before this shipped looks like.
 */
const LOTS: Batch[] = RETAIL_CATALOG.slice(0, 6).flatMap((p, i) =>
  batchesFor(p).map((b, j) => {
    const copy = b.clone();
    // Product 0 is the multi-source case: three lots, three makers. Every third
    // lot elsewhere stays blank, standing in for stock that arrived before the
    // fact was ever recorded.
    if (i === 0) copy.manufacturerId = ["mfr-1", "mfr-2", "mfr-4"][j] ?? "";
    else if (j % 3 !== 2) copy.manufacturerId = p.manufacturerId;
    return copy;
  }),
);

/**
 * The server's ListBatches predicate, as the story's fake.
 *
 * Mocked as a FUNCTION of the request rather than a canned page, so the search
 * box, the pemasok picker, the date picker and the new Pabrik picker all work
 * live -- which is the only way a filter story can demonstrate anything. A lot
 * with NO recorded maker matches no pabrik filter: it is unknown, not
 * everyone's, and the server behaves the same way.
 */
function filterBatches(
  rows: Batch[],
  req: { query: string; supplierId: string; manufacturerId: string; onlyInStock: boolean },
): Batch[] {
  const q = req.query.trim().toLowerCase();
  return rows
    .filter((b) => !req.supplierId || b.supplierId === req.supplierId)
    .filter((b) => !req.manufacturerId || b.manufacturerId === req.manufacturerId)
    .filter((b) => !req.onlyInStock || b.currentQuantity > 0n)
    .filter(
      (b) =>
        !q ||
        b.batchNumber.toLowerCase().includes(q) ||
        b.productName.toLowerCase().includes(q),
    )
    .sort((a, b) => a.expiryDate.localeCompare(b.expiryDate));
}

/** Everything the page reads. */
function listHandlers(rows: Batch[]) {
  return [
    mockRpc(BatchService, "listBatches", (req) => {
      const matched = filterBatches(rows, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { batches: matched.slice(req.offset, req.offset + limit), total: matched.length };
    }),
    // Names for the rows (resolve-by-IDs) and for the two toolbar pickers,
    // which are server searches rather than preloaded lists.
    mockRpc(ProductService, "resolveProducts", (req) => ({
      products: RETAIL_CATALOG.filter((p) => req.ids.includes(p.id)).map((p) => ({
        id: p.id,
        name: p.name,
        sku: p.sku,
      })),
    })),
    mockRpc(SupplierService, "resolveSuppliers", (req) => ({
      suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
    })),
    mockRpc(SupplierService, "searchSuppliers", (req) => ({
      suppliers: filterSuppliers(req.query).slice(0, req.limit || 20),
    })),
    // Resolve includes archived rows (a lot made by a retired factory must still
    // render its name); search is active-only. Same asymmetry as the server.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
    mockRpc(ManufacturerService, "searchManufacturers", (req) => ({
      manufacturers: filterManufacturers(MANUFACTURERS, {
        query: req.query,
        includeInactive: false,
      }).slice(0, req.limit || 20),
    })),
  ];
}

// Mounted under the real <Inventory> layout route, which supplies the page's
// header — rendering the list alone would show a page with no title and invite
// someone to "fix" it by adding a second one.
export const routes = (
  <Route path="/inventory" element={<Inventory />}>
    <Route path="batches" element={<Batches />} />
  </Route>
);

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: Batches,
  parameters: {
    router: { initialEntries: ["/inventory/batches"] },
    msw: mockApi(...listHandlers(LOTS)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Owner: story(
    "Every lot on the shelf, newest expiry last. The toolbar is live: search by lot number or " +
      "product, narrow to one pemasok, pick a date range over either the received or the expiry " +
      "date, and — the reason this page gained a column — narrow to one PABRIK. The fixture " +
      "mirrors the server's predicate rather than hard-coding a result, so all four genuinely " +
      "filter here.",
  ),
  ByManufacturer: story(
    "The Pabrik filter, which is what a recall actually runs. A product's own list says which " +
      "factories it MAY be sourced from; a lot says which one it WAS, stamped at receive from " +
      "the purchase-order line. So this is the only place that can answer \"which of this stock " +
      "do I have to pull off the shelf\" — the restock list's identical-looking filter finds the " +
      "ORDERS, not the goods. The first product here has three lots from three different " +
      "factories, which is exactly the state a single manufacturer column on the product could " +
      "not represent.",
  ),
  UnknownMakers: story(
    "Lots with no pabrik recorded, shown as a dash. This is what everything received before " +
      "the column existed looks like, and it is permanent: the fact was never captured, and " +
      "filling it in from the product's current maker would invent provenance for stock that " +
      "predates the question. A blank is worse than useless only if it is mistaken for a " +
      "failure to load, which is why it reads as a dash in a column beside a populated one " +
      "rather than as an empty cell. Such a lot matches NO pabrik filter — it is unknown, not " +
      "everyone's.",
  ),
  Empty: story(
    "A shop with no lots yet — a fresh install, or every filter combined until nothing matches. " +
      "The empty state has to be legible as \"nothing here\" rather than as a page that failed, " +
      "which is the whole reason it is worth a story.",
    { parameters: { msw: mockApi(...listHandlers([])) } },
  ),
};
