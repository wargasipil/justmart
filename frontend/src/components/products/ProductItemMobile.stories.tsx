import { Stack, StackSeparator } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";

import ProductItemMobile, { ProductItemMobileSkeleton } from "./ProductItemMobile";
import { PHARMACY_CATALOG, RETAIL_CATALOG } from "../../routes/dev/fixtures";
import { storyDocs } from "../../routes/dev/storyDocs";
import { MOBILE, MOBILE_PAGE } from "../../screens/viewports";

// Pinned to the mobile viewport — that is the only width this row is for.
// Fixture products have no picture, so no request is made.

const meta = {
  title: "components/products/ProductItemMobile",
  component: ProductItemMobile,
  globals: MOBILE,
  parameters: { ...storyDocs("product-item-mobile"), ...MOBILE_PAGE },
  tags: ["autodocs"],
  args: { product: RETAIL_CATALOG[0], onClick: fn() },
} satisfies Meta<typeof ProductItemMobile>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Tappable row: identity, price per unit, ready stock, chevron. */
export const Default: Story = {};

/** No onClick: a plain row — no chevron, not a button. */
export const ReadOnly: Story = { args: { onClick: undefined } };

/** Zero ready stock reads red. */
export const OutOfStock: Story = {
  args: { product: { ...RETAIL_CATALOG[0], readyStock: 0n } },
};

/** Long names clamp to two lines; the price + stock columns never get squeezed off. */
export const LongName: Story = {
  args: {
    product: {
      ...RETAIL_CATALOG[0],
      name: "Minyak Goreng Bimoli Spesial 2 L Pouch Isi Ulang Hemat Keluarga Besar",
    },
  },
};

/** Archived products carry a badge. */
export const Archived: Story = {
  args: { product: { ...RETAIL_CATALOG[0], active: false } },
};

/** Stock pre-formatted by the caller (the Units popover's box/strip choice). */
export const CustomStockLabel: Story = {
  args: { stockLabel: "8 dus" },
};

/** As a list: a page of rows with hairlines between them. */
export const List: Story = {
  render: (args) => (
    <Stack gap={0} separator={<StackSeparator />}>
      {[...RETAIL_CATALOG.slice(0, 6), ...PHARMACY_CATALOG.slice(0, 2)].map((p) => (
        <ProductItemMobile key={p.id} product={p} onClick={args.onClick} />
      ))}
    </Stack>
  ),
};

/** Loading placeholder: one skeleton row in the same shape as a real row. */
export const LoadingSkeleton: Story = {
  render: () => <ProductItemMobileSkeleton chevron />,
};

/** Loading a page: skeleton rows with the same hairlines as List. */
export const LoadingSkeletonList: Story = {
  render: () => (
    <Stack gap={0} separator={<StackSeparator />}>
      {Array.from({ length: 8 }, (_, i) => (
        <ProductItemMobileSkeleton key={i} chevron />
      ))}
    </Stack>
  ),
};
