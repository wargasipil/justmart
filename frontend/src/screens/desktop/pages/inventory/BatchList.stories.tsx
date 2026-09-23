import type { Meta } from "@storybook/react";

import type Batches from "../../../../routes/inventory/Batches";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/batches";
import { DESKTOP, SCREEN } from "../../../viewports";

// The batch (lot) list as a whole screen of the desktop version (1440px): the
// fixed sidebar rail beside the page, TopBar above it. Seven columns of lot
// detail is desk work, so this is the device the page is really designed for.
// Data + states live in scenarios/batches.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/desktop/pages/inventory/BatchList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Batches>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const ByManufacturer = withShell(scenario.stories.ByManufacturer);
export const UnknownMakers = withShell(scenario.stories.UnknownMakers);
export const Empty = withShell(scenario.stories.Empty);
