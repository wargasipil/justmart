import { Box, Grid, HStack, Heading, Stack } from "@chakra-ui/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import DashboardTile from "../components/DashboardTile";
import DateRangeFilter, { resolveRange, type DateRange } from "../components/DateRangeFilter";
import EnumSelect from "../components/EnumSelect";
import PageHeader from "../components/PageHeader";
import { PerformanceGranularity } from "../gen/pos_iface/v1/sale_pb";
import { formatMoney } from "../lib/format";
import { useMyPerformanceQuery } from "../queries/myPerformance";

// CASHIER/APOTEKER self-scoped performance page. The backend always scopes to
// the authenticated caller (no user id sent) and never returns profit/COGS —
// this surface shows only the cashier's own revenue + quantity over time.
export default function MyPerformance() {
  const { t } = useTranslation();
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const [granularity, setGranularity] = useState<PerformanceGranularity>(
    PerformanceGranularity.DAY,
  );

  const q = useMyPerformanceQuery({
    fromUnix: BigInt(range.fromUnix),
    toUnix: BigInt(range.toUnix),
    granularity,
  });

  const revenue = Number(q.data?.totalRevenue ?? 0n);
  const sales = Number(q.data?.totalSalesCount ?? 0n);
  const items = Number(q.data?.totalItemsSold ?? 0n);

  const trendData = (q.data?.buckets ?? []).map((b) => ({
    day: b.dayKey,
    revenue: Number(b.revenue),
  }));

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("myPerformance.title") }]}
        title={t("myPerformance.title")}
        description={t("myPerformance.description")}
      />

      <HStack mb={4} gap={3} wrap="wrap">
        <EnumSelect
          size="sm"
          width="140px"
          value={String(granularity)}
          onChange={(v) => setGranularity(Number(v) as PerformanceGranularity)}
          items={[
            { value: String(PerformanceGranularity.DAY), label: t("analytics.granularity.day") },
            { value: String(PerformanceGranularity.WEEK), label: t("analytics.granularity.week") },
            { value: String(PerformanceGranularity.MONTH), label: t("analytics.granularity.month") },
          ]}
          itemToString={(o) => o.label}
          itemToValue={(o) => o.value}
        />
        <DateRangeFilter value={range} onChange={setRange} />
      </HStack>

      <Grid templateColumns={{ base: "1fr", md: "repeat(3, 1fr)" }} gap={4} mb={5}>
        <DashboardTile label={t("myPerformance.tiles.revenue")} value={formatMoney(revenue)} to="/orders" />
        <DashboardTile label={t("myPerformance.tiles.sales")} value={String(sales)} to="/orders" />
        <DashboardTile label={t("myPerformance.tiles.items")} value={String(items)} to="/orders" />
      </Grid>

      <Stack gap={3}>
        <Heading size="sm" color="fg.muted">
          {t("myPerformance.trend.revenue")}
        </Heading>
        <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4} h="280px">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={trendData}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--chakra-colors-border)" />
              <XAxis dataKey="day" fontSize={11} />
              <YAxis fontSize={11} tickFormatter={(v) => formatMoney(v).replace("Rp", "")} />
              <Tooltip formatter={(v: number) => formatMoney(v)} />
              <Line type="monotone" dataKey="revenue" stroke="#3B82F6" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </Box>
      </Stack>
    </Box>
  );
}
