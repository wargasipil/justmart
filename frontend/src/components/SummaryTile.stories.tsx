import { SimpleGrid } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import SummaryTile from "./SummaryTile";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "Layout & page chrome/SummaryTile",
  component: SummaryTile,
  parameters: storyDocs("summary-tile"),
  tags: ["autodocs"],
  args: { label: "Ready stock", value: "12.480" },
} satisfies Meta<typeof SummaryTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/**
 * The real shape: a `SimpleGrid` directly under `<PageHeader>` and above the
 * tabs. One figure per tile — a count and its valuation are two cells side by
 * side, not a value with a subtitle, so they stay comparable down the row.
 */
export const SummaryRow: Story = {
  render: () => (
    <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
      <SummaryTile label="Ready stock" value="12.480" />
      <SummaryTile label="Ready value" value="Rp 184.320.000" />
      <SummaryTile label="Ongoing stock" value="3.200" />
      <SummaryTile label="Ongoing value" value="Rp 41.900.000" />
    </SimpleGrid>
  ),
};

/** Empty date range / fresh install. Renders "0", never a blank. */
export const ZeroState: Story = {
  render: () => (
    <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
      <SummaryTile label="Ready stock" value="0" />
      <SummaryTile label="Ready value" value="Rp 0" />
    </SimpleGrid>
  ),
};
