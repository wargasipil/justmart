import { Button, Field, HStack, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import { ProductPriceTier } from "../../gen/inventory_iface/v1/product_price_tier_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useProductQuery } from "../../queries/products";
import {
  useCreateProductPriceTierMutation,
  useUpdateProductPriceTierMutation,
} from "../../queries/productPriceTiers";

// minQty >= 2 mirrors the backend rule (and its DB CHECK): a 0/1 rung would just
// be the unit's normal price, and tier_min_qty = 0 is the "no grosir" flag on a
// sale line. No inline messages — the global Zod error map translates.
const Schema = z.object({
  productUnitId: z.string().min(1),
  minQty: z.coerce.number().int().min(2),
  price: z.coerce.bigint().min(0n),
});
type FormValues = z.infer<typeof Schema>;

const EMPTY: FormValues = { productUnitId: "", minQty: 2, price: 0n };

export default function ProductPriceTierDrawer({
  open,
  onClose,
  productId,
  editing,
}: {
  open: boolean;
  onClose: () => void;
  productId: string;
  editing: ProductPriceTier | null;
}) {
  const { t } = useTranslation();
  const create = useCreateProductPriceTierMutation();
  const update = useUpdateProductPriceTierMutation();
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: EMPTY });
  const applyServerError = useServerFormErrors(form);

  // A tier prices one specific SELLABLE unit, so the picker excludes units that
  // can't be sold (a wholesale price on them would be unreachable at POS).
  const productQ = useProductQuery(productId, open && !!productId);
  const units = useMemo(
    () => (productQ.data?.units ?? []).filter((u) => u.active && u.sellable),
    [productQ.data],
  );
  const baseUnitId = units.find((u) => u.isBase)?.id ?? "";

  useEffect(() => {
    if (!open) return;
    form.reset(
      editing
        ? { productUnitId: editing.productUnitId, minQty: editing.minQty, price: editing.price }
        : { ...EMPTY, productUnitId: baseUnitId },
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing, baseUnitId]);

  const unitId = form.watch("productUnitId");
  const price = form.watch("price");
  const unit = units.find((u) => u.id === unitId);
  const saving = unit ? unit.sellPrice - BigInt(price || 0n) : 0n;

  const submit = form.handleSubmit(async (v) => {
    try {
      if (editing) {
        await update.mutateAsync({
          id: editing.id,
          productUnitId: v.productUnitId,
          minQty: v.minQty,
          price: v.price,
        });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync({
          productId,
          productUnitId: v.productUnitId,
          minQty: v.minQty,
          price: v.price,
        });
        toast.success(t("common.create") + " ✓");
      }
      onClose();
    } catch (err) {
      applyServerError(err);
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={editing ? t("priceTiers.editTitle") : t("priceTiers.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending || update.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          {/* EnumSelect isn't FormField-integrated, so wrap it in Field.Root —
              otherwise a server product_price_tier.unit_invalid would attach to a
              field with nowhere to render. */}
          <Field.Root required invalid={!!form.formState.errors.productUnitId}>
            <Field.Label>
              {t("priceTiers.unit")}
              <Field.RequiredIndicator />
            </Field.Label>
            <EnumSelect
              value={unitId}
              onChange={(v) => form.setValue("productUnitId", v, { shouldValidate: true })}
              items={units}
              itemToValue={(u) => u.id}
              itemToString={(u) => (Number(u.factor) > 1 ? `${u.name} ×${u.factor}` : u.name)}
              placeholder={t("priceTiers.unit")}
              disabled={units.length === 0}
            />
            {form.formState.errors.productUnitId && (
              <Field.ErrorText>{form.formState.errors.productUnitId.message}</Field.ErrorText>
            )}
          </Field.Root>

          <FormField control={form.control} name="minQty" label={t("priceTiers.minQty")} number required />

          <Stack gap={1}>
            <FormField control={form.control} name="price" label={t("priceTiers.price")} money required />
            {unit && (
              <Text fontSize="xs" color="fg.muted" pl={1}>
                {t("priceTiers.vsNormal", { price: formatMoney(unit.sellPrice), unit: unit.name })}
                {saving > 0n ? ` · ${t("priceTiers.saving")} ${formatMoney(saving)}` : ""}
              </Text>
            )}
          </Stack>
        </Stack>
      </form>
    </EntityDrawer>
  );
}
