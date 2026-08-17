import { Button } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import ChartCard from "./ChartCard";
import TrendChart from "./TrendChart";
import { storyDocs } from "../routes/dev/storyDocs";

const DATA = [
  { day: "2026-07-24", revenue: 1_250_000 },
  { day: "2026-07-25", revenue: 2_100_000 },
  { day: "2026-07-26", revenue: 880_000 },
  { day: "2026-07-27", revenue: 1_640_000 },
  { day: "2026-07-28", revenue: 1_980_000 },
  { day: "2026-07-29", revenue: 2_540_000 },
  { day: "2026-07-30", revenue: 1_420_000 },
];

const plot = <TrendChart data={DATA} xKey="day" money series={[{ dataKey: "revenue", label: "Terjual" }]} />;

const meta = {
  title: "Data display/ChartCard",
  component: ChartCard,
  parameters: storyDocs("chart-card"),
  tags: ["autodocs"],
  argTypes: { children: { control: false }, actions: { control: false } },
  args: {
    title: "Pendapatan",
    description: "7 hari terakhir",
    height: "220px",
    children: plot,
  },
} satisfies Meta<typeof ChartCard>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The frame every chart sits in. The title names the single series, which is
 * why a one-series chart needs no legend.
 */
export const Default: Story = {};

export const Loading: Story = {
  args: { isLoading: true },
};

/** Empty is a real state — an empty date range, a fresh install. */
export const Empty: Story = {
  args: { isEmpty: true },
};

/** The `actions` slot takes a filter or a toggle, right-aligned in the header. */
export const WithActions: Story = {
  args: {
    actions: (
      <Button size="xs" variant="outline">
        30 hari
      </Button>
    ),
  },
};

export const NoDescription: Story = {
  args: { description: undefined },
};
