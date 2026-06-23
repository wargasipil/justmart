import { Box, Button, Checkbox, Flex, Grid, HStack, IconButton, Input, Stack, Switch, Text } from "@chakra-ui/react";
import { Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import DatePickerField from "../../components/DatePicker";
import EnumSelect from "../../components/EnumSelect";
import MoneyInput from "../../components/MoneyInput";
import SearchableSelect from "../../components/SearchableSelect";
import type { Employee } from "../../gen/payroll_iface/v1/payroll_pb";
import { searchUsers } from "../../queries/users";
import { useUserRefs } from "../../queries/refs";

export const PTKP_OPTIONS = ["TK0", "TK1", "TK2", "TK3", "K0", "K1", "K2", "K3"] as const;

export type AllowanceState = { label: string; amount: number; taxable: boolean };

export type EmployeeFormState = {
  code: string;
  name: string;
  userId: string;
  position: string;
  baseSalary: number;
  commissionPct: number; // PERCENT in the UI (e.g. 2.5); converted to basis points on submit
  npwp: string;
  ptkpStatus: string;
  bpjsKesNo: string;
  bpjsTkNo: string;
  bpjsKesEnrolled: boolean;
  bpjsJhtEnrolled: boolean;
  bpjsJpEnrolled: boolean;
  bpjsJkkEnrolled: boolean;
  bpjsJkmEnrolled: boolean;
  bankName: string;
  bankAccountNumber: string;
  bankAccountHolder: string;
  bankChannelCode: string; // Xendit payout channel (e.g. ID_BCA); "" = no gateway payout
  joinedAt: string;
  allowances: AllowanceState[];
};

// Common Indonesian bank / e-wallet payout channel codes (Xendit). Tracks the
// channels Xendit publishes for ID payouts; extend as needed.
export const XENDIT_ID_CHANNELS = [
  "ID_BCA",
  "ID_MANDIRI",
  "ID_BNI",
  "ID_BRI",
  "ID_CIMB",
  "ID_PERMATA",
  "ID_BSI",
  "ID_DANAMON",
  "ID_OVO",
  "ID_DANA",
  "ID_LINKAJA",
  "ID_SHOPEEPAY",
  "ID_GOPAY",
] as const;

export function emptyEmployeeForm(): EmployeeFormState {
  return {
    code: "",
    name: "",
    userId: "",
    position: "",
    baseSalary: 0,
    commissionPct: 0,
    npwp: "",
    ptkpStatus: "TK0",
    bpjsKesNo: "",
    bpjsTkNo: "",
    bpjsKesEnrolled: false,
    bpjsJhtEnrolled: false,
    bpjsJpEnrolled: false,
    bpjsJkkEnrolled: false,
    bpjsJkmEnrolled: false,
    bankName: "",
    bankAccountNumber: "",
    bankAccountHolder: "",
    bankChannelCode: "",
    joinedAt: "",
    allowances: [],
  };
}

export function employeeFormFrom(e: Employee): EmployeeFormState {
  return {
    code: e.code,
    name: e.name,
    userId: e.userId,
    position: e.position,
    baseSalary: Number(e.baseSalary),
    commissionPct: e.commissionPct / 100,
    npwp: e.npwp,
    ptkpStatus: e.ptkpStatus || "TK0",
    bpjsKesNo: e.bpjsKesNo,
    bpjsTkNo: e.bpjsTkNo,
    bpjsKesEnrolled: e.bpjsKesEnrolled,
    bpjsJhtEnrolled: e.bpjsJhtEnrolled,
    bpjsJpEnrolled: e.bpjsJpEnrolled,
    bpjsJkkEnrolled: e.bpjsJkkEnrolled,
    bpjsJkmEnrolled: e.bpjsJkmEnrolled,
    bankName: e.bankName,
    bankAccountNumber: e.bankAccountNumber,
    bankAccountHolder: e.bankAccountHolder,
    bankChannelCode: e.bankChannelCode,
    joinedAt: e.joinedAt,
    allowances: e.allowances.map((a) => ({
      label: a.label,
      amount: Number(a.amount),
      taxable: a.taxable,
    })),
  };
}

export function employeeFormCanSubmit(f: EmployeeFormState): boolean {
  return f.code.trim().length > 0 && f.name.trim().length > 0;
}

// Builds the create/update request body (shared shape minus id).
export function employeeFormToPayload(f: EmployeeFormState) {
  return {
    code: f.code.trim(),
    name: f.name.trim(),
    userId: f.userId,
    position: f.position.trim(),
    baseSalary: BigInt(Math.max(0, Math.round(f.baseSalary))),
    commissionPct: Math.max(0, Math.round(f.commissionPct * 100)),
    npwp: f.npwp.trim(),
    ptkpStatus: f.ptkpStatus,
    bpjsKesNo: f.bpjsKesNo.trim(),
    bpjsTkNo: f.bpjsTkNo.trim(),
    bpjsKesEnrolled: f.bpjsKesEnrolled,
    bpjsJhtEnrolled: f.bpjsJhtEnrolled,
    bpjsJpEnrolled: f.bpjsJpEnrolled,
    bpjsJkkEnrolled: f.bpjsJkkEnrolled,
    bpjsJkmEnrolled: f.bpjsJkmEnrolled,
    bankName: f.bankName.trim(),
    bankAccountNumber: f.bankAccountNumber.trim(),
    bankAccountHolder: f.bankAccountHolder.trim(),
    bankChannelCode: f.bankChannelCode,
    joinedAt: f.joinedAt,
    allowances: f.allowances
      .filter((a) => a.label.trim().length > 0)
      .map((a) => ({
        label: a.label.trim(),
        amount: BigInt(Math.max(0, Math.round(a.amount))),
        taxable: a.taxable,
      })),
  };
}

function Label({ children }: { children: React.ReactNode }) {
  return (
    <Text fontSize="sm" fontWeight="medium" color="fg.muted" mb={1}>
      {children}
    </Text>
  );
}

export default function EmployeeFormFields({
  value,
  onChange,
}: {
  value: EmployeeFormState;
  onChange: (patch: Partial<EmployeeFormState>) => void;
}) {
  const { t } = useTranslation();
  const userRefs = useUserRefs(value.userId ? [value.userId] : []);
  const selectedUserLabel = userRefs.get(value.userId)?.name;

  const updateAllowance = (idx: number, patch: Partial<AllowanceState>) => {
    const next = value.allowances.map((a, i) => (i === idx ? { ...a, ...patch } : a));
    onChange({ allowances: next });
  };
  const addAllowance = () =>
    onChange({ allowances: [...value.allowances, { label: "", amount: 0, taxable: true }] });
  const removeAllowance = (idx: number) =>
    onChange({ allowances: value.allowances.filter((_, i) => i !== idx) });

  return (
    <Stack gap={5}>
      {/* Identity */}
      <Grid templateColumns="repeat(2, 1fr)" gap={3}>
        <Box>
          <Label>{t("payroll.employee.code")} *</Label>
          <Input size="sm" value={value.code} onChange={(e) => onChange({ code: e.target.value })} autoFocus />
        </Box>
        <Box>
          <Label>{t("payroll.employee.name")} *</Label>
          <Input size="sm" value={value.name} onChange={(e) => onChange({ name: e.target.value })} />
        </Box>
        <Box>
          <Label>{t("payroll.employee.position")}</Label>
          <Input size="sm" value={value.position} onChange={(e) => onChange({ position: e.target.value })} />
        </Box>
        <Box>
          <Label>{t("payroll.employee.joinedAt")}</Label>
          <DatePickerField value={value.joinedAt} onChange={(v) => onChange({ joinedAt: v })} />
        </Box>
        <Box gridColumn="span 2">
          <Label>{t("payroll.employee.userLink")}</Label>
          <SearchableSelect
            size="sm"
            value={value.userId}
            onChange={(id) => onChange({ userId: id })}
            loadOptions={searchUsers}
            itemToString={(u) => (u.name ? `${u.name} · ${u.email}` : u.email)}
            itemToValue={(u) => u.id}
            selectedLabel={selectedUserLabel}
            placeholder={t("payroll.employee.userLinkPlaceholder")}
          />
          <Text fontSize="xs" color="fg.muted" mt={1}>
            {t("payroll.employee.userLinkHelp")}
          </Text>
        </Box>
      </Grid>

      {/* Compensation */}
      <Box>
        <Text fontWeight="medium" mb={2}>
          {t("payroll.employee.compensation")}
        </Text>
        <Grid templateColumns="repeat(2, 1fr)" gap={3}>
          <Box>
            <Label>{t("payroll.employee.baseSalary")}</Label>
            <MoneyInput size="sm" value={value.baseSalary} onChange={(raw) => onChange({ baseSalary: Number(raw || 0) })} />
          </Box>
          <Box>
            <Label>{t("payroll.employee.commissionPct")}</Label>
            <Input
              size="sm"
              type="number"
              step="0.01"
              min={0}
              value={value.commissionPct || ""}
              onChange={(e) => onChange({ commissionPct: parseFloat(e.target.value) || 0 })}
            />
            <Text fontSize="xs" color="fg.muted" mt={1}>
              {t("payroll.employee.commissionHelp")}
            </Text>
          </Box>
        </Grid>
      </Box>

      {/* Allowances */}
      <Box>
        <HStack justify="space-between" mb={2}>
          <Text fontWeight="medium">{t("payroll.employee.allowances")}</Text>
          <Button size="xs" variant="outline" onClick={addAllowance}>
            <Plus size={14} />
            {t("payroll.employee.addAllowance")}
          </Button>
        </HStack>
        <Stack gap={2}>
          {value.allowances.length === 0 && (
            <Text fontSize="sm" color="fg.muted">
              {t("payroll.employee.noAllowances")}
            </Text>
          )}
          {value.allowances.map((a, idx) => (
            <Flex key={idx} gap={2} align="center" wrap="wrap">
              <Box flex="2" minW="160px">
                <Input
                  size="sm"
                  placeholder={t("payroll.employee.allowanceLabel")}
                  value={a.label}
                  onChange={(e) => updateAllowance(idx, { label: e.target.value })}
                />
              </Box>
              <Box w="140px">
                <MoneyInput size="sm" value={a.amount} onChange={(raw) => updateAllowance(idx, { amount: Number(raw || 0) })} />
              </Box>
              <Checkbox.Root
                size="sm"
                checked={a.taxable}
                onCheckedChange={(d) => updateAllowance(idx, { taxable: !!d.checked })}
              >
                <Checkbox.HiddenInput />
                <Checkbox.Control />
                <Checkbox.Label>{t("payroll.employee.taxable")}</Checkbox.Label>
              </Checkbox.Root>
              <IconButton size="xs" variant="ghost" aria-label="remove" onClick={() => removeAllowance(idx)}>
                <Trash2 size={14} />
              </IconButton>
            </Flex>
          ))}
        </Stack>
      </Box>

      {/* Tax & BPJS */}
      <Box>
        <Text fontWeight="medium" mb={2}>
          {t("payroll.employee.taxBpjs")}
        </Text>
        <Grid templateColumns="repeat(2, 1fr)" gap={3}>
          <Box>
            <Label>{t("payroll.employee.npwp")}</Label>
            <Input size="sm" value={value.npwp} onChange={(e) => onChange({ npwp: e.target.value })} />
          </Box>
          <Box>
            <Label>{t("payroll.employee.ptkpStatus")}</Label>
            <EnumSelect
              size="sm"
              value={value.ptkpStatus}
              onChange={(v) => onChange({ ptkpStatus: v })}
              items={PTKP_OPTIONS as readonly string[]}
              itemToString={(s) => s}
              itemToValue={(s) => s}
            />
          </Box>
          <Box>
            <Label>{t("payroll.employee.bpjsKesNo")}</Label>
            <Input size="sm" value={value.bpjsKesNo} onChange={(e) => onChange({ bpjsKesNo: e.target.value })} />
          </Box>
          <Box>
            <Label>{t("payroll.employee.bpjsTkNo")}</Label>
            <Input size="sm" value={value.bpjsTkNo} onChange={(e) => onChange({ bpjsTkNo: e.target.value })} />
          </Box>
        </Grid>
        <Stack gap={1} mt={3}>
          <Text fontSize="sm" color="fg.muted">
            {t("payroll.employee.bpjsEnrollment")}
          </Text>
          <Flex gap={4} wrap="wrap">
            <BpjsSwitch label={t("payroll.bpjs.kes")} checked={value.bpjsKesEnrolled} onChange={(v) => onChange({ bpjsKesEnrolled: v })} />
            <BpjsSwitch label={t("payroll.bpjs.jht")} checked={value.bpjsJhtEnrolled} onChange={(v) => onChange({ bpjsJhtEnrolled: v })} />
            <BpjsSwitch label={t("payroll.bpjs.jp")} checked={value.bpjsJpEnrolled} onChange={(v) => onChange({ bpjsJpEnrolled: v })} />
            <BpjsSwitch label={t("payroll.bpjs.jkk")} checked={value.bpjsJkkEnrolled} onChange={(v) => onChange({ bpjsJkkEnrolled: v })} />
            <BpjsSwitch label={t("payroll.bpjs.jkm")} checked={value.bpjsJkmEnrolled} onChange={(v) => onChange({ bpjsJkmEnrolled: v })} />
          </Flex>
        </Stack>
      </Box>

      {/* Bank */}
      <Box>
        <Text fontWeight="medium" mb={2}>
          {t("payroll.employee.bankInfo")}
        </Text>
        <Grid templateColumns="repeat(3, 1fr)" gap={3}>
          <Box>
            <Label>{t("payroll.employee.bankName")}</Label>
            <Input size="sm" value={value.bankName} onChange={(e) => onChange({ bankName: e.target.value })} />
          </Box>
          <Box>
            <Label>{t("payroll.employee.bankAccountNumber")}</Label>
            <Input size="sm" value={value.bankAccountNumber} onChange={(e) => onChange({ bankAccountNumber: e.target.value })} />
          </Box>
          <Box>
            <Label>{t("payroll.employee.bankAccountHolder")}</Label>
            <Input size="sm" value={value.bankAccountHolder} onChange={(e) => onChange({ bankAccountHolder: e.target.value })} />
          </Box>
          <Box>
            <Label>{t("payroll.employee.bankChannelCode")}</Label>
            <EnumSelect
              size="sm"
              value={value.bankChannelCode}
              onChange={(v) => onChange({ bankChannelCode: v })}
              items={["", ...XENDIT_ID_CHANNELS]}
              itemToString={(s) => (s === "" ? t("payroll.employee.channelNone") : s)}
              itemToValue={(s) => s}
            />
            <Text fontSize="xs" color="fg.muted" mt={1}>
              {t("payroll.employee.bankChannelHelp")}
            </Text>
          </Box>
        </Grid>
      </Box>
    </Stack>
  );
}

function BpjsSwitch({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <Switch.Root size="sm" checked={checked} onCheckedChange={(d) => onChange(d.checked)}>
      <Switch.HiddenInput />
      <Switch.Control />
      <Switch.Label>{label}</Switch.Label>
    </Switch.Root>
  );
}
