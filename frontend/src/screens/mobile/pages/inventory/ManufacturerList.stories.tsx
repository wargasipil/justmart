import type { Meta } from "@storybook/react";

import type Manufacturers from "../../../../routes/inventory/Manufacturers";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/manufacturers";
import { MOBILE, SCREEN } from "../../../viewports";

// Pabrik as a whole screen of the mobile version (390px): no rail — the bottom bar ([Home | current page] · Menu · Profile) is the nav.
// The real AppShell wraps the page — the bottom bar and its Menu drawer; a
// phone has no TopBar — so this is the app as a user sees it.
// Data + states live in scenarios/manufacturers.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/mobile/pages/inventory/ManufacturerList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Manufacturers>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Admin = withShell(scenario.stories.Admin);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
export const Empty = withShell(scenario.stories.Empty);
export const DuplicateCode = withShell(scenario.stories.DuplicateCode);
export const Loading = withShell(scenario.stories.Loading);
export const LoadFailed = withShell(scenario.stories.LoadFailed);
