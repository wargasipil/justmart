import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { userEvent, within } from "storybook/test";

import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { ProductPriceTierService } from "../../gen/inventory_iface/v1/product_price_tier_connect";
import { PHARMACY_CATALOG } from "../dev/fixtures";
import { tierPricesFor, unitPricesFor } from "../dev/productDetailFixtures";
import { mockApi, mockRpc, mockRpcError } from "../dev/storyMocks";
import ProductPriceHistoryTab from "./ProductPriceHistoryTab";

// The "Riwayat Harga Jual" tab of the product detail page: what this product
// has been SOLD at over time, in two flavours behind one segmented control —
// the per-unit catalog price and the grosir ladder.
//
// The two are separate tables with different keys (a rung is (unit, min_qty),
// a unit price is (unit)), which is exactly why they are worth two stories:
// they page independently and they empty independently, and the segment is the
// only thing that says which one you are reading.
//
// Manager-only — both RPCs are OWNER+PHARMACIST — so there is no till story;
// ProductDetail does not mount this tab for a cashier at all.
//
// Rendered by ProductPriceHistoryTab.stories.tsx
// ("components/products/ProductPriceHistoryTab").

const PRODUCT = PHARMACY_CATALOG[0];

const unitPrices = (rows = unitPricesFor(PRODUCT)) =>
  mockRpc(ProductService, "listProductUnitPrices", (req) => ({
    prices: rows.slice(req.offset, req.offset + (req.limit || 25)),
    total: rows.length,
  }));

const tierPrices = (rows = tierPricesFor(PRODUCT)) =>
  mockRpc(ProductPriceTierService, "listProductTierPrices", (req) => ({
    prices: rows.slice(req.offset, req.offset + (req.limit || 25)),
    total: rows.length,
  }));

/** Switch to the Grosir panel — local state, so a click is the only seam. */
const openTierScope: StoryObj["play"] = async ({ canvasElement }) => {
  const canvas = within(canvasElement);
  await userEvent.click(await canvas.findByText(/^(grosir|wholesale)$/i));
};

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: ProductPriceHistoryTab,
  args: { productId: PRODUCT.id },
  parameters: { msw: mockApi(unitPrices(), tierPrices()) },
};

type Story = StoryObj<typeof ProductPriceHistoryTab>;

const story = (description: string, s: Story = {}): Story => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  PerUnit: story(
    "The panel the tab opens on: one row per price this product's units have carried. The box " +
      "was re-priced 35 days ago, so it appears twice — the older row closed, the newer one " +
      "reading \"sekarang\" in the Sampai column. Strip and base have never changed, which is " +
      "what makes \"sekarang\" legible as a state rather than a blank cell.",
  ),
  Wholesale: story(
    "The Grosir panel. A row is the price of a RUNG — \"buy ≥N of this unit\" — not of a tier " +
      "row, because tiers are hard-deleted and a tier id would dangle. The ≥5 strip rung was " +
      "re-priced 15 days ago and so appears twice; that closed/open pair is the whole reason " +
      "this table is separate from the one beside it.",
    { play: openTierScope },
  ),
  WholesaleEmpty: story(
    "A shop with per-unit prices but no grosir ladder — the common case for a small toko. The " +
      "two panels empty independently: switching back to Per satuan still shows a full table, " +
      "so the empty state has to name which of the two is empty, and it does.",
    {
      parameters: { msw: mockApi(unitPrices(), tierPrices([])) },
      play: openTierScope,
    },
  ),
  Empty: story(
    "Neither table has a row: a product created but never re-priced. The segment still renders " +
      "— you can be sure you are looking at the right panel rather than a failed load.",
    { parameters: { msw: mockApi(unitPrices([]), tierPrices([])) } },
  ),
  Loading: story("ListProductUnitPrices never answers: the panel's empty frame under a working pager.", {
    parameters: {
      msw: mockApi(
        mockRpc(ProductService, "listProductUnitPrices", { prices: [], total: 0 }, { delay: "infinite" }),
        tierPrices(),
      ),
    },
  }),
  LoadFailed: story(
    "The read fails. Like the other tabs this one has no error branch — it renders empty and " +
      "the global QueryCache raises the toast.",
    {
      parameters: {
        msw: mockApi(
          mockRpcError(ProductService, "listProductUnitPrices", Code.Unavailable, "backend down"),
          tierPrices(),
        ),
      },
    },
  ),
};
