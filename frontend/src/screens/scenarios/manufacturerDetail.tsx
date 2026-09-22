import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";

import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import type { Manufacturer } from "../../gen/inventory_iface/v1/manufacturer_pb";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import {
  MANUFACTURERS,
  PHARMACY_CATALOG,
  RETAIL_CATALOG,
  filterManufacturerProducts,
  manufacturerProductsFor,
} from "../../routes/dev/fixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import Inventory from "../../routes/Inventory";
import ManufacturerDetail from "../../routes/inventory/ManufacturerDetail";

// One pabrik: contact details + the catalog it makes.
//
// The product section is the whole argument for this page existing rather than
// being the edit drawer, so every story here is really about that table: who
// has one, who doesn't, and what it looks like while it loads or fails.
//
// Products are assigned to makers by `makerOf` in fixtures.ts — concentrated on
// five of the thirty, the way a real shelf is. mfr-1 (Kalbe) carries half the
// catalog; mfr-6 carries none, which is what NoProducts shows.

const DEFAULT_LIMIT = 25;

/** Everything the page reads, for one manufacturer over one catalog. */
export function detailHandlers(rows: Manufacturer[], catalog: Product[]) {
  return [
    mockRpc(ManufacturerService, "getManufacturer", (req) => ({
      manufacturer: rows.find((m) => m.id === req.id),
    })),
    mockRpc(ManufacturerService, "listManufacturerProducts", (req) => {
      const owned = manufacturerProductsFor(req.manufacturerId, catalog);
      const matched = filterManufacturerProducts(owned, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { products: matched.slice(req.offset, req.offset + limit), total: matched.length };
    }),
    // The header's Edit / Archive / Restore actions.
    mockRpc(ManufacturerService, "updateManufacturer", (req) => ({
      manufacturer: { ...req, active: true },
    })),
    mockRpc(ManufacturerService, "archiveManufacturer", (req) => ({
      manufacturer: { ...rows.find((m) => m.id === req.id), active: false },
    })),
    mockRpc(ManufacturerService, "unarchiveManufacturer", (req) => ({
      manufacturer: { ...rows.find((m) => m.id === req.id), active: true },
    })),
  ];
}

export const routes = (
  <Route path="/inventory" element={<Inventory />}>
    <Route path="manufacturers/:id" element={<ManufacturerDetail />} />
  </Route>
);

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: ManufacturerDetail,
  parameters: {
    router: { initialEntries: ["/inventory/manufacturers/mfr-1"] },
    msw: mockApi(...detailHandlers(MANUFACTURERS, RETAIL_CATALOG)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

/** A story pinned to one manufacturer's URL. */
const at = (id: string, extra: Record<string, unknown> = {}) => ({
  router: { initialEntries: [`/inventory/manufacturers/${id}`] },
  ...extra,
});

export const stories = {
  Owner: story(
    "PT Kalbe Farma — the shop's dominant maker, carrying half the catalog. The info grid is " +
      "the pabrik's own fields; everything below it is the part a table row could never show. " +
      "Search by name or SKU and flip \"show archived\" — both re-query, like the real RPC. " +
      "Edit opens the same drawer the list uses; Archive asks first.",
  ),
  Pharmacy: story(
    "The same page over an apotek's shelf. The chrome is unchanged — \"Pabrik\" is not the " +
      "catalog noun, so the Produk↔Obat glossary swap doesn't reach it — but the products are " +
      "now obat, which is the case where a manufacturer column actually earns its keep.",
    {
      parameters: at("mfr-1", {
        pageContext: { mode: "pharmacy" },
        msw: mockApi(...detailHandlers(MANUFACTURERS, PHARMACY_CATALOG)),
      }),
    },
  ),
  NoProducts: story(
    "PT Darya-Varia: a real pabrik in the list with nothing linked to it yet — the state every " +
      "manufacturer is in on the day the column ships. The empty cell says so in its own words " +
      "(\"Belum ada produk yang ditautkan…\") rather than reusing the generic \"no results\", " +
      "which would read as a failed search.",
    { parameters: at("mfr-6") },
  ),
  Archived: story(
    "An archived pabrik. The badge goes gray and the header action becomes Restore — the page " +
      "stays readable rather than hiding, because its products and history still matter after " +
      "the shop stops buying from it.",
    { parameters: at("mfr-29") },
  ),
  Loading: story(
    "GetManufacturer never answers: the page-level spinner, before any chrome. Worth pinning " +
      "separately from the list's spinner — this one replaces the WHOLE page, including the " +
      "title, so there is nothing to read while it hangs.",
    {
      parameters: at("mfr-1", {
        msw: mockApi(
          mockRpc(ManufacturerService, "getManufacturer", { manufacturer: undefined }, { delay: "infinite" }),
          ...detailHandlers(MANUFACTURERS, RETAIL_CATALOG),
        ),
      }),
    },
  ),
  NotFound: story(
    "A dead link — an id that no longer exists (deleted row, stale bookmark, hand-typed URL). " +
      "The page renders the neutral \"no results\" line WITH the Back button, so the user is " +
      "never stranded on a blank screen. That's the reason the early return renders " +
      "<BackButton> too and not just the text.",
    {
      parameters: at("mfr-does-not-exist", {
        msw: mockApi(
          mockRpcError(ManufacturerService, "getManufacturer", Code.NotFound, "manufacturer not found"),
          ...detailHandlers(MANUFACTURERS, RETAIL_CATALOG),
        ),
      }),
    },
  ),
};
