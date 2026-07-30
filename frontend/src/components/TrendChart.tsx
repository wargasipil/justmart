import { Box, HStack, Stack, Text } from "@chakra-ui/react";
import { useId } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { chartTokens, formatAxisTick, formatChartValue, seriesColor } from "../lib/chartTheme";

// The one line/area chart in the app — Chakra-tokened axes, grid, crosshair and
// tooltip (see lib/chartTheme.ts for why the colours are CSS variables). Render
// it inside <ChartCard>, which owns the frame, title, and loading/empty states.
//
// A single series is drawn as a gradient-filled area (magnitude over time); two
// or more are plain overlapping lines plus a legend, since stacked translucent
// fills muddy each other. There is deliberately no second Y axis: two measures
// of different scale belong in two charts.
export type TrendSeries = {
  /** Key to read out of each `data` row. */
  dataKey: string;
  /** Localized series name — tooltip row + legend label. */
  label: string;
  /**
   * Slot in CHART_SERIES. Defaults to the series' position. Pin it when a
   * metric should keep one colour across pages (revenue is always slot 0).
   */
  colorIndex?: number;
};

type Row = Record<string, string | number>;

type Props = {
  data: Row[];
  /** Row key for the X axis (a day string / bucket label). */
  xKey: string;
  series: TrendSeries[];
  /** Currency values: compact on the axis, exact in the tooltip. */
  money?: boolean;
};

export default function TrendChart({ data, xKey, series, money }: Props) {
  const gradientId = useId();
  // One series -> fill the area. More -> lines only (overlapping fills muddy).
  const filled = series.length === 1;

  return (
    <ResponsiveContainer width="100%" height="100%">
      <AreaChart data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
        {filled && (
          <defs>
            {series.map((s, i) => {
              const color = seriesColor(s.colorIndex ?? i);
              return (
                <linearGradient
                  key={s.dataKey}
                  id={`${gradientId}-${s.dataKey}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="0%" stopColor={color} stopOpacity={0.24} />
                  <stop offset="100%" stopColor={color} stopOpacity={0.02} />
                </linearGradient>
              );
            })}
          </defs>
        )}
        {/* Horizontal only + dashed: the grid supports reading values, it isn't a mark. */}
        <CartesianGrid stroke={chartTokens.grid} strokeDasharray="4 4" vertical={false} />
        <XAxis
          dataKey={xKey}
          tickLine={false}
          axisLine={{ stroke: chartTokens.grid }}
          tick={{ fill: chartTokens.tick, fontSize: chartTokens.fontSize }}
          tickMargin={8}
          minTickGap={16}
        />
        <YAxis
          width={money ? 52 : 40}
          tickLine={false}
          axisLine={false}
          tick={{ fill: chartTokens.tick, fontSize: chartTokens.fontSize }}
          tickMargin={4}
          tickFormatter={formatAxisTick}
        />
        <Tooltip
          cursor={{ stroke: chartTokens.cursor, strokeWidth: 1, strokeDasharray: "3 3" }}
          content={<ChartTooltip money={money} />}
        />
        {series.length > 1 && (
          <Legend
            iconType="plainline"
            iconSize={10}
            wrapperStyle={{ fontSize: chartTokens.fontSize, paddingTop: 4 }}
            // Legend text wears a text token — identity is carried by the swatch.
            formatter={(value) => (
              <span style={{ color: chartTokens.tick }}>{String(value)}</span>
            )}
          />
        )}
        {series.map((s, i) => {
          const color = seriesColor(s.colorIndex ?? i);
          return (
            <Area
              key={s.dataKey}
              type="monotone"
              dataKey={s.dataKey}
              name={s.label}
              stroke={color}
              strokeWidth={2}
              fill={filled ? `url(#${gradientId}-${s.dataKey})` : "none"}
              fillOpacity={1}
              dot={false}
              // 8px marker with a 2px surface ring so it reads on top of the line.
              activeDot={{ r: 4, fill: color, stroke: chartTokens.surface, strokeWidth: 2 }}
            />
          );
        })}
      </AreaChart>
    </ResponsiveContainer>
  );
}

type TooltipRow = { dataKey?: string | number; name?: string; value?: number | string; color?: string };

// Chakra-rendered tooltip: bg.panel card instead of Recharts' hardcoded white
// box (which stayed white in dark mode).
function ChartTooltip({
  active,
  payload,
  label,
  money,
}: {
  active?: boolean;
  payload?: TooltipRow[];
  label?: string | number;
  money?: boolean;
}) {
  if (!active || !payload?.length) return null;
  return (
    <Box
      bg="bg.panel"
      borderWidth="1px"
      borderColor="border"
      borderRadius="md"
      boxShadow="md"
      px={3}
      py={2}
      minW="150px"
    >
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {String(label ?? "")}
      </Text>
      <Stack gap={1}>
        {payload.map((row) => (
          <HStack key={String(row.dataKey)} justify="space-between" gap={4}>
            <HStack gap={2} minW={0}>
              <Box
                w="8px"
                h="8px"
                borderRadius="full"
                flexShrink={0}
                style={{ background: row.color }}
              />
              <Text fontSize="xs" color="fg.muted" truncate>
                {row.name}
              </Text>
            </HStack>
            <Text fontSize="xs" fontWeight="medium" fontFamily="mono" color="fg">
              {formatChartValue(Number(row.value ?? 0), money)}
            </Text>
          </HStack>
        ))}
      </Stack>
    </Box>
  );
}
