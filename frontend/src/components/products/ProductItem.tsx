import { HStack, Stack, Text } from "@chakra-ui/react";
import type { ReactNode } from "react";

import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import ProductImage from "../ProductImage";

/** The fields ProductItem reads — any Product (or a fixture) satisfies it. */
export type ProductItemData = Pick<Product, "id" | "name" | "sku" | "imageUpdatedAt">;

type Props = {
  product: ProductItemData;
  /** Thumbnail edge in px. Default 56 (the Products table row). */
  size?: number;
  /** Clicking the thumbnail opens the ORIGINAL in a lightbox (see ProductImage). */
  zoomable?: boolean;
  /** Extra content on the SKU line, after the SKU (e.g. "· pcs"). */
  meta?: ReactNode;
};

// A product's identity: photo + name + SKU, read as one thing ("which product
// is this?"). It is the always-on name cell of the Products table; the phone
// row (ProductItemMobile) builds on it.
export default function ProductItem({ product, size = 56, zoomable, meta }: Props) {
  return (
    <HStack gap={3} minW={0}>
      <ProductImage
        productId={product.id}
        name={product.name}
        version={Number(product.imageUpdatedAt)}
        size={size}
        zoomable={zoomable}
      />
      <Stack gap={0} minW={0}>
        <Text>{product.name}</Text>
        <Text fontSize="xs" color="fg.muted" fontFamily="mono">
          {product.sku}
          {meta}
        </Text>
      </Stack>
    </HStack>
  );
}
