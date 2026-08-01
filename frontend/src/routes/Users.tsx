import { useState } from "react";
import { Box, Button, Spinner, Stack, Switch, Table, Text } from "@chakra-ui/react";
import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";

import ChangePasswordDialog from "../components/ChangePasswordDialog";
import EnumSelect from "../components/EnumSelect";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import TableScroll from "../components/TableScroll";
import UserAvatar from "../components/UserAvatar";
import { usePageState } from "../lib/pagination";
import { Role } from "../gen/auth_iface/v1/policy_pb";
import { User } from "../gen/user_iface/v1/users_pb";
import { useAuth } from "../lib/auth";
import { displayName } from "../lib/roles";
import { useSetUserActiveMutation, useUpdateUserRoleMutation, useUsersQuery } from "../queries/users";
import { CreateUserDrawer } from "./users/userDrawers";
import { useRoleOptions } from "./users/roleOptions";

export default function Users() {
  const { t } = useTranslation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const page = usePageState("users");
  const usersQ = useUsersQuery({ page: page.page, pageSize: page.pageSize });

  return (
    <Box>
      <PageHeader
        title={t("users.title")}
        description={t("users.description")}
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
        <UsersTable users={usersQ.rows} />
      )}
      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={usersQ.total}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />

      <CreateUserDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Box>
  );
}

function UsersTable({ users }: { users: User[] }) {
  const { t } = useTranslation();
  return (
    <TableScroll>
      <Table.Root size="sm" stickyHeader>
        <Table.Header bg="bg.muted">
          <Table.Row>
            <Table.ColumnHeader>{t("users.user")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("users.role")}</Table.ColumnHeader>
            <Table.ColumnHeader>{t("users.active")}</Table.ColumnHeader>
            <Table.ColumnHeader />
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {users.map((u) => (
            <UserRow key={u.id} user={u} />
          ))}
        </Table.Body>
      </Table.Root>
    </TableScroll>
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
      {/* Identity cell: picture + name over email. One column instead of two —
          the email is the fallback primary line when a user has no name set. */}
      <Table.Cell>
        <Stack direction="row" align="center" gap={3}>
          <UserAvatar
            userId={user.id}
            name={displayName(user)}
            version={Number(user.avatarUpdatedAt)}
          />
          <Stack gap={0} minW={0}>
            <Text fontWeight="medium" truncate>
              {user.name || user.email}
            </Text>
            {user.name && (
              <Text fontSize="xs" color="fg.muted" truncate>
                {user.email}
              </Text>
            )}
          </Stack>
        </Stack>
      </Table.Cell>
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
