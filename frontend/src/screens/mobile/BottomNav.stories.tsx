import { Box, Flex } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";

import BottomNav from "../../components/BottomNav";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi } from "../../routes/dev/storyMocks";
import { MOBILE, SCREEN } from "../viewports";

// The phone nav bar on its own: [Home | current page] · Menu · Profile.
// Position 1 shows the page you opened from the drawer (same label + icon as
// the drawer); Menu and Profile never move. The URL and `menuOpen` decide
// everything; withPageContext seeds the business mode (Produk vs Obat) and
// mockApi() answers nothing — the bar makes no calls of its own. See
// screens/mobile/App for it inside the real shell, drawer included.

const meta = {
  title: "screens/mobile/BottomNav",
  component: BottomNav,
  globals: MOBILE,
  decorators: [
    // The bar sits in flow at the foot of AppShell's one-screen column; this
    // frame stands in for that column so the bar lands at the bottom here too.
    (Story) => (
      <Flex direction="column" h="100dvh">
        <Box flex="1" />
        <Story />
      </Flex>
    ),
    withPageContext,
  ],
  parameters: { ...SCREEN, router: { initialEntries: ["/"] }, msw: mockApi() },
  args: { menuOpen: false, onOpenMenu: fn() },
} satisfies Meta<typeof BottomNav>;

export default meta;
type Story = StoryObj<typeof meta>;

/** On `/`: Home is in position 1, active. */
export const Home: Story = {};

/** On a page opened from the drawer: it takes position 1 — here Produk. */
export const CurrentPage: Story = {
  parameters: { router: { initialEntries: ["/products"] } },
};

/** A detail route belongs to its list: `/products/:id` still shows Produk. */
export const DetailPage: Story = {
  parameters: { router: { initialEntries: ["/products/abc"] } },
};

/** Pharmacy mode: the same slot reads Obat, with the pill icon. */
export const Pharmacy: Story = {
  parameters: { router: { initialEntries: ["/products"] }, pageContext: { mode: "pharmacy" } },
};

/** A long label (Riwayat order) truncates instead of growing the bar. */
export const LongLabel: Story = {
  parameters: { router: { initialEntries: ["/orders"] } },
};

/** On `/profile`: Profile is active in its own slot; position 1 is Home. */
export const Profile: Story = {
  parameters: { router: { initialEntries: ["/profile"] } },
};

/** Drawer open: Menu is active in place — it never takes position 1. */
export const MenuOpen: Story = {
  args: { menuOpen: true },
  parameters: { router: { initialEntries: ["/products"] } },
};
