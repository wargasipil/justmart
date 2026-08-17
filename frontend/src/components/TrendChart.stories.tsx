import type { Meta, StoryObj } from "@storybook/react";

import ChartCard from "./ChartCard";
import TrendChart from "./TrendChart";
import { storyDocs } from "../routes/dev/storyDocs";

const DATA = [
  { day: "2026-07-24", revenue: 1_250_000, profit: 450_000, cogs: 800_000 },
  { day: "2026-07-25", revenue: 2_100_000, profit: 760_000, cogs: 1_340_000 },
  { day: "2026-07-26", revenue: 880_000, profit: 270_000, cogs: 610_000 },
  { day: "2026-07-27", revenue: 1_640_000, profit: 520_000, cogs: 1_120_000 },
  { day: "2026-07-28", revenue: 1_980_000, profit: 690_000, cogs: 1_290_000 },
  { day: "2026-07-29", revenue: 2_540_000, profit: 910_000, cogs: 1_630_000 },
  { day: "2026-07-30", revenue: 1_420_000, profit: 380_000, cogs: 1_040_000 },
];

const meta = {
  title: "Data display/TrendChart",
  component: TrendChart,
  parameters: storyDocs("trend-chart"),
  tags: ["autodocs"],
  argTypes: { data: { control: false }, series: { control: false } },
  args: { data: DATA, xKey: "day", money: true },
  // Always inside its frame — a bare ResponsiveContainer has no height of its own.
  decorators: [
    (Story) => (
      <ChartCard title="Pendapatan" description="7 hari terakhir" height="240px">
        <Story />
      </ChartCard>
    ),
  ],
} satisfies Meta<typeof TrendChart>;

export default meta;
type Story = StoryObj<typeof meta>;

/** One series → a gradient-filled area. Flip the theme toggle: it follows. */
export const SingleSeries: Story = {
  args: { series: [{ dataKey: "revenue", label: "Terjual" }] },
};

/**
 * Two or more → plain lines plus a legend. Translucent fills would muddy each
 * other, so the area treatment is single-series only.
 */
export const MultiSeries: Story = {
  args: {
    series: [
      { dataKey: "revenue", label: "Terjual" },
      { dataKey: "profit", label: "Profit" },
    ],
  },
};

/**
 * `colorIndex` pins a metric to one slot of the fixed 4-colour order (blue,
 * orange, teal, purple) so it keeps that colour across pages — revenue is
 * always slot 0. Never cycle the palette by rank: colour belongs to the
 * entity, so hiding one series must not repaint the others.
 */
export const PinnedColours: Story = {
  args: {
    series: [
      { dataKey: "revenue", label: "Terjual", colorIndex: 0 },
      { dataKey: "cogs", label: "HPP", colorIndex: 1 },
      { dataKey: "profit", label: "Profit", colorIndex: 2 },
    ],
  },
};

/** Counts rather than money — the axis drops the currency formatting. */
export const NonMoney: Story = {
  args: {
    money: false,
    data: DATA.map((d) => ({ day: d.day, items: Math.round(d.revenue / 12_500) })),
    series: [{ dataKey: "items", label: "Item terjual" }],
  },
};

/** A single point still has to render — a shop's first day of trading. */
export const SinglePoint: Story = {
  args: { data: [DATA[0]], series: [{ dataKey: "revenue", label: "Terjual" }] },
};
