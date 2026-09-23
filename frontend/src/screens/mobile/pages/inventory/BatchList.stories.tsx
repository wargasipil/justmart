import type { Meta } from "@storybook/react";

import type Batches from "../../../../routes/inventory/Batches";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/batches";
import { MOBILE, SCREEN } from "../../../viewports";

// The batch (lot) list as a whole screen of the mobile version (390px): no
// rail — the bottom bar is the nav, and a phone has no TopBar. Seven columns
// cannot fit, so this is where the no-horizontal-scroll rule earns its keep:
// the table scrolls inside its own TableScroll while the toolbar's four
// filters wrap instead of pushing the page sideways.
// Data + states live in scenarios/batches.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/mobile/pages/inventory/BatchList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Batches>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const ByManufacturer = withShell(scenario.stories.ByManufacturer);
export const UnknownMakers = withShell(scenario.stories.UnknownMakers);
export const Empty = withShell(scenario.stories.Empty);
