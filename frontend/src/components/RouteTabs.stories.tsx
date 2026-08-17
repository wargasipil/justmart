import { Box, Flex, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import RouteTabs from "./RouteTabs";
import { storyDocs } from "../routes/dev/storyDocs";

const ANALYTICS_TABS = [
  { value: "daily", to: "/analytics/daily", label: "Harian" },
  { value: "product", to: "/analytics/product", label: "Produk" },
  { value: "user", to: "/analytics/user", label: "Pengguna" },
];

const SETTINGS_TABS = [
  { value: "general", to: "/settings/general", label: "Umum" },
  { value: "units", to: "/settings/units", label: "Satuan" },
  { value: "printing", to: "/settings/printing", label: "Cetak" },
  { value: "backups", to: "/settings/backups", label: "Cadangan" },
  { value: "updates", to: "/settings/updates", label: "Pembaruan" },
];

const meta = {
  title: "Layout & page chrome/RouteTabs",
  component: RouteTabs,
  parameters: {
    ...storyDocs("route-tabs"),
    router: { initialEntries: ["/analytics/daily"] },
  },
  tags: ["autodocs"],
  argTypes: {
    orientation: { control: "inline-radio", options: ["horizontal", "vertical"] },
    items: { control: false },
  },
  args: { items: ANALYTICS_TABS },
} satisfies Meta<typeof RouteTabs>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The active tab is the longest `to` prefix of the current path — so it comes
 * from the URL, not from local state. Clicking navigates (client-side); the
 * canvas URL is a MemoryRouter, so the strip updates without a reload.
 */
export const Horizontal: Story = {};

/** A different entry point selects a different tab — nothing else changes. */
export const SecondTabActive: Story = {
  parameters: { router: { initialEntries: ["/analytics/product"] } },
};

/**
 * The vertical rail, as `/settings` uses it. Never hard-code this: call
 * `useTabsOrientation()` so it degrades to a strip on a viewport too narrow
 * for a rail.
 */
export const VerticalRail: Story = {
  args: { items: SETTINGS_TABS, orientation: "vertical" },
  parameters: { router: { initialEntries: ["/settings/printing"] } },
  render: (args) => (
    <Flex direction="row" gap={6}>
      <RouteTabs {...args} />
      <Box flex="1" minW={0}>
        <Text fontSize="sm" color="fg.muted">
          The caller lays the &lt;Outlet /&gt; out beside the rail.
        </Text>
      </Box>
    </Flex>
  ),
};

/**
 * Many tabs in a narrow frame. Triggers never shrink — the strip scrolls
 * instead of overlapping its own labels.
 */
export const Overflowing: Story = {
  args: { items: SETTINGS_TABS },
  parameters: { router: { initialEntries: ["/settings/general"] } },
  render: (args) => (
    <Box maxW="320px" borderWidth="1px" borderRadius="md" p={2}>
      <RouteTabs {...args} />
    </Box>
  ),
};
