import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { payrollClient } from "../lib/clients";
import type {
  AddPayslipComponentRequest,
  CreatePayrollRunRequest,
  PrintPayslipRequest,
  SetPayrollSettingsRequest,
  UpdatePayslipComponentRequest,
} from "../gen/payroll_iface/v1/payroll_pb";

import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export type PayrollRunsQueryOpts = {
  status?: string;
  year?: number;
  page?: number;
  pageSize?: number;
};

export const payrollKeys = {
  all: ["payroll"] as const,
  runs: (opts: Required<PayrollRunsQueryOpts>) => [...payrollKeys.all, "runs", opts] as const,
  run: (id: string) => [...payrollKeys.all, "run", id] as const,
  settings: ["payroll", "settings"] as const,
};

export function usePayrollRunsQuery(opts: PayrollRunsQueryOpts = {}) {
  const { status = "", year = 0, page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: payrollKeys.runs({ status, year, page, pageSize }),
    queryFn: async () => {
      const res = await payrollClient.listPayrollRuns({
        status,
        year,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.runs, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function usePayrollRunQuery(id: string, enabled = true) {
  return useQuery({
    queryKey: payrollKeys.run(id),
    queryFn: async () => {
      const res = await payrollClient.getPayrollRun({ id });
      return res.run;
    },
    enabled: enabled && !!id,
  });
}

export function useCreatePayrollRunMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreatePayrollRunRequest>) => payrollClient.createPayrollRun(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

// Component edits + lifecycle transitions all return the full run; invalidate the
// run + the runs list so the detail page and list both refresh.
export function useUpdatePayslipComponentMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdatePayslipComponentRequest>) =>
      payrollClient.updatePayslipComponent(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

export function useAddPayslipComponentMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<AddPayslipComponentRequest>) =>
      payrollClient.addPayslipComponent(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

export function useRemovePayslipComponentMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (componentId: string) => payrollClient.removePayslipComponent({ componentId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

export function useApprovePayrollRunMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => payrollClient.approvePayrollRun({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

export function useMarkPayrollRunPaidMutation() {
  const qc = useQueryClient();
  return useMutation({
    // method: "" / "MANUAL" (default) | "GATEWAY" (disburse via the active provider)
    mutationFn: (arg: { id: string; method?: string }) =>
      payrollClient.markPayrollRunPaid({ id: arg.id, method: arg.method ?? "" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

export function useVoidPayrollRunMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => payrollClient.voidPayrollRun({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.all }),
  });
}

export function usePrintPayslipMutation() {
  return useMutation({
    mutationFn: (req: PartialMessage<PrintPayslipRequest>) => payrollClient.printPayslip(req),
  });
}

export function usePayrollSettingsQuery() {
  return useQuery({
    queryKey: payrollKeys.settings,
    queryFn: async () => {
      const res = await payrollClient.getPayrollSettings({});
      return { bpjsConfigJson: res.bpjsConfigJson, pph21ConfigJson: res.pph21ConfigJson };
    },
  });
}

export function useSetPayrollSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetPayrollSettingsRequest>) =>
      payrollClient.setPayrollSettings(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: payrollKeys.settings }),
  });
}
