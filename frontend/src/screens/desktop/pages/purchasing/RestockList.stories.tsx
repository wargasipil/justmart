import type { Meta } from "@storybook/react";

import type PurchaseOrdersList from "../../../../routes/purchasing/PurchaseOrdersList";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/purchasing";
import { DESKTOP, SCREEN } from "../../../viewports";

// Restock as a whole screen of the desktop version (1440px): the fixed sidebar
// rail beside the page. The real AppShell wraps it — rail, TopBar with the
// warehouse picker, low-stock bell and user menu — so this is the app as a user
// sees it.
// Data + states live in scenarios/purchasing.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/desktop/pages/purchasing/RestockList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
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
