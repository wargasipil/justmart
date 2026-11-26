import { Alert, Button, HStack, Stack } from "@chakra-ui/react";
import { useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { useAuth } from "../../lib/auth";
import { formatDateTime } from "../../lib/format";
import type { PurchaseOrderSeed } from "../../lib/purchaseLine";
import {
  type RestockDraft,
  clearRestockDraft,
  loadRestockDraft,
  saveRestockDraft,
} from "../../lib/restockDraft";
import { useSupplierRefs } from "../../queries/refs";
import PurchaseOrderForm from "./PurchaseOrderForm";

// The create route. The form body is shared with /purchasing/:id/edit — it
// lives in PurchaseOrderForm so the two can never drift into two different
// ideas of what a restock line is.
//
// What this route adds is the KEPT DRAFT (lib/restockDraft.ts): everything
// typed is kept in this browser as it is typed, so leaving the page or a
// refresh does not lose a half-entered faktur. A kept draft is OFFERED, never
// applied unasked — the person opening the form may be starting a different
// order. While the offer is open nothing is saved, so ignoring it cannot
// overwrite the draft it offers; Lanjutkan remounts the form seeded from it,
// Buang deletes it. A successful Buat (or Batal) clears the draft.
export default function NewPurchaseOrder() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const userId = user?.id ?? "";
  const [offer, setOffer] = useState<RestockDraft | null>(() => loadRestockDraft(userId));
  const [seed, setSeed] = useState<PurchaseOrderSeed | undefined>(undefined);
  // The form reads its seed once, at mount; a new key is what applies one.
  const [formKey, setFormKey] = useState(0);

  const supplierIds = useMemo(
    () => (offer?.seed.supplierId ? [offer.seed.supplierId] : []),
    [offer],
  );
  const supplierRefs = useSupplierRefs(supplierIds);

  const onSnapshot = useCallback(
    (s: PurchaseOrderSeed) => {
      if (!offer) saveRestockDraft(userId, s);
    },
    [offer, userId],
  );
  const onFinish = useCallback(() => {
    if (!offer) clearRestockDraft(userId);
  }, [offer, userId]);

  const resume = () => {
    if (!offer) return;
    setSeed(offer.seed);
    setOffer(null);
    setFormKey((k) => k + 1);
  };
  const discard = () => {
    clearRestockDraft(userId);
    setOffer(null);
  };

  const summary = offer
    ? [
        offer.seed.supplierId ? supplierRefs.get(offer.seed.supplierId)?.name : "",
        t("purchasing.draftKept.items", { count: offer.seed.lines.length }),
        t("purchasing.draftKept.savedAt", { when: formatDateTime(offer.savedAt) }),
      ]
        .filter(Boolean)
        .join(" · ")
    : "";

  return (
    <Stack gap={4}>
      {offer && (
        <Alert.Root status="info" flexWrap="wrap" alignItems="center">
          <Alert.Indicator />
          <Alert.Content minW="12rem">
            <Alert.Title>{t("purchasing.draftKept.title")}</Alert.Title>
            <Alert.Description>{summary}</Alert.Description>
          </Alert.Content>
          <HStack gap={2} ms="auto">
            <Button size="sm" colorPalette="blue" onClick={resume}>
              {t("purchasing.draftKept.resume")}
            </Button>
            <Button size="sm" variant="ghost" onClick={discard}>
              {t("purchasing.draftKept.discard")}
            </Button>
          </HStack>
        </Alert.Root>
      )}
      <PurchaseOrderForm key={formKey} seed={seed} onSnapshot={onSnapshot} onFinish={onFinish} />
    </Stack>
  );
}
