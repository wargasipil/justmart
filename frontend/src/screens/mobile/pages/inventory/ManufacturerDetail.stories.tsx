import type { Meta } from "@storybook/react";

import type ManufacturerDetail from "../../../../routes/inventory/ManufacturerDetail";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/manufacturerDetail";
import { MOBILE, SCREEN } from "../../../viewports";

// One pabrik as a whole screen of the mobile version (390px): no rail — the bottom bar ([Home | current page] · Menu · Profile) is the nav.
// The real AppShell wraps the page — the bottom bar and its Menu drawer; a
// phone has no TopBar — so this is the app as a user sees it.
// Data + states live in scenarios/manufacturerDetail.tsx; the shell's own reads
// are prepended by withShell.

const meta = {
  title: "screens/mobile/pages/inventory/ManufacturerDetail",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof ManufacturerDetail>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
export const NoProducts = withShell(scenario.stories.NoProducts);
export const Archived = withShell(scenario.stories.Archived);
export const Loading = withShell(scenario.stories.Loading);
export const NotFound = withShell(scenario.stories.NotFound);
