import { Box, Button, HStack, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import { ProductDiscount } from "../../gen/inventory_iface/v1/product_discount_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useProductQuery } from "../../queries/products";
import {
  type DiscountModeValue,
  modeParts,
  modeValue,
  useCreateProductDiscountMutation,
  useUpdateProductDiscountMutation,
} from "../../queries/productDiscounts";

const MODES: DiscountModeValue[] = ["FIXED", "PERCENT", "FIXED_ITEM", "PERCENT_ITEM"];

const Schema = z.object({
  mode: z.enum(["FIXED", "PERCENT", "FIXED_ITEM", "PERCENT_ITEM"]),
  value: z.coerce.number().min(0), // FIXED* = rupiah; PERCENT* = percent (0–100)
  minQty: z.coerce.number().min(0),
  minQtyUnitId: z.string(), // "" = base unit
  expiresAt: z.string(),
});
type FormValues = z.infer<typeof Schema>;

const EMPTY: FormValues = { mode: "PERCENT", value: 0, minQty: 0, minQtyUnitId: "", expiresAt: "" };

export default function ProductDiscountDrawer({
  open,
  onClose,
  productId,
  editing,
}: {
  open: boolean;
  onClose: () => void;
  productId: string;
  editing: ProductDiscount | null;
}) {
  const { t } = useTranslation();
  const create = useCreateProductDiscountMutation();
  const update = useUpdateProductDiscountMutation();
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: EMPTY });
  const applyServerError = useServerFormErrors(form);

  // The product's active units drive the min-qty unit picker ("" = base unit).
  const productQ = useProductQuery(productId, open && !!productId);
  const units = useMemo(() => (productQ.data?.units ?? []).filter((u) => u.active), [productQ.data]);

  useEffect(() => {
    if (!open) return;
    if (editing) {
      form.reset({
        mode: modeValue(editing.discountType, editing.perItem),
        value: editing.discountType === "PERCENT" ? Number(editing.value) / 100 : Number(editing.value),
        minQty: editing.minQty,
        minQtyUnitId: editing.minQtyUnitId,
        expiresAt: editing.expiresAt,
      });
    } else {
      form.reset(EMPTY);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing]);

  const mode = form.watch("mode");
  const minQtyUnitId = form.watch("minQtyUnitId");
  const isPercent = mode === "PERCENT" || mode === "PERCENT_ITEM";

  const submit = form.handleSubmit(async (v) => {
    const { discountType, perItem } = modeParts(v.mode);
    const value = BigInt(discountType === "PERCENT" ? Math.round(v.value * 100) : Math.round(v.value));
    try {
      if (editing) {
        await update.mutateAsync({
          id: editing.id, discountType, perItem, value,
          minQty: v.minQty, minQtyUnitId: v.minQtyUnitId, expiresAt: v.expiresAt,
        });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync({
          productId, discountType, perItem, value,
          minQty: v.minQty, minQtyUnitId: v.minQtyUnitId, expiresAt: v.expiresAt,
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
      title={editing ? t("productDiscounts.editTitle") : t("productDiscounts.addTitle")}
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
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("productDiscounts.mode")} *
            </Text>
            <EnumSelect
              value={mode}
              onChange={(val) => form.setValue("mode", val as DiscountModeValue)}
              items={MODES}
              itemToValue={(m) => m}
              itemToString={(m) =>
                t(
                  m === "FIXED"
                    ? "productDiscounts.modeFixed"
                    : m === "PERCENT"
                      ? "productDiscounts.modePercent"
                      : m === "FIXED_ITEM"
                        ? "productDiscounts.modeFixedItem"
                        : "productDiscounts.modePercentItem",
                )
              }
            />
          </Stack>

          {isPercent ? (
            <FormField control={form.control} name="value" label={t("productDiscounts.valuePercent")} type="number" required />
          ) : (
            <FormField control={form.control} name="value" label={t("productDiscounts.valueFixed")} money required />
          )}

          <HStack gap={2} align="flex-end">
            <Box flex="1">
              <FormField control={form.control} name="minQty" label={t("productDiscounts.minQty")} type="number" />
            </Box>
            <Box width="140px">
              <EnumSelect
                value={minQtyUnitId}
                onChange={(v) => form.setValue("minQtyUnitId", v)}
                items={units}
                itemToValue={(u) => (u.isBase ? "" : u.id)}
                itemToString={(u) => (Number(u.factor) > 1 ? `${u.name} ×${u.factor}` : u.name)}
                placeholder={t("productDiscounts.minQtyUnit")}
                disabled={units.length === 0}
              />
            </Box>
          </HStack>
          <FormField control={form.control} name="expiresAt" label={t("productDiscounts.expiresAt")} type="date" />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
