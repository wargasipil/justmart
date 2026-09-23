import type { Meta } from "@storybook/react";

import type PurchaseOrdersList from "../../../../routes/purchasing/PurchaseOrdersList";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/purchasing";
import { MOBILE, SCREEN } from "../../../viewports";

// Restock as a whole screen of the mobile version (390px): no rail — the bottom
// bar ([Home | current page] · Menu · Profile) is the nav, and a phone has no
// TopBar. Below md the status tab strip is not a strip at all but an
// <EnumSelect> of the same items: eight triggers cannot fit 390px, and a strip
// that clips is one that hides where you are.
// Data + states live in scenarios/purchasing.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/mobile/pages/purchasing/RestockList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof PurchaseOrdersList>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Admin = withShell(scenario.stories.Admin);
export const Sent = withShell(scenario.stories.Sent);
export const Voided = withShell(scenario.stories.Voided);
export const ByManufacturer = withShell(scenario.stories.ByManufacturer);
export const SuppliersLedger = withShell(scenario.stories.SuppliersLedger);
export const Empty = withShell(scenario.stories.Empty);
export const Loading = withShell(scenario.stories.Loading);
export const LoadFailed = withShell(scenario.stories.LoadFailed);
