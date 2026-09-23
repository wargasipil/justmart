import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { userEvent, within } from "storybook/test";

import { ProductDiscountService } from "../../gen/inventory_iface/v1/product_discount_connect";
import { PHARMACY_CATALOG } from "../dev/fixtures";
import { discountsFor } from "../dev/productDetailFixtures";
import { mockApi, mockRpc, mockRpcError } from "../dev/storyMocks";
import DiscountTab from "./ProductDiscountTab";
import { discountShopHandlers, newDiscountShop } from "./ProductDiscountTab.shop";

// The "Diskon" tab of the product detail page — the automatic per-product price
// rules POS applies at the till.
//
// It is the only one of the five tabs that WRITES, so unlike its neighbours it
// runs over a small in-memory shop (ProductDiscountTab.shop.ts) rather than a
// canned read: Add lands a row, Edit changes one, Delete removes one. Against a
// fixed fixture every one of those would look broken, because each mutation
// invalidates the list key and the refetch would serve the original rows back.
//
// Manager-only (the discount RPCs are OWNER+PHARMACIST), so no till story.
//
// Rendered by ProductDiscountTab.stories.tsx
// ("components/products/ProductDiscountTab").

const PRODUCT = PHARMACY_CATALOG[0];

/** A story's own copy of the rules — one story's delete must not reach the next. */
const own = (seed = discountsFor(PRODUCT)) =>
  mockApi(...discountShopHandlers(PRODUCT, newDiscountShop(PRODUCT, seed)));

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: DiscountTab,
  args: { productId: PRODUCT.id },
  parameters: { msw: own() },
};

type Story = StoryObj<typeof DiscountTab>;

const story = (description: string, s: Story = {}): Story => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

// The row actions are icon-only, and each carries an aria-label naming the RULE
// it acts on ("Hapus diskon Persen per item") rather than just the verb — so a
// play() asks for the button by name instead of reaching into the row by
// position, and the query breaks loudly if that name ever goes away.

export const stories = {
  Manager: story(
    "Two rules, which between them cover every cell the table derives: a 10% per-item cut with " +
      "no qty rule, and a flat amount on the line gated at 100 tablets that lapsed three days " +
      "ago. An expired rule STAYS listed, marked with the red badge — it is history, not " +
      "rubbish, and POS simply stops applying it.",
  ),
  Adding: story(
    "The Add drawer open over the table. The mode picker is the thing to read: fixed vs " +
      "percent, and per item vs per line, are four different arithmetics at the till, so they " +
      "are one four-way choice here rather than a checkbox beside a number. Saving really does " +
      "land a row — the mocks hold the table.",
    {
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await userEvent.click(await canvas.findByRole("button", { name: /tambah diskon|add discount/i }));
      },
    },
  ),
  Deleting: story(
    "The delete confirmation for the live rule. It is a Chakra alertdialog, not window.confirm " +
      "— the app has no native dialogs — and confirming removes the row for real here. Worth " +
      "its own story because a discount delete is silent at the till: nothing announces that " +
      "the price went back up.",
    {
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await userEvent.click(
          await canvas.findByRole("button", { name: /^(hapus diskon|delete) .*(per item|item discount)/i }),
        );
      },
    },
  ),
  Empty: story(
    "No rules yet — the normal state for most products. The Add button sits above the empty " +
      "table rather than inside the empty state, so it stays in the same place once there are " +
      "rows.",
    { parameters: { msw: own([]) } },
  ),
  Loading: story("ListProductDiscounts never answers: the empty table under a working pager.", {
    parameters: {
      msw: mockApi(
        mockRpc(ProductDiscountService, "listProductDiscounts", { discounts: [], total: 0 }, { delay: "infinite" }),
      ),
    },
  }),
  LoadFailed: story(
    "The read fails: an empty table plus the global QueryCache's toast. Add is still enabled — " +
      "the tab does not know the list failed, and a create would fail loudly on its own.",
    {
      parameters: {
        msw: mockApi(
          mockRpcError(ProductDiscountService, "listProductDiscounts", Code.Unavailable, "backend down"),
        ),
      },
    },
  ),
};
