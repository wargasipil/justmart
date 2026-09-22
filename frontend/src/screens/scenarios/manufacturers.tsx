import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";

import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import type { Manufacturer } from "../../gen/inventory_iface/v1/manufacturer_pb";
import {
  MANUFACTURERS,
  RETAIL_CATALOG,
  filterManufacturerProducts,
  filterManufacturers,
  manufacturerProductsFor,
} from "../../routes/dev/fixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError, rpcError } from "../../routes/dev/storyMocks";
import Inventory from "../../routes/Inventory";
import ManufacturerDetail from "../../routes/inventory/ManufacturerDetail";
import Manufacturers from "../../routes/inventory/Manufacturers";

// Pabrik (manufacturers) — the admin list, on the shared fixture set
// (routes/dev/fixtures.ts). Rendered by
// screens/{desktop,mobile}/pages/inventory/ManufacturerList.stories.tsx; this
// module is the one place its data and states are written.
//
// ListManufacturers is mocked as a FUNCTION of the request, not a canned page:
// the fake applies the same predicate the server will (archived switch,
// case-insensitive name/code search), orders by name and pages — so the search
// box, the "show archived" switch and the pager all work live here. The
// summary tile aggregates the SAME filtered set, which is the filter-parity
// rule GetManufacturersSummary has to be written to.
//
// The page is mounted UNDER the real <Inventory> layout route, because that is
// what supplies its PageHeader ("Inventaris") — rendering the list alone would
// show a page with no title and invite someone to "fix" it by adding a second
// header.

const DEFAULT_LIMIT = 25;

/** Everything the list reads. */
export function listHandlers(rows: Manufacturer[]) {
  return [
    mockRpc(ManufacturerService, "listManufacturers", (req) => {
      const matched = filterManufacturers(rows, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return {
        manufacturers: matched.slice(req.offset, req.offset + limit),
        total: matched.length,
      };
    }),
    mockRpc(ManufacturerService, "getManufacturersSummary", (req) => ({
      totalManufacturers: BigInt(filterManufacturers(rows, req).length),
    })),
    // Opening a row: the detail page reads these two.
    mockRpc(ManufacturerService, "getManufacturer", (req) => ({
      manufacturer: rows.find((m) => m.id === req.id),
    })),
    mockRpc(ManufacturerService, "listManufacturerProducts", (req) => {
      const owned = manufacturerProductsFor(req.manufacturerId, RETAIL_CATALOG);
      const matched = filterManufacturerProducts(owned, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { products: matched.slice(req.offset, req.offset + limit), total: matched.length };
    }),
  ];
}

/** Writes that succeed — enough for the drawers to close on save. */
function writeHandlers(rows: Manufacturer[]) {
  return [
    mockRpc(ManufacturerService, "createManufacturer", (req) => ({
      manufacturer: { ...req, id: "mfr-new", active: true },
    })),
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
    <Route path="manufacturers" element={<Manufacturers />} />
    {/* Clicking a row opens the REAL detail page, so the list story is also the
        check that the two agree. Its own states live in
        scenarios/manufacturerDetail.tsx. */}
    <Route path="manufacturers/:id" element={<ManufacturerDetail />} />
  </Route>
);

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: Manufacturers,
  parameters: {
    router: { initialEntries: ["/inventory/manufacturers"] },
    msw: mockApi(...listHandlers(MANUFACTURERS), ...writeHandlers(MANUFACTURERS)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Owner: story(
    "28 active pabrik over two pages. Everything on the toolbar is live: search by name or " +
      "code, flip \"show archived\" to reveal the two closed factories (dimmed, with Restore " +
      "instead of Archive), and page — the total tile re-counts with every filter rather than " +
      "counting the page. Add and Edit open the drawer; Archive asks first.",
  ),
  Admin: story(
    "The same page for the manager tier (PHARMACIST, shown as \"Admin\"). Identical: pabrik is " +
      "catalog reference data, not cost, so there is nothing to withhold from a manager. The " +
      "till never reaches this page at all — it is OWNER + PHARMACIST in the proto.",
    { parameters: { pageContext: { user: "admin" } } },
  ),
  Pharmacy: story(
    "Pharmacy mode. The page is word-for-word the same — \"Pabrik\" is not the catalog noun, " +
      "so the Produk↔Obat glossary swap doesn't touch it. Worth pinning: this is the story " +
      "that would catch someone wiring the manufacturer label into the glossary by mistake.",
    { parameters: { pageContext: { mode: "pharmacy" } } },
  ),
  Empty: story(
    "A fresh install with no pabrik yet. The empty cell is a prompt (\"Belum ada pabrik. " +
      "Tambahkan yang pertama.\") rather than the generic \"no results\" a search would give, " +
      "and the tile reads \"0\", not blank — that's the formatCount-vs-formatThousands rule: a " +
      "blank tile reads as \"failed to load\" when zero is the real answer.",
    { parameters: { msw: mockApi(...listHandlers([]), ...writeHandlers([])) } },
  ),
  DuplicateCode: story(
    "The create drawer's refusal path. Press Add and save with code MFR-0001 (any other code " +
      "succeeds): the server answers the stable token `manufacturer.code_taken`, and " +
      "useServerFormErrors puts the translated message on the Kode field rather than in a " +
      "toast. Try a bad email too — that one is caught client-side by Zod, through the global " +
      "error map, so it never reaches the server.",
    {
      parameters: {
        msw: mockApi(
          mockRpc(ManufacturerService, "createManufacturer", (req) =>
            MANUFACTURERS.some((m) => m.code.toLowerCase() === req.code.trim().toLowerCase())
              ? rpcError(Code.AlreadyExists, "manufacturer.code_taken")
              : { manufacturer: { ...req, id: "mfr-new", active: true } },
          ),
          ...listHandlers(MANUFACTURERS),
          ...writeHandlers(MANUFACTURERS),
        ),
      },
    },
  ),
  Loading: story(
    "ListManufacturers never answers: the table's spinner. The tile still lands, which is the " +
      "point of them being separate queries.",
    {
      parameters: {
        msw: mockApi(
          mockRpc(
            ManufacturerService,
            "listManufacturers",
            { manufacturers: [], total: 0 },
            { delay: "infinite" },
          ),
          ...listHandlers(MANUFACTURERS),
        ),
      },
    },
  ),
  LoadFailed: story(
    "ListManufacturers fails. In the app the global QueryCache turns this into an error toast; " +
      "the page itself has no error state and falls through to the empty cell — which is worth " +
      "seeing, since it is indistinguishable from a genuinely empty list.",
    {
      parameters: {
        msw: mockApi(
          mockRpcError(ManufacturerService, "listManufacturers", Code.Unavailable, "database unavailable"),
          ...listHandlers(MANUFACTURERS),
        ),
      },
    },
  ),
};
