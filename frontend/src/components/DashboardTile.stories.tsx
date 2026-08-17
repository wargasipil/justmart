import { SimpleGrid } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import DashboardTile from "./DashboardTile";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "Layout & page chrome/DashboardTile",
  component: DashboardTile,
  parameters: storyDocs("dashboard-tile"),
  tags: ["autodocs"],
  argTypes: {
    tone: { control: "inline-radio", options: ["default", "warning", "danger"] },
    to: { control: "text" },
  },
  args: { label: "Revenue today", value: "Rp 4.250.000" },
} satisfies Meta<typeof DashboardTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithHint: Story = {
  args: { hint: "12 sales" },
};

/** `to` makes the whole tile a router link — the Dashboard's tiles all drill in. */
export const Clickable: Story = {
  args: { label: "Low stock", value: "7", to: "/products" },
};

export const Warning: Story = {
  args: { label: "Low stock", value: "7", tone: "warning", to: "/products" },
};

export const Danger: Story = {
  args: { label: "Expired lots", value: "2", tone: "danger" },
};

/** The tones side by side — how an alert reads against its neighbours. */
export const ToneRow: Story = {
  render: () => (
    <SimpleGrid columns={{ base: 1, md: 3 }} gap={4}>
      <DashboardTile label="Revenue today" value="Rp 4.250.000" hint="12 sales" />
      <DashboardTile label="Low stock" value="7" tone="warning" to="/products" />
      <DashboardTile label="Expired lots" value="2" tone="danger" />
    </SimpleGrid>
  ),
};

/**
 * Zero is a real state, not a failure — it must render as "0", never as a blank
 * cell. That is why counts go through `formatCount`, not `formatThousands`
 * (which returns "" at zero so a MoneyInput can show its placeholder).
 */
export const ZeroValue: Story = {
  args: { label: "Low stock", value: "0", hint: "nothing below threshold" },
};
