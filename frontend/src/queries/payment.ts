import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { disbursementClient, paymentIntegrationClient } from "../lib/clients";
import type { ApplyProviderCredentialsRequest } from "../gen/payment_iface/v1/payment_pb";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";

export const paymentKeys = {
  all: ["payment"] as const,
  status: ["payment", "status"] as const,
  disbursements: (opts: { status: string; referenceType: string; referenceId: string; page: number; pageSize: number }) =>
    [...paymentKeys.all, "disbursements", opts] as const,
};

export function useIntegrationStatusQuery() {
  return useQuery({
    queryKey: paymentKeys.status,
    queryFn: async () => {
      const res = await paymentIntegrationClient.getIntegrationStatus({});
      return res;
    },
  });
}

export function useApplyXenditCredentialsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (creds: { apiKey: string; webhookToken: string }) =>
      paymentIntegrationClient.applyProviderCredentials({
        provider: "xendit",
        credentials: { api_key: creds.apiKey, webhook_token: creds.webhookToken },
      } as PartialMessage<ApplyProviderCredentialsRequest>),
    onSuccess: () => qc.invalidateQueries({ queryKey: paymentKeys.all }),
    meta: { silentError: true },
  });
}

export function useSetActiveProviderMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (provider: string) => paymentIntegrationClient.setActiveProvider({ provider }),
    onSuccess: () => qc.invalidateQueries({ queryKey: paymentKeys.all }),
  });
}

export type DisbursementsQueryOpts = {
  status?: string;
  referenceType?: string;
  referenceId?: string;
  page?: number;
  pageSize?: number;
};

export function useDisbursementsQuery(opts: DisbursementsQueryOpts = {}) {
  const { status = "", referenceType = "", referenceId = "", page = 0, pageSize = DEFAULT_PAGE_SIZE } = opts;
  const q = useQuery({
    queryKey: paymentKeys.disbursements({ status, referenceType, referenceId, page, pageSize }),
    queryFn: async () => {
      const res = await disbursementClient.listDisbursements({
        status,
        referenceType,
        referenceId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.disbursements, total: res.total };
    },
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useRetryDisbursementMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => disbursementClient.retryDisbursement({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: paymentKeys.all }),
  });
}
