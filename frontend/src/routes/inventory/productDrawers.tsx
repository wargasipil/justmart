import { Button, HStack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";

import EntityDialog from "../../components/EntityDialog";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useCreateProductMutation, useUpdateProductMutation } from "../../queries/products";
import ProductForm, {
  Schema,
  nonBaseDrafts,
  toUnitInputs,
  type FormValues,
  type UnitDraft,
} from "./ProductFormFields";

// Create + edit dialogs for a product. The schema and the field stack they both
// render live in ProductFormFields.tsx.

export function CreateProductDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateProductMutation();
  const [units, setUnits] = useState<UnitDraft[]>([]);
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: {
      sku: "",
      name: "",
      unit: "tablet",
      unitPrice: 0n,
      prescriptionRequired: false,
      manufacturerId: "",
    },
  });
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync({ ...values, units: toUnitInputs(units) });
      toast.success(t("common.create") + " ✓");
      form.reset();
      setUnits([]);
      onClose();
    } catch (err) {
      onServerError(err); // product.sku_taken → field error on `sku`
    }
  });

  return (
    <EntityDialog
      open={open}
      onClose={onClose}
      title={t("inventory.products.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <ProductForm form={form} units={units} setUnits={setUnits} referenceCost={0n} isCreate />
      </form>
    </EntityDialog>
  );
}

export function EditProductDialog({
  product,
  onClose,
}: {
  product: Product | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const update = useUpdateProductMutation();
  const [units, setUnits] = useState<UnitDraft[]>([]);
  // Re-seed the unit drafts whenever the edited product changes.
  useEffect(() => {
    setUnits(nonBaseDrafts(product));
  }, [product]);

  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    values: product
      ? {
          sku: product.sku,
          name: product.name,
          unit: product.unit,
          unitPrice: product.unitPrice,
          prescriptionRequired: product.prescriptionRequired,
          manufacturerId: product.manufacturerId,
        }
      : undefined,
  });
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (values) => {
    if (!product) return;
    try {
      await update.mutateAsync({
        id: product.id,
        sku: values.sku,
        name: values.name,
        unit: values.unit,
        unitPrice: values.unitPrice,
        prescriptionRequired: values.prescriptionRequired,
        manufacturerId: values.manufacturerId,
        units: toUnitInputs(units),
      });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch (err) {
      onServerError(err);
    }
  });

  return (
    <EntityDialog
      open={!!product}
      onClose={onClose}
      title={product ? `${t("inventory.products.editTitle")} · ${product.sku}` : ""}
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
        <ProductForm
          form={form}
          units={units}
          setUnits={setUnits}
          referenceCost={product?.referenceCost ?? 0n}
          isCreate={false}
        />
      </form>
    </EntityDialog>
  );
}
