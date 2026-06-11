import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  Spinner,
  Table,
  Text,
} from "@chakra-ui/react";
import { Pencil, Plus, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ConfirmDialog from "../components/ConfirmDialog";
import EntityDrawer from "../components/EntityDrawer";
import EnumSelect from "../components/EnumSelect";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import SearchableSelect from "../components/SearchableSelect";
import type { Prescription } from "../gen/prescription_iface/v1/prescription_pb";
import { formatDate } from "../lib/format";
import { usePageState } from "../lib/pagination";
import { toast } from "../lib/toaster";
import { searchCustomers } from "../queries/customers";
import { useCustomerRefs } from "../queries/refs";
import {
  usePrescriptionsQuery,
  useUpdatePrescriptionMutation,
  useVoidPrescriptionMutation,
} from "../queries/prescriptions";
import PrescriptionFormFields, {
  rxFormCanSubmit,
  rxFormFromPrescription,
  rxFormToPayload,
  type RxFormState,
} from "./prescriptions/PrescriptionFormFields";

const ALL_STATUSES = ["ACTIVE", "DISPENSED", "EXPIRED", "VOIDED"] as const;

function statusBadge(status: string) {
  return status === "ACTIVE"
    ? "green"
    : status === "DISPENSED"
      ? "blue"
      : status === "EXPIRED"
        ? "red"
        : "gray";
}

export default function Prescriptions() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [customerFilter, setCustomerFilter] = useState<string>("");
  const [editing, setEditing] = useState<Prescription | null>(null);
  const [voidConfirmId, setVoidConfirmId] = useState<string | null>(null);

  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${statusFilter}|${customerFilter}`,
  );
  const rxQ = usePrescriptionsQuery({
    status: statusFilter,
    customerId: customerFilter,
    limit: pageSize,
    offset: page * pageSize,
  });
  const voidMut = useVoidPrescriptionMutation();

  // Resolve names for the page's prescription patients + the active filter
  // (resolve-by-IDs; the customer filter searches server-side via loadOptions).
  const customerRefs = useCustomerRefs(
    useMemo(
      () => [customerFilter, ...rxQ.rows.map((rx) => rx.customerId)],
      [customerFilter, rxQ.rows],
    ),
  );

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("prescriptions.title") }]}
        title={t("prescriptions.title")}
        actions={
          <Button colorPalette="blue" onClick={() => navigate("/prescriptions/new")}>
            <Plus size={16} />
            {t("prescriptions.addTitle")}
          </Button>
        }
      />

      <Flex gap={3} mb={4} wrap="wrap">
        <Box minW="180px">
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("prescriptions.filterStatus")}
          </Text>
          <EnumSelect
            size="sm"
            value={statusFilter}
            onChange={setStatusFilter}
            placeholder={t("prescriptions.filterAll")}
            items={[
              { value: "", label: t("prescriptions.filterAll") },
              ...ALL_STATUSES.map((s) => ({
                value: s as string,
                label: t(`prescriptions.states.${s.toLowerCase()}`),
              })),
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
        </Box>
        <Box minW="240px">
          <Text fontSize="xs" color="fg.muted" mb={1}>
            {t("prescriptions.customer")}
          </Text>
          <SearchableSelect
            size="sm"
            value={customerFilter}
            onChange={setCustomerFilter}
            loadOptions={searchCustomers}
            itemToString={(c) => c.name}
            itemToValue={(c) => c.id}
            selectedLabel={customerRefs.get(customerFilter)?.name}
            placeholder={t("prescriptions.filterAll")}
          />
        </Box>
      </Flex>

      {rxQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("prescriptions.rxNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.customer")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.issuerName")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.issuedAt")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.expiresAt")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.filterStatus")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("prescriptions.items")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {rxQ.rows.map((rx) => {
              const dispensedAny = rx.items.some((it) => it.dispensedQty > 0);
              return (
                <Table.Row key={rx.id}>
                  <Table.Cell fontFamily="mono">{rx.rxNo}</Table.Cell>
                  <Table.Cell>{customerRefs.get(rx.customerId)?.name ?? "—"}</Table.Cell>
                  <Table.Cell>{rx.issuerName}</Table.Cell>
                  <Table.Cell>{rx.issuedAt}</Table.Cell>
                  <Table.Cell>{rx.expiresAt}</Table.Cell>
                  <Table.Cell>
                    <Badge colorPalette={statusBadge(rx.status)}>
                      {t(`prescriptions.states.${rx.status.toLowerCase()}`)}
                    </Badge>
                  </Table.Cell>
                  <Table.Cell>{rx.items.length}</Table.Cell>
                  <Table.Cell>
                    <HStack gap={1}>
                      {rx.status === "ACTIVE" && !dispensedAny && (
                        <Button size="xs" variant="ghost" onClick={() => setEditing(rx)}>
                          <Pencil size={14} />
                          {t("common.edit")}
                        </Button>
                      )}
                      {rx.status === "ACTIVE" && (
                        <Button size="xs" variant="ghost" onClick={() => setVoidConfirmId(rx.id)}>
                          <X size={14} />
                          {t("prescriptions.void")}
                        </Button>
                      )}
                    </HStack>
                  </Table.Cell>
                </Table.Row>
              );
            })}
            {rxQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={8}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("common.noResults")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Box mt={3}>
        <Pagination
          page={page}
          pageSize={pageSize}
          total={rxQ.total}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />
      </Box>

      <EditPrescriptionDrawer editing={editing} onClose={() => setEditing(null)} />
      <ConfirmDialog
        open={voidConfirmId !== null}
        title={t("prescriptions.void")}
        body={t("prescriptions.confirmVoid")}
        confirmLabel={t("prescriptions.void")}
        loading={voidMut.isPending}
        onConfirm={async () => {
          if (!voidConfirmId) return;
          try {
            await voidMut.mutateAsync(voidConfirmId);
            setVoidConfirmId(null);
          } catch {
            /* toast handled globally */
          }
        }}
        onCancel={() => setVoidConfirmId(null)}
      />
    </Box>
  );
}

// Edit-only drawer (create moved to the full /prescriptions/new page). Shares
// the same form body as the create page so biaya jasa + clinical fields are
// editable here too.
function EditPrescriptionDrawer({
  editing,
  onClose,
}: {
  editing: Prescription | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const updateMut = useUpdatePrescriptionMutation();
  const [form, setForm] = useState<RxFormState | null>(null);

  // Hydrate the form when the drawer opens / target changes.
  useEffect(() => {
    setForm(editing ? rxFormFromPrescription(editing) : null);
  }, [editing]);

  const onChange = (patch: Partial<RxFormState>) =>
    setForm((cur) => (cur ? { ...cur, ...patch } : cur));

  const submit = async () => {
    if (!editing || !form) return;
    try {
      await updateMut.mutateAsync({ id: editing.id, ...rxFormToPayload(form) });
      toast.success(t("common.save") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <EntityDrawer
      open={!!editing}
      onClose={onClose}
      title={t("prescriptions.editTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={submit}
            loading={updateMut.isPending}
            disabled={!form || !rxFormCanSubmit(form)}
          >
            {t("common.save")}
          </Button>
        </HStack>
      }
    >
      {form && (
        <>
          <PrescriptionFormFields value={form} onChange={onChange} editing={editing} />
          {editing && (
            <Text fontSize="xs" color="fg.muted" mt={4}>
              {t("prescriptions.rxNo")}: {editing.rxNo} · {formatDate(editing.issuedAt)}
            </Text>
          )}
        </>
      )}
    </EntityDrawer>
  );
}
