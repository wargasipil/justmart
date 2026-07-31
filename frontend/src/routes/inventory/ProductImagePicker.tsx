import { Button, HStack, IconButton, Stack, Text } from "@chakra-ui/react";
import { Camera, Trash2 } from "lucide-react";
import { useRef, useState, type ChangeEvent } from "react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import ProductImage from "../../components/ProductImage";
import { ACCEPT_ATTR, ImageRenditionError } from "../../lib/imageRenditions";
import { toast } from "../../lib/toaster";
import {
  useDeleteProductImageMutation,
  useUploadProductImageMutation,
} from "../../queries/products";

type Props = {
  productId: string;
  name: string;
  /** The product's imageUpdatedAt. 0 = no picture yet. */
  version: number;
};

// A product's picture + its Change/Remove actions. The two renditions (original
// + thumbnail) are produced inside useUploadProductImageMutation, so this
// component only ever hands over the raw File — see the two-rendition HARD RULE.
// Mirrors routes/profile/AvatarPicker.tsx.
export default function ProductImagePicker({ productId, name, version }: Props) {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const upload = useUploadProductImageMutation();
  const remove = useDeleteProductImageMutation();

  const onFileChange = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    // Reset immediately so re-picking the SAME file fires change again.
    e.target.value = "";
    if (!file) return;
    try {
      await upload.mutateAsync({ productId, file });
      toast.success(t("inventory.products.imageUpdated"));
    } catch (err) {
      // Client-side rejections (wrong type, too big, undecodable) carry their
      // own i18n key; anything else is a server error and goes through the
      // shared translator.
      if (err instanceof ImageRenditionError) toast.error(t(err.i18nKey));
      else toast.fromError(err);
    }
  };

  return (
    <Stack gap={3} align="center">
      {/* The one deliberate full-size surface: `full` reads the ORIGINAL. */}
      <ProductImage productId={productId} name={name} version={version} size={200} full />
      <HStack gap={1}>
        <Button
          size="xs"
          variant="outline"
          onClick={() => inputRef.current?.click()}
          loading={upload.isPending}
        >
          <Camera size={13} />
          {t(version > 0 ? "inventory.products.imageChange" : "inventory.products.imageUpload")}
        </Button>
        {version > 0 && (
          <IconButton
            size="xs"
            variant="ghost"
            aria-label={t("inventory.products.imageRemove")}
            onClick={() => setConfirmRemove(true)}
            loading={remove.isPending}
          >
            <Trash2 size={13} />
          </IconButton>
        )}
      </HStack>
      <Text fontSize="2xs" color="fg.muted" textAlign="center" maxW="220px">
        {t("inventory.products.imageHelp")}
      </Text>

      {/* Hidden native input: a file picker is OS chrome by necessity, so this
          is not a "no native dialogs" violation (same call as ImportProducts). */}
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT_ATTR}
        style={{ display: "none" }}
        onChange={onFileChange}
      />

      <ConfirmDialog
        open={confirmRemove}
        title={t("inventory.products.imageRemove")}
        body={t("inventory.products.imageRemoveConfirm")}
        confirmLabel={t("common.delete")}
        loading={remove.isPending}
        onCancel={() => setConfirmRemove(false)}
        onConfirm={async () => {
          await remove.mutateAsync(productId);
          setConfirmRemove(false);
        }}
      />
    </Stack>
  );
}
