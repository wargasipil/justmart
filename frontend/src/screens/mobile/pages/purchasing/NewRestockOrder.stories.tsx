import type { Meta } from "@storybook/react";

import type NewPurchaseOrder from "../../../../routes/purchasing/NewPurchaseOrder";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/newPurchaseOrder";
import { MOBILE, SCREEN } from "../../../viewports";

// Writing a restock order as a whole screen of the mobile version (390px): no
// rail — the bottom bar ([Home | current page] · Menu · Profile) is the nav,
// and a phone has no TopBar. This is the densest form in the app on the
// narrowest screen, so it is also where the no-horizontal-scroll rule is worth
// checking: the line table scrolls inside its own TableScroll, and the header
// fields, the totals card and the action row all have to wrap instead of
// pushing the page sideways.
// Data + states live in scenarios/newPurchaseOrder.tsx; the shell's own reads
// are prepended by withShell.

const meta = {
  title: "screens/mobile/pages/purchasing/NewRestockOrder",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof NewPurchaseOrder>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Admin = withShell(scenario.stories.Admin);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
export const AboveAgreedPrice = withShell(scenario.stories.AboveAgreedPrice);
export const NoAgreements = withShell(scenario.stories.NoAgreements);
export const MultipleManufacturers = withShell(scenario.stories.MultipleManufacturers);
export const MissingManufacturer = withShell(scenario.stories.MissingManufacturer);
export const EmptyCatalog = withShell(scenario.stories.EmptyCatalog);
export const CreateFailed = withShell(scenario.stories.CreateFailed);
