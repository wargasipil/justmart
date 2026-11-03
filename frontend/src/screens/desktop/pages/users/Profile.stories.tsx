import type { Meta } from "@storybook/react";

import type Profile from "../../../../routes/Profile";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/profile";
import { DESKTOP, SCREEN } from "../../../viewports";

// Profile as a whole screen of the desktop version (1440px): the fixed sidebar rail beside the page.
// The real AppShell wraps the page — sidebar, TopBar with the warehouse
// picker, low-stock bell and user menu — so this is the app as a user sees it.
// Data + states live in scenarios/profile.tsx; the shell's own reads are
// added by withShell.

const meta = {
  title: "screens/desktop/pages/users/Profile",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Profile>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Cashier = withShell(scenario.stories.Cashier);
export const WithPicture = withShell(scenario.stories.WithPicture);
