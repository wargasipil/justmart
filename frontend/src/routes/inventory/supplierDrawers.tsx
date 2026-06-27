import { Button, HStack, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import type { Supplier } from "../../gen/inventory_iface/v1/supplier_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useCreateSupplierMutation, useUpdateSupplierMutation } from "../../queries/suppliers";

const Schema = z.object({
  code: z.string().min(1),
  name: z.string().min(1),
  contactEmail: z.string().email().or(z.literal("")),
  phone: z.string(),
  address: z.string(),
  bankName: z.string(),
  bankAccountNumber: z.string(),
  bankAccountHolder: z.string(),
});
type FormValues = z.infer<typeof Schema>;

const EMPTY: FormValues = {
  code: "",
  name: "",
  contactEmail: "",
  phone: "",
  address: "",
  bankName: "",
  bankAccountNumber: "",
  bankAccountHolder: "",
};

// Shared field stack for both create + edit. `code` is editable for suppliers
// (the backend re-checks uniqueness on update), so it's a normal field here.
function SupplierForm({
  form,
  autoFocusCode,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  autoFocusCode?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Stack gap={4}>
      <FormField control={form.control} name="code" label={t("inventory.suppliers.code")} required autoFocus={autoFocusCode} />
      <FormField control={form.control} name="name" label={t("inventory.suppliers.name")} required />
      <FormField control={form.control} name="contactEmail" label={t("inventory.suppliers.email")} type="email" />
      <FormField control={form.control} name="phone" label={t("inventory.suppliers.phone")} />
      <FormField control={form.control} name="address" label={t("inventory.suppliers.address")} />
      <Text fontSize="sm" fontWeight="medium" color="fg.muted" pt={2}>
        {t("inventory.suppliers.bankInfo")}
      </Text>
      <FormField control={form.control} name="bankName" label={t("inventory.suppliers.bankName")} />
      <FormField control={form.control} name="bankAccountNumber" label={t("inventory.suppliers.bankAccountNumber")} />
      <FormField control={form.control} name="bankAccountHolder" label={t("inventory.suppliers.bankAccountHolder")} />
    </Stack>
  );
}

export function CreateSupplierDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateSupplierMutation();
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: EMPTY });
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync({ ...values });
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch (err) {
      onServerError(err); // supplier.code_taken / name_taken → field error
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.suppliers.addTitle")}
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
        <SupplierForm form={form} autoFocusCode />
      </form>
    </EntityDrawer>
  );
}

export function EditSupplierDrawer({ supplier, onClose }: { supplier: Supplier | null; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateSupplierMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    // Reactive pre-fill: re-seeds whenever the edited supplier changes.
    values: supplier
      ? {
          code: supplier.code,
          name: supplier.name,
          contactEmail: supplier.contactEmail,
          phone: supplier.phone,
          address: supplier.address,
          bankName: supplier.bankName,
          bankAccountNumber: supplier.bankAccountNumber,
          bankAccountHolder: supplier.bankAccountHolder,
        }
      : EMPTY,
  });
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (values) => {
    if (!supplier) return;
    try {
      await update.mutateAsync({ id: supplier.id, ...values });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch (err) {
      onServerError(err);
    }
  });

  return (
    <EntityDrawer
      open={!!supplier}
      onClose={onClose}
      title={supplier ? `${t("inventory.suppliers.editTitle")} · ${supplier.code}` : ""}
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
        <SupplierForm form={form} />
      </form>
    </EntityDrawer>
  );
}
