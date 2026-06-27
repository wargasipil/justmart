import { Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import SearchableSelect from "../../components/SearchableSelect";
import { Button, HStack } from "@chakra-ui/react";
import { PriceAgreement } from "../../gen/inventory_iface/v1/price_agreement_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useProductQuery } from "../../queries/products";
import { useUpdatePriceAgreementMutation } from "../../queries/priceAgreements";
import { useProductRefs, useSupplierRefs } from "../../queries/refs";
import { searchProducts } from "../../queries/products";
import { searchSuppliers } from "../../queries/suppliers";

const Schema = z.object({
  productUnitId: z.string().min(1),
  price: z.coerce.bigint().min(0n),
  validFrom: z.string(),
  validUntil: z.string(),
  note: z.string(),
});
type FormValues = z.infer<typeof Schema>;

type Props = {
  open: boolean;
  onClose: () => void;
  editing: PriceAgreement | null;
};

// Edit-only drawer for a single supplier price agreement. Creating is now the
// dedicated multi-line page (NewPriceAgreement). Supplier + product are
// immutable; the unit/price/dates/note can change.
export default function PriceAgreementDrawer({ open, onClose, editing }: Props) {
  const { t } = useTranslation();
  const update = useUpdatePriceAgreementMutation();

  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: { productUnitId: "", price: 0n, validFrom: "", validUntil: "", note: "" },
  });
  const applyServerError = useServerFormErrors(form);

  // Reset the form whenever the drawer opens onto a record.
  useEffect(() => {
    if (!open || !editing) return;
    form.reset({
      productUnitId: editing.productUnitId,
      price: editing.price,
      validFrom: editing.validFrom,
      validUntil: editing.validUntil,
      note: editing.note,
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing]);

  const supplierId = editing?.supplierId ?? "";
  const productId = editing?.productId ?? "";
  const productUnitId = form.watch("productUnitId");

  // The product's units drive the unit options.
  const productQ = useProductQuery(productId, !!productId);
  const units = useMemo(
    () => (productQ.data?.units ?? []).filter((u) => u.active),
    [productQ.data],
  );

  // Locked labels for the read-only supplier/product pickers.
  const supplierRefs = useSupplierRefs(useMemo(() => (supplierId ? [supplierId] : []), [supplierId]));
  const productRefs = useProductRefs(useMemo(() => (productId ? [productId] : []), [productId]));

  const submit = form.handleSubmit(async (v) => {
    if (!editing) return;
    try {
      await update.mutateAsync({
        id: editing.id,
        productUnitId: v.productUnitId,
        price: v.price,
        validFrom: v.validFrom,
        validUntil: v.validUntil,
        note: v.note,
      });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch (err) {
      applyServerError(err);
    }
  });

  const supLabel = (() => {
    const s = supplierRefs.get(supplierId);
    return s ? `${s.code} · ${s.name}` : undefined;
  })();
  const prodLabel = productRefs.get(productId)?.name;

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.priceAgreements.editTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={update.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.priceAgreements.supplier")}
            </Text>
            <SearchableSelect
              value={supplierId}
              onChange={() => {}}
              loadOptions={searchSuppliers}
              itemToString={(s) => `${s.code} · ${s.name}`}
              itemToValue={(s) => s.id}
              selectedLabel={supLabel}
              disabled
            />
          </Stack>

          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.priceAgreements.product")}
            </Text>
            <SearchableSelect
              value={productId}
              onChange={() => {}}
              loadOptions={searchProducts}
              itemToString={(m) => `${m.sku} · ${m.name}`}
              itemToValue={(m) => m.id}
              selectedLabel={prodLabel}
              disabled
            />
          </Stack>

          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.priceAgreements.unit")} *
            </Text>
            <EnumSelect
              value={productUnitId}
              onChange={(v) => form.setValue("productUnitId", v)}
              items={units}
              itemToString={(u) => (Number(u.factor) > 1 ? `${u.name} ×${u.factor}` : u.name)}
              itemToValue={(u) => u.id}
              placeholder={t("inventory.priceAgreements.selectUnit")}
              disabled={units.length === 0}
            />
          </Stack>

          <FormField
            control={form.control}
            name="price"
            label={t("inventory.priceAgreements.pricePerUnit")}
            money
            required
          />
          <FormField control={form.control} name="validFrom" label={t("inventory.priceAgreements.validFrom")} type="date" />
          <FormField control={form.control} name="validUntil" label={t("inventory.priceAgreements.validUntil")} type="date" />
          <FormField control={form.control} name="note" label={t("inventory.priceAgreements.note")} />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
