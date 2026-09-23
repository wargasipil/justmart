import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import type { HttpHandler } from "msw";
import { Route } from "react-router-dom";

import { ManufacturerService } from "../../gen/inventory_iface/v1/manufacturer_connect";
import { PriceAgreementService } from "../../gen/inventory_iface/v1/price_agreement_connect";
import type { PriceAgreement } from "../../gen/inventory_iface/v1/price_agreement_pb";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { PurchaseOrderService } from "../../gen/purchasing_iface/v1/order_connect";
import {
  MANUFACTURERS,
  PHARMACY_CATALOG,
  RETAIL_CATALOG,
  filterManufacturers,
  filterProducts,
} from "../../routes/dev/fixtures";
import { agreementsForSupplier } from "../../routes/dev/restockFixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import NewPurchaseOrder from "../../routes/purchasing/NewPurchaseOrder";
import PurchaseOrderDetail from "../../routes/purchasing/PurchaseOrderDetail";
import PurchaseOrdersList from "../../routes/purchasing/PurchaseOrdersList";
import Purchasing from "../../routes/purchasing/Purchasing";
import { listHandlers } from "./purchasing";
import { newShop, shopHandlers } from "./restockShop";

// Writing a restock order (/purchasing/new) — the only FORM of the three
// restock screens, and the one that produces the orders the other two read.
// Rendered by screens/{desktop,mobile}/pages/purchasing/NewRestockOrder.stories
// .tsx; this module is the one place its data and states are written.
//
// EVERY STORY OPENS THE SAME EMPTY FORM, and that is a property of the page,
// not a gap here: the lines live in local component state with no prop and no
// query param, so there is no seam to seed a half-written order through. What
// the stories differ in is therefore what the SERVER does — which suppliers
// come back, whether they have agreed prices, what the catalog holds, and
// whether the create succeeds. That is the interesting axis anyway: the form
// itself is the same six fields every time.
//
// The mocks WRITE (see restockShop.ts), because submitting NAVIGATES to
// /purchasing/<new id>. A canned create would land every story on a detail page
// for an order that does not exist, so the one screen this form exists to
// produce would be the one it could never show.

// Mounted under the real <Purchasing> shell: `new` reads as a subpage there, so
// the "Restock" header stays and the status tabs + stat row are hidden. The
// list and detail routes ARE registered here, unlike in the detail scenario,
// because this page's two exits are Batal (to /purchasing/all) and a successful
// create (to /purchasing/<id>) — a form whose exits dead-end is a form you
// cannot check.
export const routes = (
  <Route path="/purchasing" element={<Purchasing />}>
    <Route path="new" element={<NewPurchaseOrder />} />
    <Route path="all" element={<PurchaseOrdersList />} />
    <Route path=":id" element={<PurchaseOrderDetail />} />
  </Route>
);

const DEFAULT_LIMIT = 25;

type FormOpts = {
  /** What the "Tambah produk" picker can offer. Defaults to the retail shelf. */
  catalog?: Product[];
  /** The chosen supplier's agreed prices. Defaults to the shop's real ones. */
  agreements?: (supplierId: string) => PriceAgreement[];
  /** Handlers to answer FIRST — how a story stages a refusal. */
  first?: HttpHandler[];
};

/**
 * Everything this page reads and writes, over a fresh copy of the ledger.
 *
 * ListProducts is mocked as a function of the request (the same rule the
 * Products story follows) so the picker's search box and its pager genuinely
 * work — a canned page would demo a dialog whose whole job is finding one
 * product among thirty.
 */
export function formHandlers(opts: FormOpts = {}): HttpHandler[] {
  const { catalog = RETAIL_CATALOG, agreements = agreementsForSupplier, first = [] } = opts;
  const shop = newShop();
  // The story's OWN copy of the shelf, because setting a line's pabrik writes
  // to the product. Mutating the shared fixture would leak one story's edit
  // into every other story that reads the catalog, in whatever order the
  // reader happened to open them.
  const shelf = catalog.map((p) => p.clone());
  const productOf = (id: string) => shelf.find((p) => p.id === id);
  return [
    ...first,
    mockRpc(ProductService, "listProducts", (req) => {
      const matched = filterProducts(shelf, req);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { products: matched.slice(req.offset, req.offset + limit), total: matched.length };
    }),
    // Gated on a supplier in the page, so this stays unasked until one is
    // picked — which is the behaviour worth seeing, not a detail to mock around.
    mockRpc(PriceAgreementService, "listPriceAgreements", (req) => {
      const rows = agreements(req.supplierId);
      return { agreements: rows, total: rows.length };
    }),
    // A line's pabrik: resolved for the label (one batched call for every
    // line), searched for the picker. Both mirror the server's predicate --
    // resolve INCLUDES archived rows (a product already pointing at one must
    // still render a name) while search is active-only (you cannot assign an
    // archived pabrik), which is exactly the asymmetry the real RPCs have.
    mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
      manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
    })),
    mockRpc(ManufacturerService, "searchManufacturers", (req) => ({
      manufacturers: filterManufacturers(MANUFACTURERS, {
        query: req.query,
        includeInactive: false,
      }).slice(0, req.limit || 20),
    })),
    // ...and it WRITES, like the create below it. A canned success would make
    // the picker look identical whether or not the catalog actually learned,
    // which is the one thing a story about a catalog side effect has to show.
    //
    // ADDITIVE, mirroring the server: naming a pabrik the product is not
    // approved for APPROVES it and never touches the usual maker. A mock that
    // overwrote the primary here would demonstrate precisely the behaviour this
    // model was changed to stop.
    mockRpc(ProductService, "addProductManufacturer", (req) => {
      const p = productOf(req.productId);
      if (p && !p.manufacturerIds.includes(req.manufacturerId)) {
        p.manufacturerIds = [...p.manufacturerIds, req.manufacturerId];
        if (!p.manufacturerId) p.manufacturerId = req.manufacturerId;
      }
      return { product: p };
    }),
    // The supplier picker + the list this form returns to, over the same shop
    // the create writes into — so a new order is in the list on the way back.
    ...listHandlers(shop.orders),
    ...shopHandlers(shop),
  ];
}

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: NewPurchaseOrder,
  parameters: {
    router: { initialEntries: ["/purchasing/new"] },
    msw: mockApi(...formHandlers()),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

/**
 * A story's own copy of the ledger and its own handlers. Each story calls this,
 * so an order created while reading one story cannot turn up in the next one's
 * list.
 */
const own = (opts: FormOpts = {}, extra: Record<string, unknown> = {}): StoryObj["parameters"] => ({
  ...extra,
  msw: mockApi(...formHandlers(opts)),
});

/**
 * The retail shelf with no pabrik recorded on any product.
 *
 * Cleared on a COPY, never on the fixture: RETAIL_CATALOG is one shared array
 * read by every product, manufacturer and restock story, and a module-level
 * mutation would silently empty the Pabrik field on all of them.
 */
const NO_MAKERS: Product[] = RETAIL_CATALOG.map((p) => {
  const copy = p.clone();
  copy.manufacturerId = "";
  copy.manufacturerIds = [];
  return copy;
});

/**
 * A shelf where the first few products are sourced from several pabrik.
 *
 * Built rather than found so the story is not hostage to which fixture index
 * happens to be multi-source: the first rows of the picker are the ones a
 * reader will click, so those are the ones that have to demonstrate it.
 */
const MULTI_SOURCE: Product[] = RETAIL_CATALOG.map((p, i) => {
  if (i > 5) return p;
  const copy = p.clone();
  copy.manufacturerIds = ["mfr-1", "mfr-2", "mfr-4"];
  copy.manufacturerId = copy.manufacturerIds[0];
  return copy;
});

export const stories = {
  Owner: story(
    "Writing a restock order, live end to end. Pick a pemasok and the form fetches that " +
      "supplier's agreed prices; Tambah produk opens the real picker over a thirty-product " +
      "shelf (its search and its pager work — it is mocked as a function of the request); " +
      "each checked product becomes a line you set a pack count and a cost on, and the totals " +
      "card re-adds itself as you type, including the PPN switch. Each line also names the " +
      "pabrik it is sourced from, suggested from the product's usual maker and confirmed " +
      "against the invoice; every id on screen is resolved in one batched call. Press Buat " +
      "and the order is " +
      "written to the ledger and you land on its detail page — a real DRAFT, with Kirim and " +
      "Batalkan waiting on it.",
    { parameters: own() },
  ),
  Admin: story(
    "The same form for the manager tier (PHARMACIST, shown as Admin). Restock is manager work " +
      "end to end — what the shop spends and what it will owe — so both roles that can reach " +
      "/purchasing write orders the same way, and a cashier is not routed here at all.",
    { parameters: own({}, { pageContext: { user: "admin" } }) },
  ),
  Pharmacy: story(
    "The same form in an apotek. The catalog noun follows the business mode through the " +
      "glossary, so the picker and the line table say Obat where a toko says Produk — one " +
      "i18n bundle, not a second form. The arithmetic below is untouched by mode: a pharmacy " +
      "buys in packs and pays PPN exactly like a minimarket.",
    { parameters: own({ catalog: PHARMACY_CATALOG }, { pageContext: { mode: "pharmacy" } }) },
  ),
  AboveAgreedPrice: story(
    "This shop has negotiated prices with two of its four distributors. Pick PT Sinar Farma " +
      "or PT Indo Grosir Jaya, add one of their products, and the line carries a Disepakati " +
      "figure — what that pack was agreed at. The fixture prices every agreement just UNDER " +
      "the product's own cost, so typing the straightforward pack cost trips the warning the " +
      "field exists for. It only ever warns: an agreement is a negotiating position, not a " +
      "rule the app enforces, and a shop paying above one has usually already decided to.",
    { parameters: own() },
  ),
  NoAgreements: story(
    "The common case for a small toko: nothing negotiated with anyone, so a line is just a " +
      "qty and a cost with no reference beside it. Worth seeing next to the story above, " +
      "because the agreement hint is the only part of a line that is sometimes absent — and " +
      "its absence has to read as nothing-agreed, not as a figure that failed to load.",
    { parameters: own({ agreements: () => [] }) },
  ),
  EmptyCatalog: story(
    "A shop whose catalog is still empty — the state a brand-new install is in when someone " +
      "reaches for Restock first. Tambah produk opens on the picker's empty state, and the " +
      "form cannot be submitted: Buat stays disabled while there are no lines, which is the " +
      "one piece of validation this page has (there is no field-level Zod schema here — the " +
      "lines are not an RHF form).",
    { parameters: own({ catalog: [] }) },
  ),
  MultipleManufacturers: story(
    "One product, several factories — the case a single manufacturer column could not " +
      "hold. The first products on this shelf are approved for three pabrik each, so their " +
      "lines offer a short list instead of a search box: two clicks, no typing, and the " +
      "usual maker already selected as a suggestion. What is picked here is ORDER data. It " +
      "travels with the line, lands on purchase_order_items, and is stamped onto the lot at " +
      "receive — which is the only place the maker of physical stock is ever recorded, and " +
      "what the Pabrik filter on the batches page reads. Changing it does NOT touch the " +
      "product: restocking the same item from a second pabrik used to overwrite the one " +
      "column every earlier batch was implicitly labelled by.",
    { parameters: own({ catalog: MULTI_SOURCE }) },
  ),
  MissingManufacturer: story(
    "The same shelf with no pabrik recorded anywhere — the state a real shop is in, since " +
      "the manufacturer column shipped with no backfill. With no approved list to offer, " +
      "every line opens the full searchable picker rather than a grey label, so the missing " +
      "case is the one asking to be filled. Naming a pabrik the product is not approved for " +
      "does two separate things, and the distinction is the point: it sets the SOURCE OF " +
      "THIS LINE (order data, committed by Buat), and it APPROVES that pabrik for the " +
      "product so the next order starts from a list that has learned. Additive only — it " +
      "never removes a source and never moves the usual one, which is why it can be safely " +
      "done mid-order by a buyer reading an invoice. The toast reports the catalog half, " +
      "because that is the half which outlives this form and survives abandoning it.",
    { parameters: own({ catalog: NO_MAKERS }) },
  ),
  CreateFailed: story(
    "The server refuses the create. The global MutationCache turns it into an error toast and " +
      "the page stays exactly where it was — supplier, lines, discounts and PPN all still " +
      "filled in. That is the behaviour worth pinning: a failed submit on a long form must " +
      "never be a cleared form, and this page navigates away ONLY on success. The refusal is " +
      "a REAL token the backend emits (purchasing.items_required) rather than invented prose, " +
      "so the toast also shows the Tier-1 rule working: serverErrors.ts translates it to " +
      "\"Tambahkan minimal satu item\" instead of leaking a machine string at the operator.",
    {
      parameters: own({
        first: [
          mockRpcError(
            PurchaseOrderService,
            "createPurchaseOrder",
            Code.InvalidArgument,
            "purchasing.items_required",
          ),
        ],
      }),
    },
  ),
};
