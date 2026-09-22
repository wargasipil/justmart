import type { Meta } from "@storybook/react";

import type AppShell from "../../components/AppShell";
import { screenRender, withShell } from "../frames";
import * as scenario from "../scenarios/app";
import { MOBILE, SCREEN } from "../viewports";

// The mobile app layout (390px): no TopBar, the page (the only thing that
// scrolls), and <BottomNav> —
// [Home | current page] · Menu · Profile — whose Menu slot opens the complete
// nav in a bottom-sheet drawer.
// No rail and no hamburger at this width.
// Data + states live in scenarios/app.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/mobile/App",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof AppShell>;

export default meta;

export const Home = withShell(scenario.stories.Home);
export const OtherPage = withShell(scenario.stories.OtherPage);
export const Profile = withShell(scenario.stories.Profile);
export const MenuOpen = withShell(scenario.stories.MenuOpen);
export const Cashier = withShell(scenario.stories.Cashier);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
