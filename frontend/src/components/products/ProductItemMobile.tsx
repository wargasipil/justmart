import {
  Badge,
  Box,
  HStack,
  Skeleton,
  SkeletonText,
  Stack,
  Text,
  chakra,
} from "@chakra-ui/react";
import { ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../../lib/format";
import ProductImage from "../ProductImage";

export type ProductItemMobileData = Pick<
  Product,
  "id" | "name" | "sku" | "imageUpdatedAt" | "unit" | "unitPrice" | "readyStock" | "active"
>;

type Props = {
  product: ProductItemMobileData;
  /** Tapping the row. Omit for a read-only row (no chevron, not a button). */
  onClick?: () => void;
  /**
   * Pre-formatted ready stock (e.g. from formatStock with the Units popover's
   * choice). Defaults to "<readyStock> <unit>".
   */
  stockLabel?: string;
};

// A product as one phone-width row: the table's columns don't fit at 390px,
// so the figures a shopper-facing lookup needs — sell price and ready stock —
// ride on the row itself. Sell-side data only: nothing here is cost, so the
// same row is safe for the till.
export default function ProductItemMobile({ product, onClick, stockLabel }: Props) {
  const { t } = useTranslation();
  const out = product.readyStock <= 0n;
  const stock = stockLabel ?? `${product.readyStock} ${product.unit}`.trim();

  const body = (
    <HStack gap={3} align="center" w="full" minW={0} py={2}>
      <ProductImage
        productId={product.id}
        name={product.name}
        version={Number(product.imageUpdatedAt)}
        size={48}
      />
      <Stack gap={0.5} flex="1" minW={0}>
        <Text fontWeight="medium" lineClamp={2}>
          {product.name}
        </Text>
        <Text fontSize="xs" color="fg.muted" fontFamily="mono" truncate>
          {product.sku}
        </Text>
        <HStack gap={2} fontSize="sm" flexWrap="wrap">
          <Text fontWeight="semibold">
            {formatMoney(product.unitPrice)}
            {product.unit && (
              <Text as="span" color="fg.muted" fontWeight="normal">
                {" "}/ {product.unit}
              </Text>
            )}
          </Text>
          {!product.active && (
            <Badge size="sm" colorPalette="gray">
              {t("common.archived")}
            </Badge>
          )}
        </HStack>
      </Stack>
      <Stack gap={0} align="flex-end" flexShrink={0}>
        <Text fontSize="xs" color="fg.muted">
          {t("inventory.products.readyStock")}
        </Text>
        <Text fontSize="sm" fontFamily="mono" color={out ? "red.fg" : undefined}>
          {stock}
        </Text>
      </Stack>
      {onClick && (
        <Box color="fg.muted" flexShrink={0}>
          <ChevronRight size={16} />
        </Box>
      )}
    </HStack>
  );

  if (!onClick) return body;
  return (
    <chakra.button
      type="button"
      onClick={onClick}
      w="full"
      textAlign="start"
      px={3}
      borderRadius="md"
      _hover={{ bg: "bg.muted" }}
      _focusVisible={{ outline: "2px solid", outlineColor: "colorPalette.focusRing" }}
    >
      {body}
    </chakra.button>
  );
}

// The loading placeholder for one ProductItemMobile row: same 48px thumbnail,
// same text stack and right-hand stock column, so rows swap in without the
// list jumping. Pass `chevron` when the real rows will be tappable.
export function ProductItemMobileSkeleton({ chevron }: { chevron?: boolean }) {
  return (
    <HStack gap={3} align="center" w="full" minW={0} py={2} px={chevron ? 3 : 0}>
      <Skeleton boxSize="48px" borderRadius="md" flexShrink={0} />
      <Stack gap={1.5} flex="1" minW={0}>
        <SkeletonText noOfLines={1} w="70%" />
        <Skeleton h="3" w="30%" />
        <Skeleton h="3.5" w="40%" />
      </Stack>
      <Stack gap={1.5} align="flex-end" flexShrink={0}>
        <Skeleton h="3" w="10" />
        <Skeleton h="3.5" w="14" />
      </Stack>
      {chevron && <Box w="16px" flexShrink={0} />}
    </HStack>
  );
}
