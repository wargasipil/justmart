import { useEffect, useState } from "react";
import { Box, Button, HStack, Input, Spinner, Stack, Switch, Table, Text } from "@chakra-ui/react";
import { Archive, Pencil, Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import EntityDrawer from "../../components/EntityDrawer";
import Pagination from "../../components/Pagination";
import type { Employee } from "../../gen/payroll_iface/v1/payroll_pb";
import {
  useArchiveEmployeeMutation,
  useCreateEmployeeMutation,
  useEmployeesQuery,
  useUpdateEmployeeMutation,
} from "../../queries/employees";
import { usePageState } from "../../lib/pagination";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import EmployeeFormFields, {
  emptyEmployeeForm,
  employeeFormCanSubmit,
  employeeFormFrom,
  employeeFormToPayload,
  type EmployeeFormState,
} from "./EmployeeFormFields";

export default function Employees() {
  const { t } = useTranslation();
  const [includeInactive, setIncludeInactive] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Employee | null>(null);
  const [archiveId, setArchiveId] = useState<string | null>(null);

  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(`${query}|${includeInactive}`);
  const q = useEmployeesQuery({ includeInactive, query, page, pageSize });
  const archive = useArchiveEmployeeMutation();

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={3}>
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="280px"
              placeholder={t("payroll.employee.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <Switch.Root checked={includeInactive} onCheckedChange={(d) => setIncludeInactive(d.checked)}>
            <Switch.HiddenInput />
            <Switch.Control />
            <Switch.Label>{t("common.showArchived")}</Switch.Label>
          </Switch.Root>
        </HStack>
        <Button size="sm" colorPalette="blue" onClick={() => setCreateOpen(true)}>
          <Plus size={16} />
          {t("payroll.employee.addTitle")}
        </Button>
      </HStack>

      {q.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("payroll.employee.code")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("payroll.employee.name")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("payroll.employee.position")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("payroll.employee.baseSalary")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("payroll.employee.commissionPct")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {q.rows.map((e) => (
              <Table.Row key={e.id}>
                <Table.Cell fontFamily="mono">{e.code}</Table.Cell>
                <Table.Cell>{e.name}</Table.Cell>
                <Table.Cell>{e.position || "—"}</Table.Cell>
                <Table.Cell textAlign="end">{formatMoney(Number(e.baseSalary))}</Table.Cell>
                <Table.Cell textAlign="end">{e.commissionPct ? `${e.commissionPct / 100}%` : "—"}</Table.Cell>
                <Table.Cell>{e.active ? t("common.yes") : t("common.no")}</Table.Cell>
                <Table.Cell>
                  <HStack gap={1}>
                    <Button size="xs" variant="ghost" onClick={() => setEditing(e)}>
                      <Pencil size={14} />
                      {t("common.edit")}
                    </Button>
                    {e.active && (
                      <Button size="xs" variant="ghost" onClick={() => setArchiveId(e.id)}>
                        <Archive size={14} />
                        {t("common.archive")}
                      </Button>
                    )}
                  </HStack>
                </Table.Cell>
              </Table.Row>
            ))}
            {q.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={7}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Pagination page={page} pageSize={pageSize} total={q.total} onPageChange={setPage} onPageSizeChange={setPageSize} />

      <CreateDrawer open={createOpen} onClose={() => setCreateOpen(false)} />
      <EditDrawer employee={editing} onClose={() => setEditing(null)} />

      <ConfirmDialog
        open={archiveId !== null}
        title={t("payroll.employee.archiveTitle")}
        body={t("payroll.employee.archiveBody")}
        confirmLabel={t("common.archive")}
        loading={archive.isPending}
        onConfirm={async () => {
          if (!archiveId) return;
          try {
            await archive.mutateAsync({ id: archiveId });
            setArchiveId(null);
          } catch {
            /* toast handled globally */
          }
        }}
        onCancel={() => setArchiveId(null)}
      />
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateEmployeeMutation();
  const [form, setForm] = useState<EmployeeFormState>(emptyEmployeeForm);

  const submit = async () => {
    try {
      await create.mutateAsync(employeeFormToPayload(form));
      toast.success(t("common.create") + " ✓");
      setForm(emptyEmployeeForm());
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("payroll.employee.addTitle")}
      size="lg"
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending} disabled={!employeeFormCanSubmit(form)}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <EmployeeFormFields value={form} onChange={(patch) => setForm((f) => ({ ...f, ...patch }))} />
    </EntityDrawer>
  );
}

function EditDrawer({ employee, onClose }: { employee: Employee | null; onClose: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateEmployeeMutation();
  const [form, setForm] = useState<EmployeeFormState>(emptyEmployeeForm);

  // Reset the form whenever a new employee is opened.
  useEffect(() => {
    if (employee) setForm(employeeFormFrom(employee));
  }, [employee]);

  const submit = async () => {
    if (!employee) return;
    try {
      await update.mutateAsync({ id: employee.id, ...employeeFormToPayload(form) });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <EntityDrawer
      open={employee !== null}
      onClose={onClose}
      title={t("payroll.employee.editTitle")}
      size="lg"
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={update.isPending} disabled={!employeeFormCanSubmit(form)}>
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      <EmployeeFormFields value={form} onChange={(patch) => setForm((f) => ({ ...f, ...patch }))} />
    </EntityDrawer>
  );
}
