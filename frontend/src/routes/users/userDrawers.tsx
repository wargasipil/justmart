import { Button, Stack, Text } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import FormField from "../../components/FormField";
import { Role } from "../../gen/auth_iface/v1/policy_pb";
import { useServerFormErrors } from "../../lib/formErrors";
import { toast } from "../../lib/toaster";
import { useCreateUserMutation } from "../../queries/users";
import { useRoleOptions } from "./roleOptions";

const CreateSchema = z.object({
  email: z.string().email(),
  name: z.string(),
  password: z.string().min(8),
  role: z.coerce.number().int(),
});
type CreateValues = z.infer<typeof CreateSchema>;

export function CreateUserDrawer({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const create = useCreateUserMutation();
  const form = useForm<CreateValues>({
    resolver: zodResolver(CreateSchema),
    defaultValues: { email: "", name: "", password: "", role: Role.CASHIER },
  });
  const onServerError = useServerFormErrors(form);

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync({
        email: values.email,
        name: values.name,
        password: values.password,
        role: values.role as Role,
      });
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch (err) {
      onServerError(err); // user.email_taken → field error on `email`
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("users.createTitle")}
      footer={
        <Stack direction="row" justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("common.save")}
          </Button>
        </Stack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <FormField
            control={form.control}
            name="email"
            label={t("users.email")}
            type="email"
            required
            autoFocus
          />
          <FormField control={form.control} name="name" label={t("users.name")} />
          <FormField
            control={form.control}
            name="password"
            label={t("users.password")}
            type="password"
            helperText={t("users.passwordHelp")}
            required
          />
          <RoleSelect form={form} />
        </Stack>
      </form>
    </EntityDrawer>
  );
}

function RoleSelect({ form }: { form: ReturnType<typeof useForm<CreateValues>> }) {
  const { t } = useTranslation();
  const value = form.watch("role");
  const roleItems = useRoleOptions(); // new user → no out-of-mode role to force-include
  return (
    <Stack gap={1}>
      <Text fontSize="sm" fontWeight="medium" color="fg.muted">
        {t("users.role")}
      </Text>
      <EnumSelect
        value={String(value)}
        onChange={(v) => form.setValue("role", Number(v))}
        items={roleItems}
        itemToString={(o) => o.label}
        itemToValue={(o) => o.value}
      />
    </Stack>
  );
}
