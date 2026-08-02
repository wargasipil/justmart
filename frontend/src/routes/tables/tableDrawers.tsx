import { Button, HStack, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMemo } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import type { DiningTable } from "../../gen/table_iface/v1/table_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useCreateTableMutation, useUpdateTableMutation } from "../../queries/tables";

// Plain rules only — the global Zod error map translates them (validation HARD
// RULE); never inline an English message. maxTableCodeLen (16) mirrors the
// backend bound.
const Schema = z.object({
  code: z.string().min(1).max(16),
  name: z.string(),
  area: z.string(),
  seats: z.coerce.number().int().min(0),
});
type FormValues = z.infer<typeof Schema>;

// Shared create/edit drawer for dining tables. Unlike warehouses, the CODE stays
// editable on edit: a floor gets rearranged and "T3" becomes "T4", and the
// backend already enforces per-outlet uniqueness among live tables.
export default function TableDrawer({
  open,
  table,
  onClose,
}: {
  open: boolean;
  table?: DiningTable | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!table;
  const create = useCreateTableMutation();
  const update = useUpdateTableMutation();

  // RHF `values` re-seeds whenever the table prop changes (a different row, or
  // create → edit) — no manual seed bookkeeping.
  const values = useMemo<FormValues>(
    () => ({
      code: table?.code ?? "",
      name: table?.name ?? "",
      area: table?.area ?? "",
      seats: table?.seats ?? 0,
    }),
    [table],
  );
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), values });
  const onServerError = useServerFormErrors(form);

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      if (isEdit && table) {
        await update.mutateAsync({ id: table.id, ...v });
        toast.success(t("common.save") + " ✓");
      } else {
        await create.mutateAsync(v);
        toast.success(t("common.create") + " ✓");
      }
      onClose();
    } catch (err) {
      onServerError(err); // table.code_taken → field error on `code`
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={isEdit ? t("tables.editTitle") : t("tables.addTitle")}
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
        <FormField
          control={form.control}
          name="code"
          label={t("tables.code")}
          helperText={t("tables.codeHelp")}
          required
        />
        <FormField control={form.control} name="name" label={t("tables.name")} />
        <FormField
          control={form.control}
          name="area"
          label={t("tables.area")}
          helperText={t("tables.areaHelp")}
        />
        <FormField control={form.control} name="seats" label={t("tables.seats")} type="number" />
      </Stack>
    </EntityDrawer>
  );
}
