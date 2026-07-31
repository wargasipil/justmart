import {
  Box,
  chakra,
  Dialog,
  Heading,
  HStack,
  IconButton,
  Image,
  Portal,
  Spinner,
  Text,
} from "@chakra-ui/react";
import { Package, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { ProductImageVariant } from "../gen/inventory_iface/v1/product_pb";
import { useProductImageQuery } from "../queries/products";

type Props = {
  productId: string;
  /** Display name — used as the alt text (and the lightbox title). */
  name: string;
  /**
   * The product's `imageUpdatedAt` (unix sec). 0 = no picture, which skips the
   * fetch entirely and renders the placeholder. Also the cache key, so a
   * re-upload shows up without an invalidate.
   */
  version: number;
  /** Rendered box size in px (square). */
  size?: number;
  /**
   * Load the full-size ORIGINAL instead of the thumbnail. Only for a deliberate
   * full-size view — see the two-rendition HARD RULE in CLAUDE.md. Every table
   * cell and list row leaves this off.
   */
  full?: boolean;
  /**
   * Clicking opens a lightbox showing the ORIGINAL. Ignored when there is no
   * picture (`version === 0`) — a placeholder must not look interactive.
   * The click is stopPropagation'd, so this is safe inside a clickable row.
   */
  zoomable?: boolean;
};

/**
 * A product's picture with a neutral box placeholder.
 *
 * Reads the THUMB rendition by default, so a 25-row product list pulls a few KB
 * per row instead of a few hundred. Products with no picture (`version === 0`)
 * cost zero requests — the query is disabled, not fetched-then-discarded.
 *
 * `objectFit="contain"` on purpose: the thumbnail is already a centre-cropped
 * square, and a product shot is about recognising the item — cropping it twice
 * would cut the label off.
 */
export default function ProductImage({
  productId,
  name,
  version,
  size = 40,
  full,
  zoomable,
}: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const q = useProductImageQuery(
    productId,
    version,
    full ? ProductImageVariant.ORIGINAL : ProductImageVariant.THUMB,
  );
  const src = q.data ?? "";
  const canZoom = !!zoomable && version > 0;

  const box = (
    <Box
      w={`${size}px`}
      h={`${size}px`}
      flexShrink={0}
      borderRadius="md"
      borderWidth="1px"
      borderColor="border.muted"
      bg="bg.muted"
      overflow="hidden"
      display="flex"
      alignItems="center"
      justifyContent="center"
      color="fg.muted"
    >
      {src ? (
        <Image src={src} alt={name} w="100%" h="100%" objectFit="contain" />
      ) : (
        <Package size={Math.max(12, Math.round(size * 0.45))} />
      )}
    </Box>
  );

  if (!canZoom) return box;

  return (
    <>
      {/* chakra.button, not `Box as="button"`: the polymorphic Box drops native
          button props, and `type="button"` matters so this can never submit a
          form it happens to be rendered inside. */}
      <chakra.button
        type="button"
        aria-label={t("inventory.products.imagePreview")}
        display="block"
        borderRadius="md"
        cursor="zoom-in"
        // The list row this sits in navigates on click — swallow the event so
        // opening the preview does not also leave the page.
        onClick={(e) => {
          e.stopPropagation();
          setOpen(true);
        }}
        _focusVisible={{
          outline: "2px solid",
          outlineColor: "colorPalette.focusRing",
          outlineOffset: "2px",
        }}
        colorPalette="blue"
      >
        {box}
      </chakra.button>
      <ProductImageLightbox
        productId={productId}
        name={name}
        version={version}
        open={open}
        onClose={() => setOpen(false)}
      />
    </>
  );
}

/**
 * Full-size preview. `Dialog.Root` stays mounted and is driven purely by `open`
 * (never `{open && …}` — see the dialog-mount HARD RULE in CLAUDE.md); only its
 * CONTENT is lazy, via lazyMount/unmountOnExit, which is what keeps 25 closed
 * lightboxes on a list page free.
 */
function ProductImageLightbox({
  productId,
  name,
  version,
  open,
  onClose,
}: {
  productId: string;
  name: string;
  version: number;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  // The ORIGINAL is fetched only while this is open, so a list of thumbnails
  // never pulls the heavy renditions.
  const q = useProductImageQuery(productId, version, ProductImageVariant.ORIGINAL, open);
  const src = q.data ?? "";

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      size="lg"
      lazyMount
      unmountOnExit
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header borderBottomWidth="1px">
              <HStack justify="space-between" w="full">
                <Heading size="md">{name}</Heading>
                <IconButton
                  aria-label={t("common.close")}
                  variant="ghost"
                  size="sm"
                  onClick={onClose}
                >
                  <X size={18} />
                </IconButton>
              </HStack>
            </Dialog.Header>
            <Dialog.Body>
              <Box
                display="flex"
                alignItems="center"
                justifyContent="center"
                minH="240px"
                bg="bg.muted"
                borderRadius="md"
              >
                {q.isLoading ? (
                  <Spinner />
                ) : src ? (
                  <Image
                    src={src}
                    alt={name}
                    maxW="100%"
                    maxH="70vh"
                    objectFit="contain"
                  />
                ) : (
                  <Text fontSize="sm" color="fg.muted">
                    {t("common.noResults")}
                  </Text>
                )}
              </Box>
            </Dialog.Body>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
