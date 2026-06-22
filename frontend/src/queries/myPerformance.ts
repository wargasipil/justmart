import { useQuery } from "@tanstack/react-query";

import { saleClient } from "../lib/clients";
import { PerformanceGranularity } from "../gen/pos_iface/v1/sale_pb";

// Self-scoped "My performance" for CASHIER/APOTEKER. The backend always scopes
// to the authenticated caller — no user id is sent — and the response carries no
// profit/COGS. Mirrors useSalesSummaryQuery's shape (staleTime, filter-keyed).
export function useMyPerformanceQuery(opts: {
  fromUnix: bigint;
  toUnix: bigint;
  granularity?: PerformanceGranularity;
}) {
  const req = {
    fromUnix: opts.fromUnix,
    toUnix: opts.toUnix,
    granularity: opts.granularity ?? PerformanceGranularity.DAY,
  };
  return useQuery({
    queryKey: ["my-performance", req],
    queryFn: () => saleClient.getMyPerformance(req),
    staleTime: 30_000,
  });
}
