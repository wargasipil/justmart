import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import MetricTable from "./MetricTable";
import { storyDocs } from "../routes/dev/storyDocs";
import {
  MetricOrder,
  MetricStock,
  MetricType,
  Sort,
} from "../gen/analytics_iface/v1/analytics_pb";

const DAYS = ["2026-07-28", "2026-07-29", "2026-07-30"];

const ORDER = new MetricOrder({
  data: {
    "2026-07-28": { terjual: 1_250_000n, hpp: 800_000n, profit: 450_000n, avgSold: 12n },
    "2026-07-29": { terjual: 2_100_000n, hpp: 1_340_000n, profit: 760_000n, avgSold: 19n },
    "2026-07-30": { terjual: 880_000n, hpp: 610_000n, profit: 270_000n, avgSold: 8n },
  },
});

const STOCK = new MetricStock({
  data: {
    "2026-07-28": { ready: 640n, ongoing: 120n, expiring: 0n },
    "2026-07-29": { ready: 590n, ongoing: 120n, expiring: 24n },
    "2026-07-30": { ready: 540n, ongoing: 60n, expiring: 24n },
  },
});

const LABELS = new Map(DAYS.map((d) => [d, d]));

const meta = {
  title: "Data display/MetricTable",
  component: MetricTable,
  parameters: storyDocs("metric-table"),
  tags: ["autodocs"],
  argTypes: {
    ids: { control: false },
    order: { control: false },
    stock: { control: false },
    labelById: { control: false },
    metricTypes: { control: false },
    visibleFields: { control: false },
    onSortChange: { control: false },
    sort: { control: false },
  },
  args: {
    ids: DAYS,
    order: ORDER,
    stock: STOCK,
    metricTypes: [MetricType.ORDER, MetricType.STOCK],
    labelById: LABELS,
    dimensionHeader: "Harian",
  },
  // Sorting is a real interaction, so hold it in state rather than freezing it.
  render: (args) => {
    function Demo() {
      const [sort, setSort] = useState<Sort | undefined>(args.sort);
      return <MetricTable {...args} sort={sort} onSortChange={setSort} />;
    }
    return <Demo />;
  },
} satisfies Meta<typeof MetricTable>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Grouped headers, click-to-sort columns, ids resolved to labels through a
 * map. The handler returns IDs and numbers only — display names come from the
 * `Resolve<Domain>(ids)` hooks, which is what keeps analytics N+1-free.
 *
 * Clicking a column header cycles unsorted → DESC → ASC.
 */
export const OrderAndStock: Story = {};

/** Only the ORDER group requested — the stock columns aren't rendered at all. */
export const OrderOnly: Story = {
  args: { metricTypes: [MetricType.ORDER], stock: undefined },
};

/**
 * `visibleFields` hides individual columns client-side (driven by
 * `<ColumnsPopover>`); the backend still ships the whole group's block.
 */
export const SomeColumnsHidden: Story = {
  args: { visibleFields: new Set(["order.terjual", "order.profit", "stock.ready"]) },
};

export const Loading: Story = {
  args: { isLoading: true, ids: [] },
};

/** No rows in range — the table still renders its headers. */
export const Empty: Story = {
  args: { ids: [], order: undefined, stock: undefined },
};
