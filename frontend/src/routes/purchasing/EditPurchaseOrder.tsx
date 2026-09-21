import { Alert, Spinner, Stack } from "@chakra-ui/react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import PurchaseOrderForm, { type PurchaseOrderSeed } from "./PurchaseOrderForm";
import { POStatus } from "../../gen/purchasing_iface/v1/order_pb";
import { lineFromItem } from "../../lib/purchaseLine";
import { useProductsWithUnitsQuery } from "../../queries/products";
import { usePurchaseOrderQuery } from "../../queries/purchasing";
import { useSupplierRefs } from "../../queries/refs";

/**
 * Edit an existing restock order.
 *
 * Only DRAFT is editable — once the order is SENT the supplier has the
 * document, so the backend refuses (UpdatePurchaseOrder, FailedPrecondition)
 * and this page says so rather than presenting a form that cannot save.
 *
 * The page's whole job is to assemble a complete seed BEFORE mounting the form:
 * the saved lines carry base-unit quantities and per-base costs, and turning
 * those back into editable per-unit figures needs each product's unit list. So
 * the form is held back until the products have loaded and then mounted under a
 * `key` — it reads its seed once, at mount, and a later refetch must never
 * overwrite what the user is typing.
 */
export default function EditPurchaseOrder() {
  const { t } = useTranslation();
  const { id = "" } = useParams();

  const poQ = usePurchaseOrderQuery(id);
  const po = poQ.data;
  const isDraft = po?.status === POStatus.PO_STATUS_DRAFT;

  // Only a DRAFT is ever edited, so nothing else pays for the product hydration.
  const productIds = useMemo(
    () => (isDraft ? (po?.items ?? []).map((i) => i.productId) : []),
    [isDraft, po],
  );
  const productsQ = useProductsWithUnitsQuery(productIds, isDraft);
  const supplierRefs = useSupplierRefs(po ? [po.supplierId] : []);

  const seed: PurchaseOrderSeed | undefined = useMemo(() => {
    if (!po || !isDraft || !productsQ.isReady) return undefined;
    return {
      supplierId: po.supplierId,
      invoiceNo: po.invoiceNo,
      invoiceDate: po.invoiceDate,
      dueAt: po.dueAt,
      note: po.note,
      lines: po.items.map((it) => lineFromItem(it, productsQ.map.get(it.productId))),
      cartDiscount: Number(po.cartDiscount),
      // 0 means "never set"; the form offers the same default a new order gets.
      ppnRate: po.ppnRate || 11,
      ppnEnabled: po.ppnEnabled,
    };
  }, [po, isDraft, productsQ.isReady, productsQ.map]);

  if (poQ.isLoading) return <Spinner />;
  if (!po) return null;

  const back = `/purchasing/${id}`;

  if (!isDraft) {
    return (
      <Stack gap={4}>
        <BackButton to={back} />
        <Alert.Root status="warning">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Description>{t("purchasing.editOnlyDraft")}</Alert.Description>
          </Alert.Content>
        </Alert.Root>
      </Stack>
    );
  }

  // A failed hydration would seed every line at factor 1 and silently re-price
  // the order, so it blocks the form rather than degrading it.
  if (productsQ.isError) {
    return (
      <Stack gap={4}>
        <BackButton to={back} />
        <Alert.Root status="error">
          <Alert.Indicator />
          <Alert.Content>
            <Alert.Description>{t("purchasing.editLoadFailed")}</Alert.Description>
          </Alert.Content>
        </Alert.Root>
      </Stack>
    );
  }

  if (!seed) return <Spinner />;

  return (
    <Stack gap={4}>
      <BackButton to={back} />
      <PurchaseOrderForm
        key={po.id}
        poId={po.id}
        seed={seed}
        supplierName={supplierRefs.get(po.supplierId)?.name}
      />
    </Stack>
  );
}
