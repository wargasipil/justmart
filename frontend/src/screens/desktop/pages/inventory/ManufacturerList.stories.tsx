import type { Meta } from "@storybook/react";

import type Manufacturers from "../../../../routes/inventory/Manufacturers";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/manufacturers";
import { DESKTOP, SCREEN } from "../../../viewports";

// Pabrik as a whole screen of the desktop version (1440px): the fixed sidebar rail beside the page.
// The real AppShell wraps the page — sidebar, TopBar with the warehouse
// picker, low-stock bell and user menu — so this is the app as a user sees it.
// Data + states live in scenarios/manufacturers.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/desktop/pages/inventory/ManufacturerList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
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
