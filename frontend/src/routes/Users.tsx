import { useState } from "react";
import {
  Box,
  Button,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import ChangePasswordDialog from "../components/ChangePasswordDialog";
import EntityDrawer from "../components/EntityDrawer";
import EnumSelect from "../components/EnumSelect";
import FormField from "../components/FormField";
import PageHeader from "../components/PageHeader";
import { Role } from "../gen/auth_iface/v1/policy_pb";
import { User } from "../gen/user_iface/v1/users_pb";
import { useAuth } from "../lib/auth";
import { useServerFormErrors } from "../lib/formErrors";
import { toast } from "../lib/toaster";
import {
  useCreateUserMutation,
  useSetUserActiveMutation,
  useUpdateUserRoleMutation,
  useUsersQuery,
} from "../queries/users";
import { useBusinessMode } from "../queries/settings";

const ROLE_OPTIONS: { value: Role; key: string }[] = [
  { value: Role.OWNER, key: "owner" },
  { value: Role.PHARMACIST, key: "pharmacist" }, // labeled "Admin" (manager tier, both modes)
  { value: Role.APOTEKER, key: "apoteker" }, // pharmacy-only
  { value: Role.CASHIER, key: "cashier" },
];

// useRoleOptions returns the assignable roles for the current business mode:
// APOTEKER (the Rx-authority role) is pharmacy-only — kept out of retail to stop
// roles mixing across modes (mirrors the backend rolesByMode). `include` forces a
// role to remain even when filtered, so an existing out-of-mode user's current
// role still renders in their row instead of showing blank.
function useRoleOptions(include?: Role): { value: string; label: string }[] {
  const { t } = useTranslation();
  const { isPharmacy } = useBusinessMode();
  return ROLE_OPTIONS.filter(
    (o) => o.value !== Role.APOTEKER || isPharmacy || o.value === include,
  ).map((o) => ({ value: String(o.value), label: t(`dashboard.roles.${o.key}`) }));
}

const CreateSchema = z.object({
  email: z.string().email(),
  name: z.string(),
  password: z.string().min(8),
  role: z.coerce.number().int(),
});
type CreateValues = z.infer<typeof CreateSchema>;

export default function Users() {
  const { t } = useTranslation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const usersQ = useUsersQuery();

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("users.title") }]}
        title={t("users.title")}
        actions={
          <Button colorPalette="blue" onClick={() => setDrawerOpen(true)}>
            <Plus size={16} />
            {t("common.add")}
          </Button>
        }
      />

      {usersQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <UsersTable users={usersQ.data ?? []} />
      )}

      <CreateUserDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Box>
  );
}

function UsersTable({ users }: { users: User[] }) {
  return (
    <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg" overflow="hidden">
      <Table.Header bg="bg.muted">
        <Table.Row>
          <Table.ColumnHeader>Email</Table.ColumnHeader>
          <Table.ColumnHeader>Name</Table.ColumnHeader>
          <Table.ColumnHeader>Role</Table.ColumnHeader>
          <Table.ColumnHeader>Active</Table.ColumnHeader>
          <Table.ColumnHeader />
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {users.map((u) => (
          <UserRow key={u.id} user={u} />
        ))}
      </Table.Body>
    </Table.Root>
  );
}

function UserRow({ user }: { user: User }) {
  const { t } = useTranslation();
  const { user: me } = useAuth();
  const setRole = useUpdateUserRoleMutation();
  const setActive = useSetUserActiveMutation();
  const [passwordOpen, setPasswordOpen] = useState(false);
  const canChangePw = me?.role === Role.OWNER && me?.id !== user.id;
  // Keep this user's current role visible even if it's out-of-mode (e.g. an
  // existing APOTEKER while in retail) so the select shows it, not a blank.
  const roleItems = useRoleOptions(user.role);

  return (
    <Table.Row>
      <Table.Cell>{user.email}</Table.Cell>
      <Table.Cell>{user.name}</Table.Cell>
      <Table.Cell>
        <EnumSelect
          size="sm"
          width="160px"
          value={String(user.role)}
          onChange={(v) => {
            const next = Number(v) as Role;
            if (next !== user.role) {
              setRole.mutate({ userId: user.id, role: next });
            }
          }}
          items={roleItems}
          itemToString={(o) => o.label}
          itemToValue={(o) => o.value}
        />
      </Table.Cell>
      <Table.Cell>
        <Switch.Root
          checked={user.active}
          onCheckedChange={(d) => setActive.mutate({ userId: user.id, active: d.checked })}
        >
          <Switch.HiddenInput />
          <Switch.Control />
        </Switch.Root>
      </Table.Cell>
      <Table.Cell>
        {canChangePw && (
          <>
            <Button size="xs" variant="ghost" onClick={() => setPasswordOpen(true)}>
              {t("users.changePassword")}
            </Button>
            <ChangePasswordDialog
              open={passwordOpen}
              onClose={() => setPasswordOpen(false)}
              userId={user.id}
              isSelf={false}
              userLabel={user.email}
            />
          </>
        )}
      </Table.Cell>
    </Table.Row>
  );
}

function CreateUserDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
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
