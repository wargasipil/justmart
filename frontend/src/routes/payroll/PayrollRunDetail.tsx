import { useState } from "react";
import {
  Badge,
  Box,
  Button,
  Grid,
  HStack,
  IconButton,
  Input,
  SimpleGrid,
  Spinner,
  Stack,
  Text,
} from "@chakra-ui/react";
import { Check, FileText, Plus, RefreshCw, Trash2, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import ConfirmDialog from "../../components/ConfirmDialog";
import MoneyInput from "../../components/MoneyInput";
import PageHeader from "../../components/PageHeader";
import type { Payslip, PayslipComponent } from "../../gen/payroll_iface/v1/payroll_pb";
import {
  useAddPayslipComponentMutation,
  useApprovePayrollRunMutation,
  useMarkPayrollRunPaidMutation,
  usePayrollRunQuery,
  useRemovePayslipComponentMutation,
  useUpdatePayslipComponentMutation,
  useVoidPayrollRunMutation,
} from "../../queries/payroll";
import { useIntegrationStatusQuery, useRetryDisbursementMutation } from "../../queries/payment";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import PayslipDialog from "./PayslipDialog";
import { RunStatusBadge, useMonthLabel } from "./shared";

type ConfirmKind = "approve" | "paid" | "paidGateway" | "void";

export default function PayrollRunDetail() {
  const { t } = useTranslation();
  const monthLabel = useMonthLabel();
  const { id = "" } = useParams();
  const q = usePayrollRunQuery(id);
  const run = q.data;

  const [confirm, setConfirm] = useState<ConfirmKind | null>(null);
  const [viewSlip, setViewSlip] = useState<Payslip | null>(null);

  const approve = useApprovePayrollRunMutation();
  const markPaid = useMarkPayrollRunPaidMutation();
  const voidRun = useVoidPayrollRunMutation();
  const integ = useIntegrationStatusQuery();
  const gatewayActive = !!integ.data?.activeProvider;

  if (q.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  if (!run) {
    return (
      <Box>
        <BackButton to="/payroll/runs" />
        <Text color="fg.muted" mt={4}>
          {t("common.noResults")}
        </Text>
      </Box>
    );
  }

  const isDraft = run.status === "DRAFT";
  const isApproved = run.status === "APPROVED";

  const runConfirm = async () => {
    try {
      if (confirm === "approve") await approve.mutateAsync(run.id);
      else if (confirm === "paid") await markPaid.mutateAsync({ id: run.id });
      else if (confirm === "paidGateway") await markPaid.mutateAsync({ id: run.id, method: "GATEWAY" });
      else if (confirm === "void") await voidRun.mutateAsync(run.id);
      setConfirm(null);
    } catch {
      setConfirm(null);
    }
  };

  const confirmCopy: Record<ConfirmKind, { title: string; body: string; label: string; palette: string }> = {
    approve: {
      title: t("payroll.run.approveTitle"),
      body: t("payroll.run.approveBody"),
      label: t("payroll.run.approve"),
      palette: "blue",
    },
    paid: {
      title: t("payroll.run.markPaidTitle"),
      body: t("payroll.run.markPaidBody"),
      label: t("payroll.run.markPaid"),
      palette: "green",
    },
    paidGateway: {
      title: t("payroll.run.payGatewayTitle"),
      body: t("payroll.run.payGatewayBody"),
      label: t("payroll.run.payGateway"),
      palette: "blue",
    },
    void: {
      title: t("payroll.run.voidTitle"),
      body: t("payroll.run.voidBody"),
      label: t("payroll.run.void"),
      palette: "red",
    },
  };

  return (
    <Box>
      <BackButton to="/payroll/runs" />
      <PageHeader
        breadcrumbs={[
          { label: t("nav.payroll"), to: "/payroll/runs" },
          { label: run.runNo },
        ]}
        title={run.runNo}
        actions={
          <HStack gap={2}>
            {isDraft && (
              <Button size="sm" colorPalette="blue" onClick={() => setConfirm("approve")}>
                {t("payroll.run.approve")}
              </Button>
            )}
            {isApproved && (
              <Button size="sm" colorPalette="green" onClick={() => setConfirm("paid")}>
                {t("payroll.run.markPaid")}
              </Button>
            )}
            {isApproved && gatewayActive && (
              <Button size="sm" colorPalette="blue" onClick={() => setConfirm("paidGateway")}>
                {t("payroll.run.payGateway")}
              </Button>
            )}
            {(isDraft || isApproved) && (
              <Button size="sm" variant="outline" colorPalette="red" onClick={() => setConfirm("void")}>
                {t("payroll.run.void")}
              </Button>
            )}
          </HStack>
        }
      />

      {/* Summary */}
      <SimpleGrid columns={{ base: 2, md: 4 }} gap={3} mb={5}>
        <SummaryTile label={t("payroll.run.period")} value={`${monthLabel(run.periodMonth)} ${run.periodYear}`} />
        <SummaryTile label={t("common.status")} value={<RunStatusBadge status={run.status} />} />
        <SummaryTile label={t("payroll.run.gross")} value={formatMoney(Number(run.totalGross))} />
        <SummaryTile label={t("payroll.run.net")} value={formatMoney(Number(run.totalNet))} />
      </SimpleGrid>

      {run.note && (
        <Text fontSize="sm" color="fg.muted" mb={4}>
          {run.note}
        </Text>
      )}

      <Stack gap={3}>
        {run.payslips.length === 0 && (
          <Text color="fg.muted">{t("payroll.run.noPayslips")}</Text>
        )}
        {run.payslips.map((p) => (
          <PayslipCard key={p.id} payslip={p} editable={isDraft} onView={() => setViewSlip(p)} />
        ))}
      </Stack>

      <PayslipDialog open={viewSlip !== null} onClose={() => setViewSlip(null)} payslip={viewSlip} run={run} />

      <ConfirmDialog
        open={confirm !== null}
        title={confirm ? confirmCopy[confirm].title : ""}
        body={confirm ? confirmCopy[confirm].body : ""}
        confirmLabel={confirm ? confirmCopy[confirm].label : ""}
        confirmColorPalette={confirm ? confirmCopy[confirm].palette : "red"}
        loading={approve.isPending || markPaid.isPending || voidRun.isPending}
        onConfirm={runConfirm}
        onCancel={() => setConfirm(null)}
      />
    </Box>
  );
}

function SummaryTile({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={3}>
      <Text fontSize="xs" color="fg.muted">
        {label}
      </Text>
      <Box fontWeight="semibold" mt={1}>
        {value}
      </Box>
    </Box>
  );
}

const DISB_PALETTE: Record<string, string> = {
  PENDING: "gray",
  PROCESSING: "blue",
  COMPLETED: "green",
  FAILED: "red",
  VOIDED: "gray",
};

function DisbursementStatus({ payslip }: { payslip: Payslip }) {
  const { t } = useTranslation();
  const retry = useRetryDisbursementMutation();
  const status = payslip.disbursementStatus;
  if (!status) return null;
  return (
    <HStack gap={1}>
      <Badge colorPalette={DISB_PALETTE[status] ?? "gray"}>
        {t(`payroll.disbursement.${status.toLowerCase()}`)}
      </Badge>
      {status === "FAILED" && payslip.disbursementId && (
        <IconButton
          size="xs"
          variant="ghost"
          aria-label={t("payroll.disbursement.retry")}
          loading={retry.isPending}
          onClick={() => retry.mutate(payslip.disbursementId)}
        >
          <RefreshCw size={13} />
        </IconButton>
      )}
    </HStack>
  );
}

function PayslipCard({ payslip, editable, onView }: { payslip: Payslip; editable: boolean; onView: () => void }) {
  const { t } = useTranslation();
  const earnings = payslip.components.filter((c) => c.kind === "EARNING");
  const deductions = payslip.components.filter((c) => c.kind === "DEDUCTION");
  const employer = payslip.components.filter((c) => c.kind === "EMPLOYER_CONTRIB");

  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4}>
      <HStack justify="space-between" mb={3}>
        <Box>
          <Text fontWeight="semibold">{payslip.employeeName}</Text>
          <Text fontSize="xs" color="fg.muted">
            {payslip.employeeCode}
            {payslip.position ? ` · ${payslip.position}` : ""}
          </Text>
        </Box>
        <HStack gap={3}>
          <DisbursementStatus payslip={payslip} />
          <Box textAlign="end">
            <Text fontSize="xs" color="fg.muted">
              {t("payroll.payslip.net")}
            </Text>
            <Text fontWeight="semibold">{formatMoney(Number(payslip.net))}</Text>
          </Box>
          <Button size="xs" variant="outline" onClick={onView}>
            <FileText size={14} />
            {t("payroll.payslip.view")}
          </Button>
        </HStack>
      </HStack>

      <Grid templateColumns={{ base: "1fr", md: "1fr 1fr" }} gap={4}>
        <ComponentGroup
          title={t("payroll.payslip.earnings")}
          kind="EARNING"
          payslipId={payslip.id}
          components={earnings}
          total={Number(payslip.gross)}
          totalLabel={t("payroll.payslip.totalEarnings")}
          editable={editable}
        />
        <ComponentGroup
          title={t("payroll.payslip.deductions")}
          kind="DEDUCTION"
          payslipId={payslip.id}
          components={deductions}
          total={Number(payslip.totalDeductions)}
          totalLabel={t("payroll.payslip.totalDeductions")}
          editable={editable}
        />
      </Grid>

      {employer.length > 0 && (
        <Box mt={3} pt={2} borderTopWidth="1px">
          <Text fontSize="xs" color="fg.muted">
            {t("payroll.payslip.employerCost", { amount: formatMoney(Number(payslip.employerCost)) })}
          </Text>
        </Box>
      )}
    </Box>
  );
}

function ComponentGroup({
  title,
  kind,
  payslipId,
  components,
  total,
  totalLabel,
  editable,
}: {
  title: string;
  kind: "EARNING" | "DEDUCTION";
  payslipId: string;
  components: PayslipComponent[];
  total: number;
  totalLabel: string;
  editable: boolean;
}) {
  return (
    <Box>
      <Text fontSize="sm" fontWeight="medium" mb={1}>
        {title}
      </Text>
      <Stack gap={1}>
        {components.map((c) => (
          <ComponentRow key={c.id} component={c} editable={editable} />
        ))}
        {editable && <AddLineRow payslipId={payslipId} kind={kind} />}
        <HStack justify="space-between" pt={1} mt={1} borderTopWidth="1px">
          <Text fontSize="sm" fontWeight="semibold">
            {totalLabel}
          </Text>
          <Text fontSize="sm" fontWeight="semibold">
            {formatMoney(total)}
          </Text>
        </HStack>
      </Stack>
    </Box>
  );
}

function ComponentRow({ component, editable }: { component: PayslipComponent; editable: boolean }) {
  const update = useUpdatePayslipComponentMutation();
  const remove = useRemovePayslipComponentMutation();
  const [amount, setAmount] = useState<string>(String(component.amount));

  const commit = () => {
    const next = BigInt(amount || 0);
    if (next === component.amount) return;
    update.mutate({
      componentId: component.id,
      label: component.label,
      amount: next,
      taxable: component.taxable,
    });
  };

  return (
    <HStack justify="space-between" gap={2}>
      <Text fontSize="sm" flex="1" truncate title={component.label}>
        {component.label}
        {component.overridden && " *"}
      </Text>
      {editable ? (
        <HStack gap={1}>
          <MoneyInput size="xs" width="120px" value={amount} onChange={setAmount} onBlur={commit} />
          {!component.systemGenerated && (
            <IconButton size="xs" variant="ghost" aria-label="remove" onClick={() => remove.mutate(component.id)}>
              <Trash2 size={13} />
            </IconButton>
          )}
        </HStack>
      ) : (
        <Text fontSize="sm">{formatMoney(Number(component.amount))}</Text>
      )}
    </HStack>
  );
}

function AddLineRow({ payslipId, kind }: { payslipId: string; kind: "EARNING" | "DEDUCTION" }) {
  const { t } = useTranslation();
  const add = useAddPayslipComponentMutation();
  const [open, setOpen] = useState(false);
  const [label, setLabel] = useState("");
  const [amount, setAmount] = useState("");

  const reset = () => {
    setLabel("");
    setAmount("");
    setOpen(false);
  };

  const submit = () => {
    if (!label.trim()) return;
    add.mutate(
      {
        payslipId,
        kind,
        code: kind === "EARNING" ? "BONUS" : "MANUAL",
        label: label.trim(),
        amount: BigInt(amount || 0),
        taxable: kind === "EARNING",
      },
      { onSuccess: reset, onError: () => toast.error(t("errors.generic")) },
    );
  };

  if (!open) {
    return (
      <Button size="xs" variant="ghost" alignSelf="flex-start" onClick={() => setOpen(true)}>
        <Plus size={13} />
        {t("payroll.payslip.addLine")}
      </Button>
    );
  }
  return (
    <HStack gap={1}>
      <Input size="xs" placeholder={t("payroll.payslip.lineLabel")} value={label} onChange={(e) => setLabel(e.target.value)} flex="1" />
      <MoneyInput size="xs" width="100px" value={amount} onChange={setAmount} />
      <IconButton size="xs" variant="ghost" aria-label="save" onClick={submit} loading={add.isPending}>
        <Check size={13} />
      </IconButton>
      <IconButton size="xs" variant="ghost" aria-label="cancel" onClick={reset}>
        <X size={13} />
      </IconButton>
    </HStack>
  );
}
