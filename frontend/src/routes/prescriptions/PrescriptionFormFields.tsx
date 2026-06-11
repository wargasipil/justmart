import {
  Box,
  Button,
  Flex,
  HStack,
  Heading,
  IconButton,
  Input,
  Stack,
  Text,
  Textarea,
} from "@chakra-ui/react";
import { Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import DatePickerField from "../../components/DatePicker";
import MoneyInput from "../../components/MoneyInput";
import SearchableSelect from "../../components/SearchableSelect";
import type { Prescription } from "../../gen/prescription_iface/v1/prescription_pb";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { searchCustomers, useCreateCustomerMutation, useCustomerQuery } from "../../queries/customers";
import { searchProducts } from "../../queries/products";
import { useCustomerRefs, useProductRefs } from "../../queries/refs";
import { searchUsers } from "../../queries/users";

// --- Shared form state -----------------------------------------------------

export type RxLine = {
  productId: string;
  prescribedQty: number;
  dosageInstructions: string;
  note: string;
};

export const emptyRxLine: RxLine = {
  productId: "",
  prescribedQty: 1,
  dosageInstructions: "",
  note: "",
};

export type RxFormState = {
  customerId: string;
  issuerName: string;
  issuedAt: string;
  expiresAt: string;
  note: string;
  biayaJasa: number; // minor units (whole rupiah)
  patientAge: number;
  patientWeight: string;
  patientAllergy: string;
  lines: RxLine[];
};

function isoToday() {
  return new Date().toISOString().slice(0, 10);
}

export function emptyRxForm(): RxFormState {
  return {
    customerId: "",
    issuerName: "",
    issuedAt: isoToday(),
    expiresAt: "",
    note: "",
    biayaJasa: 0,
    patientAge: 0,
    patientWeight: "",
    patientAllergy: "",
    lines: [{ ...emptyRxLine }],
  };
}

export function rxFormFromPrescription(rx: Prescription): RxFormState {
  return {
    customerId: rx.customerId,
    issuerName: rx.issuerName,
    issuedAt: rx.issuedAt,
    expiresAt: rx.expiresAt,
    note: rx.note,
    biayaJasa: Number(rx.biayaJasa),
    patientAge: rx.patientAge,
    patientWeight: rx.patientWeight,
    patientAllergy: rx.patientAllergy,
    lines: rx.items.map((it) => ({
      productId: it.productId,
      prescribedQty: it.prescribedQty,
      dosageInstructions: it.dosageInstructions,
      note: it.note,
    })),
  };
}

export function rxFormCanSubmit(f: RxFormState): boolean {
  return (
    !!f.customerId &&
    f.issuerName.trim().length > 0 &&
    f.issuedAt.length > 0 &&
    f.biayaJasa >= 0 &&
    f.lines.length > 0 &&
    f.lines.every((l) => l.productId && l.prescribedQty > 0)
  );
}

// rxFormToPayload builds the Create/UpdatePrescriptionRequest shape (minus id).
// biaya_jasa is int64 on the wire -> BigInt.
export function rxFormToPayload(f: RxFormState) {
  return {
    customerId: f.customerId,
    issuerName: f.issuerName.trim(),
    issuedAt: f.issuedAt,
    expiresAt: f.expiresAt,
    note: f.note.trim(),
    biayaJasa: BigInt(Math.max(0, Math.round(f.biayaJasa))),
    patientAge: Math.max(0, Math.round(f.patientAge)),
    patientWeight: f.patientWeight.trim(),
    patientAllergy: f.patientAllergy.trim(),
    items: f.lines.map((l) => ({
      productId: l.productId,
      prescribedQty: l.prescribedQty,
      dosageInstructions: l.dosageInstructions,
      note: l.note,
    })),
  };
}

// --- Patient panel ---------------------------------------------------------

// Read-only details for the picked patient (name/phone/address) + an inline
// "create new patient" affordance (page-only, gated by allowCreatePatient).
function PatientSection({
  customerId,
  onChange,
  allowCreatePatient,
}: {
  customerId: string;
  onChange: (id: string) => void;
  allowCreatePatient?: boolean;
}) {
  const { t } = useTranslation();
  const customerRefs = useCustomerRefs(useMemo(() => (customerId ? [customerId] : []), [customerId]));
  const detailQ = useCustomerQuery(customerId);
  const createMut = useCreateCustomerMutation();

  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newPhone, setNewPhone] = useState("");
  const [newAddress, setNewAddress] = useState("");
  // Name of the just-inline-created patient — seeds the select trigger label
  // immediately so it shows the name (not the raw UUID) before refs/detail load.
  const [manualLabel, setManualLabel] = useState("");

  const submitNewPatient = async () => {
    if (!newName.trim()) return;
    try {
      const res = await createMut.mutateAsync({
        name: newName.trim(),
        phone: newPhone.trim(),
        address: newAddress.trim(),
      });
      if (res.customer?.id) {
        setManualLabel(newName.trim());
        onChange(res.customer.id);
      }
      toast.success(t("common.create") + " ✓");
      setCreating(false);
      setNewName("");
      setNewPhone("");
      setNewAddress("");
    } catch {
      /* toast handled globally */
    }
  };

  const detail = detailQ.data;
  // Resolved label for the trigger: prefer the refs/detail name, fall back to
  // the name captured at inline-create time (covers the window before the
  // ResolveCustomers / GetCustomer responses land).
  const selectedLabel =
    customerRefs.get(customerId)?.name ?? detail?.name ?? (customerId ? manualLabel : "") ?? "";

  return (
    <Box>
      <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
        {t("prescriptions.customer")} *
      </Text>
      <SearchableSelect
        value={customerId}
        onChange={onChange}
        loadOptions={searchCustomers}
        itemToString={(c) => (c.phone ? `${c.name} · ${c.phone}` : c.name)}
        itemToValue={(c) => c.id}
        selectedLabel={selectedLabel || undefined}
        placeholder={t("prescriptions.selectCustomer")}
      />

      {allowCreatePatient && !creating && (
        <Button size="xs" variant="ghost" mt={1} onClick={() => setCreating(true)}>
          <Plus size={12} />
          {t("prescriptions.newPatient")}
        </Button>
      )}

      {allowCreatePatient && creating && (
        <Box mt={2} borderWidth="1px" borderRadius="md" p={3} bg="bg.subtle" borderColor="border.muted">
          <Stack gap={2}>
            <Box>
              <Text fontSize="xs" color="fg.muted" mb={1}>
                {t("customers.name")} *
              </Text>
              <Input size="sm" value={newName} onChange={(e) => setNewName(e.target.value)} />
            </Box>
            <Flex gap={2}>
              <Box flex="1">
                <Text fontSize="xs" color="fg.muted" mb={1}>
                  {t("customers.phone")}
                </Text>
                <Input size="sm" value={newPhone} onChange={(e) => setNewPhone(e.target.value)} />
              </Box>
              <Box flex="1">
                <Text fontSize="xs" color="fg.muted" mb={1}>
                  {t("customers.address")}
                </Text>
                <Input size="sm" value={newAddress} onChange={(e) => setNewAddress(e.target.value)} />
              </Box>
            </Flex>
            <HStack justify="flex-end" gap={2}>
              <Button size="xs" variant="ghost" onClick={() => setCreating(false)}>
                {t("common.cancel")}
              </Button>
              <Button
                size="xs"
                colorPalette="blue"
                onClick={submitNewPatient}
                loading={createMut.isPending}
                disabled={!newName.trim()}
              >
                {t("common.create")}
              </Button>
            </HStack>
          </Stack>
        </Box>
      )}

      {customerId && detail && (
        <Box mt={2} fontSize="xs" color="fg.muted">
          <HStack gap={4} wrap="wrap">
            <Text fontWeight="medium" color="fg">
              {detail.name}
            </Text>
            {detail.phone && (
              <Text>
                {t("customers.phone")}: {detail.phone}
              </Text>
            )}
            {detail.address && (
              <Text>
                {t("customers.address")}: {detail.address}
              </Text>
            )}
          </HStack>
        </Box>
      )}
    </Box>
  );
}

// --- Shared form body ------------------------------------------------------

export default function PrescriptionFormFields({
  value,
  onChange,
  editing,
  allowCreatePatient,
}: {
  value: RxFormState;
  onChange: (patch: Partial<RxFormState>) => void;
  editing?: Prescription | null;
  allowCreatePatient?: boolean;
}) {
  const { t } = useTranslation();
  const productRefs = useProductRefs(
    useMemo(() => value.lines.map((l) => l.productId).filter(Boolean), [value.lines]),
  );

  const addLine = () => onChange({ lines: [...value.lines, { ...emptyRxLine }] });
  const removeLine = (idx: number) =>
    onChange({ lines: value.lines.filter((_, i) => i !== idx) });
  const updateLine = (idx: number, patch: Partial<RxLine>) =>
    onChange({ lines: value.lines.map((l, i) => (i === idx ? { ...l, ...patch } : l)) });

  return (
    <Stack gap={4}>
      <PatientSection
        customerId={value.customerId}
        onChange={(id) => onChange({ customerId: id })}
        allowCreatePatient={allowCreatePatient}
      />

      <Box>
        <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
          {t("prescriptions.issuerName")} *
        </Text>
        <SearchableSelect
          value={value.issuerName}
          onChange={(v) => onChange({ issuerName: v })}
          loadOptions={searchUsers}
          itemToString={(u) => u.name || u.email}
          itemToValue={(u) => u.name || u.email}
          selectedLabel={value.issuerName || undefined}
          placeholder={t("prescriptions.selectIssuer")}
        />
      </Box>

      <Flex gap={3}>
        <Box flex="1">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.issuedAt")} *
          </Text>
          <DatePickerField value={value.issuedAt} onChange={(v) => onChange({ issuedAt: v })} />
        </Box>
        <Box flex="1">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.expiresAt")}
          </Text>
          <DatePickerField value={value.expiresAt} onChange={(v) => onChange({ expiresAt: v })} />
          <Text fontSize="xs" color="fg.muted" mt={1}>
            {t("prescriptions.expiresAtHelp")}
          </Text>
        </Box>
      </Flex>

      {/* Clinical / patient fields stored on the resep. */}
      <Flex gap={3} wrap="wrap">
        <Box w="120px">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.patientAge")}
          </Text>
          <Input
            type="number"
            min={0}
            value={value.patientAge || ""}
            placeholder="0"
            onChange={(e) => onChange({ patientAge: parseInt(e.target.value, 10) || 0 })}
          />
        </Box>
        <Box flex="1" minW="140px">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.patientWeight")}
          </Text>
          <Input
            value={value.patientWeight}
            onChange={(e) => onChange({ patientWeight: e.target.value })}
          />
        </Box>
        <Box flex="2" minW="200px">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.patientAllergy")}
          </Text>
          <Input
            value={value.patientAllergy}
            onChange={(e) => onChange({ patientAllergy: e.target.value })}
          />
        </Box>
      </Flex>

      <Flex gap={3} wrap="wrap" align="flex-start">
        <Box w="180px">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.biayaJasa")}
          </Text>
          <MoneyInput
            value={value.biayaJasa}
            onChange={(raw) => onChange({ biayaJasa: Number(raw || 0) })}
          />
        </Box>
        <Box flex="1" minW="220px">
          <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
            {t("prescriptions.note")}
          </Text>
          <Textarea value={value.note} onChange={(e) => onChange({ note: e.target.value })} rows={2} />
        </Box>
      </Flex>

      <Box>
        <HStack justify="space-between" mb={2}>
          <Heading size="sm">{t("prescriptions.items")}</Heading>
          <Button size="xs" variant="outline" onClick={addLine}>
            <Plus size={14} />
            {t("prescriptions.addLine")}
          </Button>
        </HStack>
        <Stack gap={3}>
          {value.lines.map((l, idx) => {
            const dispensed = editing?.items[idx]?.dispensedQty ?? 0;
            return (
              <Box
                key={idx}
                borderWidth="1px"
                borderRadius="md"
                p={3}
                bg="bg.subtle"
                borderColor="border.muted"
              >
                <Flex gap={2} wrap="wrap" align="flex-end">
                  <Box flex="2" minW="220px">
                    <Text fontSize="xs" color="fg.muted" mb={1}>
                      {t("prescriptions.selectProduct")}
                    </Text>
                    <SearchableSelect
                      size="sm"
                      value={l.productId}
                      onChange={(v) => updateLine(idx, { productId: v })}
                      loadOptions={searchProducts}
                      itemToString={(m) =>
                        `${m.name} · ${formatMoney(Number(m.unitPrice))}/${m.unit}${
                          m.prescriptionRequired ? " (Rx)" : ""
                        }`
                      }
                      itemToValue={(m) => m.id}
                      selectedLabel={productRefs.get(l.productId)?.name}
                      placeholder={t("prescriptions.selectProduct")}
                    />
                  </Box>
                  <Box w="120px">
                    <Text fontSize="xs" color="fg.muted" mb={1}>
                      {t("prescriptions.prescribedQty")}
                    </Text>
                    <Input
                      size="sm"
                      type="number"
                      value={l.prescribedQty}
                      onChange={(e) =>
                        updateLine(idx, { prescribedQty: parseInt(e.target.value, 10) || 0 })
                      }
                    />
                  </Box>
                  {editing && (
                    <Box w="90px">
                      <Text fontSize="xs" color="fg.muted" mb={1}>
                        {t("prescriptions.dispensedQty")}
                      </Text>
                      <Text fontSize="sm" fontFamily="mono" pt={1}>
                        {dispensed}
                      </Text>
                    </Box>
                  )}
                  <IconButton
                    aria-label="remove line"
                    size="xs"
                    variant="ghost"
                    onClick={() => removeLine(idx)}
                    disabled={value.lines.length === 1}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                </Flex>
                <Box mt={2}>
                  <Text fontSize="xs" color="fg.muted" mb={1}>
                    {t("prescriptions.dosage")}
                  </Text>
                  <Input
                    size="sm"
                    value={l.dosageInstructions}
                    onChange={(e) => updateLine(idx, { dosageInstructions: e.target.value })}
                  />
                </Box>
              </Box>
            );
          })}
        </Stack>
      </Box>
    </Stack>
  );
}
