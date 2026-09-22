import type { Meta } from "@storybook/react";

import type Profile from "../../../../routes/Profile";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/profile";
import { MOBILE, SCREEN } from "../../../viewports";

// Profile as a whole screen of the mobile version (390px): no rail — the bottom bar ([Home | current page] · Menu · Profile) is the nav, with Profile active.
// The real AppShell wraps the page — the bottom bar and its Menu drawer; a
// phone has no TopBar — so this is the app as a user sees it.
// Data + states live in scenarios/profile.tsx; the shell's own reads are
// added by withShell.

const meta = {
  title: "screens/mobile/pages/users/Profile",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Profile>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Cashier = withShell(scenario.stories.Cashier);
export const WithPicture = withShell(scenario.stories.WithPicture);
