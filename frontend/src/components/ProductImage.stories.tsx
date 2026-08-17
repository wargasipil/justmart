import { HStack, Stack, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import ProductImage from "./ProductImage";
import { storyDocs } from "../routes/dev/storyDocs";

// `version={0}` means "no picture" and disables the fetch, so these stories
// render the neutral placeholder and make no request.

const meta = {
  title: "Data display/ProductImage",
  component: ProductImage,
  parameters: storyDocs("product-image"),
  tags: ["autodocs"],
  args: { productId: "demo-1", name: "Paracetamol 500mg", version: 0, size: 56 },
} satisfies Meta<typeof ProductImage>;

export default meta;
type Story = StoryObj<typeof meta>;

/** No picture → the neutral box placeholder, and zero requests. */
export const Placeholder: Story = {};

export const Sizes: Story = {
  render: () => (
    <HStack gap={4} align="center" flexWrap="wrap">
      {[28, 36, 56, 96].map((size) => (
        <Stack key={size} gap={1} align="center">
          <ProductImage productId={`demo-${size}`} name="Paracetamol" version={0} size={size} />
          <Text fontSize="xs" color="fg.muted">
            {size}px
          </Text>
        </Stack>
      ))}
    </HStack>
  ),
};

/**
 * `zoomable` is the cheap way to offer the original: the lightbox fetches
 * ORIGINAL only WHILE OPEN, so a list of 25 thumbnails never pulls the heavy
 * bytes. Ignored at `version: 0`, and the click is stopPropagation'd so it is
 * safe inside a clickable table row.
 */
export const Zoomable: Story = {
  args: { zoomable: true, size: 96 },
};

/**
 * objectFit is `contain`, not `cover` — the thumb is already centre-cropped
 * square, and cropping a product shot twice cuts the label off.
 */
export const FullRendition: Story = {
  args: { full: true, size: 120 },
};
