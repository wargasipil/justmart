import { Button, HStack, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import FormField from "../../components/FormField";
import type { Manufacturer } from "../../gen/inventory_iface/v1/manufacturer_pb";
import { useResetOnOpen } from "../../lib/formReset";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import {
  useCreateManufacturerMutation,
  useUpdateManufacturerMutation,
} from "../../queries/manufacturers";

// Plain rules, no inline messages: the global Zod error map translates every
// issue into a validation.* key (see CLAUDE.md "Validation & error messages").
const Schema = z.object({
  code: z.string().min(1),
  name: z.string().min(1),
  phone: z.string(),
  contactEmail: z.string().email().or(z.literal("")),
  address: z.string(),
  note: z.string(),
});
type FormValues = z.infer<typeof Schema>;

const EMPTY: FormValues = {
  code: "",
  name: "",
  phone: "",
  contactEmail: "",
  address: "",
  note: "",
};

// Shared field stack for both create + edit. `code` is editable (the backend
// re-checks uniqueness on update), so it's a normal field here — same posture
// as the supplier form.
function ManufacturerForm({
  form,
  autoFocusCode,
}: {
  form: ReturnType<typeof useForm<FormValues>>;
  autoFocusCode?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Stack gap={4}>
      <FormField
        control={form.control}
        name="code"
        label={t("inventory.manufacturers.code")}
        required
        autoFocus={autoFocusCode}
      />
      <FormField control={form.control} name="name" label={t("inventory.manufacturers.name")} required />
      <FormField control={form.control} name="phone" label={t("inventory.manufacturers.phone")} />
      <FormField
        control={form.control}
        name="contactEmail"
        label={t("inventory.manufacturers.email")}
        type="email"
      />
      <FormField control={form.control} name="address" label={t("inventory.manufacturers.address")} />
      <FormField
        control={form.control}
        name="note"
        label={t("inventory.manufacturers.note")}
        placeholder={t("inventory.manufacturers.notePlaceholder")}
      />
    </Stack>
  );
}

export function CreateManufacturerDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateManufacturerMutation();
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: EMPTY });
  useResetOnOpen(form, open, EMPTY);
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync({ ...values });
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch (err) {
      onServerError(err); // manufacturer.code_taken / name_taken → field error
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.manufacturers.addTitle")}
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
        <ManufacturerForm form={form} autoFocusCode />
      </form>
    </EntityDrawer>
  );
}

export function EditManufacturerDrawer({
  manufacturer,
  onClose,
}: {
  manufacturer: Manufacturer | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const update = useUpdateManufacturerMutation();
  // Reactive pre-fill: re-seeds whenever the edited row changes. It can't undo
  // an ABANDONED edit though (re-opening the same row is deep-equal, so RHF
  // skips the sync) — that's what useResetOnOpen covers.
  const values: FormValues = manufacturer
    ? {
        code: manufacturer.code,
        name: manufacturer.name,
        phone: manufacturer.phone,
        contactEmail: manufacturer.contactEmail,
        address: manufacturer.address,
        note: manufacturer.note,
      }
    : EMPTY;
  const form = useForm<FormValues>({ resolver: zodResolver(Schema), defaultValues: EMPTY, values });
  useResetOnOpen(form, manufacturer != null, values);
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (v) => {
    if (!manufacturer) return;
    try {
      await update.mutateAsync({ id: manufacturer.id, ...v });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch (err) {
      onServerError(err);
    }
  });

  return (
    <EntityDrawer
      open={manufacturer != null}
      onClose={onClose}
      title={manufacturer ? `${t("inventory.manufacturers.editTitle")} · ${manufacturer.code}` : ""}
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
        <ManufacturerForm form={form} />
      </form>
    </EntityDrawer>
  );
}
