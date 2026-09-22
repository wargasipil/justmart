import type { Meta } from "@storybook/react";

import type ManufacturerDetail from "../../../../routes/inventory/ManufacturerDetail";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/manufacturerDetail";
import { DESKTOP, SCREEN } from "../../../viewports";

// One pabrik as a whole screen of the desktop version (1440px): the fixed sidebar rail beside the page.
// The real AppShell wraps the page — sidebar, TopBar with the warehouse
// picker, low-stock bell and user menu — so this is the app as a user sees it.
// Data + states live in scenarios/manufacturerDetail.tsx; the shell's own reads
// are prepended by withShell.

const meta = {
  title: "screens/desktop/pages/inventory/ManufacturerDetail",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof ManufacturerDetail>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
export const NoProducts = withShell(scenario.stories.NoProducts);
export const Archived = withShell(scenario.stories.Archived);
export const Loading = withShell(scenario.stories.Loading);
export const NotFound = withShell(scenario.stories.NotFound);
