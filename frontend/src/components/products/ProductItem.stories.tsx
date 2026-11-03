import { Stack } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import ProductItem from "./ProductItem";
import { RETAIL_CATALOG } from "../../routes/dev/fixtures";
import { storyDocs } from "../../routes/dev/storyDocs";

// Fixture products carry imageUpdatedAt 0 ("no picture"), so these stories
// render the placeholder thumbnail and make no request.

const meta = {
  title: "components/products/ProductItem",
  component: ProductItem,
  parameters: storyDocs("product-item"),
  tags: ["autodocs"],
  args: { product: RETAIL_CATALOG[0], size: 56 },
} satisfies Meta<typeof ProductItem>;

export default meta;
type Story = StoryObj<typeof meta>;

/** As the Products table's name cell renders it. */
export const Default: Story = {};

/** Compact, with the unit on the SKU line — the picker-row shape. */
export const CompactWithMeta: Story = {
  args: { size: 28, meta: ` · ${RETAIL_CATALOG[0].unit}` },
};

/** A long name wraps inside a narrow cell instead of widening it. */
export const LongName: Story = {
  args: {
    product: { ...RETAIL_CATALOG[0], name: "Minyak Goreng Bimoli Spesial 2 L Pouch Isi Ulang Hemat" },
  },
  render: (args) => (
    <Stack maxW="220px">
      <ProductItem {...args} />
    </Stack>
  ),
};
