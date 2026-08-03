import { Button, HStack, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDrawer from "./EntityDrawer";
import FormField from "./FormField";
import type { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";
import { useServerFormErrors } from "../lib/formErrors";
import { useResetOnOpen } from "../lib/formReset";
import { toast } from "../lib/toaster";
import {
  useCreateWarehouseMutation,
  useUpdateWarehouseMutation,
} from "../queries/warehouses";

const Schema = z.object({
  code: z.string().min(1),
  name: z.string().min(1),
  address: z.string(),
  phone: z.string(),
});
type FormValues = z.infer<typeof Schema>;

// Shared create/edit drawer for warehouses. Used by both /warehouses (list)
// and /warehouses/:id (detail) so the form lives in one place.
export default function WarehouseDrawer({
  open,
  warehouse,
  onClose,
}: {
  open: boolean;
  warehouse?: Warehouse | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!warehouse;
  const create = useCreateWarehouseMutation();
  const update = useUpdateWarehouseMutation();

  // RHF `values` re-seeds the form whenever the warehouse prop changes (open for
  // a different row, or switch from create → edit) — no manual seed bookkeeping.
  const values = useMemo<FormValues>(
    () => ({
      code: warehouse?.code ?? "",
      name: warehouse?.name ?? "",
      address: warehouse?.address ?? "",
      phone: warehouse?.phone ?? "",
    }),
    [warehouse],
  );
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), values });
  // ...but `values` alone can't undo an ABANDONED edit: re-opening on the same
  // row is deep-equal, so RHF skips the sync and the stale draft reappears.
  useResetOnOpen(form, open, values);
  const onServerError = useServerFormErrors(form);

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      if (isEdit && warehouse) {
        await update.mutateAsync({ id: warehouse.id, name: v.name, address: v.address, phone: v.phone });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync({ code: v.code, name: v.name, address: v.address, phone: v.phone });
        toast.success(t("common.create") + " ✓");
      }
      onClose();
    } catch (err) {
      onServerError(err); // warehouse.code_taken → field error on `code`
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={isEdit ? t("warehouses.editTitle") : t("warehouses.addTitle")}
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
        {/* code is immutable on edit */}
        <FormField control={form.control} name="code" label={t("warehouses.code")} required disabled={isEdit} />
        <FormField control={form.control} name="name" label={t("warehouses.name")} required />
        <FormField control={form.control} name="address" label={t("warehouses.address")} />
        <FormField control={form.control} name="phone" label={t("warehouses.phone")} />
      </Stack>
    </EntityDrawer>
  );
}
