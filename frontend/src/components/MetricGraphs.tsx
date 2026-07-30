import { Grid } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import ChartCard from "./ChartCard";
import TrendChart from "./TrendChart";
import {
  MetricType,
  type MetricOrder,
  type MetricStock,
} from "../gen/analytics_iface/v1/analytics_pb";

// MetricGraphs — one chart per active metric field (terjual/hpp/profit when
// ORDER requested + ready/ongoing when STOCK requested). X = the dimension key
// (the day string for Daily), Y = the field's value pulled out of the
// corresponding map. Daily-only per the plan.
//
// Styling lives in the shared <ChartCard> + <TrendChart> (Chakra tokens, so the
// charts follow light/dark); this file only maps metric fields onto them. Each
// field pins its own CHART_SERIES slot so a metric keeps one colour across
// pages — revenue is the blue accent here and on the Dashboard trend.
type Props = {
  ids: string[];
  order?: MetricOrder;
  stock?: MetricStock;
  metricTypes: MetricType[];
  // visibleFields: per-field visibility (e.g. "order.terjual"). Charts for
  // un-listed fields are skipped. When omitted, every field in the requested
  // metric groups is shown.
  visibleFields?: Set<string>;
  isLoading?: boolean;
};

export default function MetricGraphs({
  ids,
  order,
  stock,
  metricTypes,
  visibleFields,
  isLoading,
}: Props) {
  const { t } = useTranslation();
  const want = (groupOk: boolean, fieldId: string) =>
    groupOk && (visibleFields ? visibleFields.has(fieldId) : true);
  const hasOrderGroup = metricTypes.includes(MetricType.ORDER);
  const hasStockGroup = metricTypes.includes(MetricType.STOCK);

  type ChartSpec = {
    field: string;
    colorIndex: number;
    money?: boolean;
    value: (id: string) => number;
  };
  const ALL_CHARTS: ChartSpec[] = [
    { field: "order.terjual", colorIndex: 0, money: true, value: (id) => Number(order?.data[id]?.terjual ?? 0n) },
    { field: "order.hpp",     colorIndex: 1, money: true, value: (id) => Number(order?.data[id]?.hpp ?? 0n) },
    { field: "order.profit",  colorIndex: 2, money: true, value: (id) => Number(order?.data[id]?.profit ?? 0n) },
    { field: "stock.ready",   colorIndex: 0, value: (id) => Number(stock?.data[id]?.ready ?? 0n) },
    { field: "stock.ongoing", colorIndex: 1, value: (id) => Number(stock?.data[id]?.ongoing ?? 0n) },
  ];
  const charts = ALL_CHARTS.filter((c) =>
    want(c.field.startsWith("order.") ? hasOrderGroup : hasStockGroup, c.field),
  );

  return (
    <Grid templateColumns={{ base: "1fr", md: "repeat(2, 1fr)" }} gap={4}>
      {charts.map((c) => {
        const label = t(`analytics.metric.${c.field}`);
        return (
          <ChartCard
            key={c.field}
            title={label}
            height="200px"
            isLoading={isLoading}
            isEmpty={ids.length === 0}
          >
            <TrendChart
              data={ids.map((id) => ({ key: id, value: c.value(id) }))}
              xKey="key"
              money={c.money}
              series={[{ dataKey: "value", label, colorIndex: c.colorIndex }]}
            />
          </ChartCard>
        );
      })}
    </Grid>
  );
}
