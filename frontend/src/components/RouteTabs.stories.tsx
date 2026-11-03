import { Box, Flex, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import RouteTabs from "./RouteTabs";
import { storyDocs } from "../routes/dev/storyDocs";
import { MOBILE } from "../screens/viewports";

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
  title: "components/layout/RouteTabs",
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
 * Many tabs in a narrow CONTAINER on a wide viewport — a rail's panel, say.
 * Triggers never shrink, so the strip scrolls instead of overlapping its own
 * labels. Note what scrolling costs: the tabs past the edge give no hint that
 * they exist. That is tolerable here, where the page around it is wide; on a
 * phone it is not, which is what the Phone story below shows.
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

/**
 * The same tabs on a phone (390px): not a strip at all, but an `<EnumSelect>`
 * of the same items — `<RouteTabs>` switches on its own via `useTabsAsSelect()`,
 * so no caller passes anything.
 *
 * Compare with `Overflowing` above. At this width the strip fitted three of
 * these six triggers, cut a fourth mid-word and put two entirely off screen —
 * and since nothing scrolled the active trigger into view, landing on
 * `/settings/backups` showed a strip with no tab selected. The picker cannot
 * clip an option, and its trigger always names where you are.
 */
export const Phone: Story = {
  args: { items: SETTINGS_TABS },
  globals: MOBILE,
  parameters: { router: { initialEntries: ["/settings/backups"] } },
};
