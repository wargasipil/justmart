import { Button, Field, HStack, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { Controller, useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import SearchableSelect from "../../components/SearchableSelect";
import type { ProductRecipeItem } from "../../gen/inventory_iface/v1/product_recipe_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { searchProducts } from "../../queries/products";
import {
  useCreateProductRecipeItemMutation,
  useUpdateProductRecipeItemMutation,
} from "../../queries/productRecipes";

const Schema = z.object({
  componentProductId: z.string().min(1),
  qtyBase: z.coerce.number().int().positive(),
  note: z.string(),
});
type FormValues = z.infer<typeof Schema>;

// Add / edit one ingredient line.
//
// The COMPONENT is immutable on edit, mirroring the backend: swapping it in
// place would silently redirect what a sale deducts while the line keeps its
// identity — an edit in the UI, a different ingredient in reality. Remove and
// re-add instead.
export default function ProductRecipeDrawer({
  productId,
  item,
  open,
  onClose,
}: {
  productId: string;
  item?: ProductRecipeItem | null;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!item;
  const create = useCreateProductRecipeItemMutation();
  const update = useUpdateProductRecipeItemMutation();

  const values = useMemo<FormValues>(
    () => ({
      componentProductId: item?.componentProductId ?? "",
      qtyBase: Number(item?.qtyBase ?? 1),
      note: item?.note ?? "",
    }),
    [item],
  );
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), values });
  const onServerError = useServerFormErrors(form);

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      if (isEdit && item) {
        await update.mutateAsync({ id: item.id, qtyBase: BigInt(v.qtyBase), note: v.note });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync({
          productId,
          componentProductId: v.componentProductId,
          qtyBase: BigInt(v.qtyBase),
          note: v.note,
        });
        toast.success(t("common.create") + " ✓");
      }
      onClose();
    } catch (err) {
      // product_recipe.component_taken → field error on componentProductId
      onServerError(err);
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={
        isEdit
          ? t("inventory.products.recipe.editTitle")
          : t("inventory.products.recipe.add")
      }
      footer={
        <HStack justify="space-between" w="100%">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={onSubmit}
            loading={create.isPending || update.isPending}
          >
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={3}>
        <Controller
          control={form.control}
          name="componentProductId"
          render={({ field, fieldState }) => (
            <Field.Root invalid={!!fieldState.error}>
              <Field.Label>{t("inventory.products.recipe.component")}</Field.Label>
              {/* Server-side search (loadOptions), per the dynamic-select HARD
                  RULE — the ingredient list grows with the catalog. */}
              <SearchableSelect
                value={field.value}
                onChange={field.onChange}
                loadOptions={searchProducts}
                itemToString={(p) => `${p.name} · ${p.sku}`}
                itemToValue={(p) => p.id}
                selectedLabel={item?.componentName}
                disabled={isEdit}
              />
              {fieldState.error?.message && (
                <Field.ErrorText>{fieldState.error.message}</Field.ErrorText>
              )}
            </Field.Root>
          )}
        />
        <FormField
          control={form.control}
          name="qtyBase"
          label={t("inventory.products.recipe.qtyBase")}
          helperText={t("inventory.products.recipe.qtyBaseHelp")}
          type="number"
          required
        />
        <FormField
          control={form.control}
          name="note"
          label={t("inventory.products.recipe.note")}
        />
        {isEdit && item?.componentUnit && (
          <Text fontSize="xs" color="fg.muted">
            {item.componentName} · {item.componentUnit}
          </Text>
        )}
      </Stack>
    </EntityDrawer>
  );
}
