import { Badge, HStack, IconButton, Text } from "@chakra-ui/react";
import { Pencil } from "lucide-react";
import { useTranslation } from "react-i18next";

import ExpiryBadge from "../../components/ExpiryBadge";
import { ExpirySource, type Batch } from "../../gen/inventory_iface/v1/batch_pb";

// A lot's expiry as a table cell, shared by the Batches page and a product's
// Batches tab. Page-local to the inventory routes (both of its callers live
// here), so it is not part of components/.
//
// It reads the lot's expiry_source, which is the point: a "does not expire" lot
// shows as that rather than as a real date in 2099, and a date that is still
// the product's DEFAULT is marked, so a shelf check can tell an estimate from a
// date somebody read off the pack. `onEdit` (managers only) puts the
// correction one click from the value it corrects.
export default function BatchExpiryCell({ batch, onEdit }: { batch: Batch; onEdit?: () => void }) {
  const { t } = useTranslation();
  const none = batch.expirySource === ExpirySource.NONE;
  return (
    <HStack gap={2} wrap="wrap">
      {none ? (
        <Text color="fg.muted">{t("inventory.batches.noExpiry")}</Text>
      ) : (
        <>
          <Text>{batch.expiryDate}</Text>
          <ExpiryBadge expiry={batch.expiryDate} />
          {batch.expirySource === ExpirySource.DEFAULT && (
            <Badge variant="outline" colorPalette="gray" aria-label={t("inventory.batches.defaultTagHint")}>
              {t("inventory.batches.defaultTag")}
            </Badge>
          )}
        </>
      )}
      {onEdit && (
        <IconButton
          aria-label={t("inventory.batches.editExpiry")}
          variant="ghost"
          size="2xs"
          onClick={onEdit}
        >
          <Pencil size={12} />
        </IconButton>
      )}
    </HStack>
  );
}
