import type { Meta } from "@storybook/react";

import type NewPurchaseOrder from "../../../../routes/purchasing/NewPurchaseOrder";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/newPurchaseOrder";
import { DESKTOP, SCREEN } from "../../../viewports";

// Writing a restock order as a whole screen of the desktop version (1440px):
// the fixed sidebar rail beside the page, TopBar above it — the app as a buyer
// sees it while typing an order off a distributor's price list.
// Data + states live in scenarios/newPurchaseOrder.tsx; the shell's own reads
// are prepended by withShell.

const meta = {
  title: "screens/desktop/pages/purchasing/NewRestockOrder",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
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
