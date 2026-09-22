import type { Meta } from "@storybook/react";

import type PurchaseOrderDetail from "../../../../routes/purchasing/PurchaseOrderDetail";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/purchaseOrderDetail";
import { DESKTOP, SCREEN } from "../../../viewports";

// One restock order as a whole screen of the desktop version (1440px): the
// fixed sidebar rail beside the page, TopBar above it — the app as a user sees
// it.
// Data + states live in scenarios/purchaseOrderDetail.tsx; the shell's own
// reads are prepended by withShell.

const meta = {
  title: "screens/desktop/pages/purchasing/RestockOrder",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof PurchaseOrderDetail>;

export default meta;

export const Draft = withShell(scenario.stories.Draft);
export const Sent = withShell(scenario.stories.Sent);
export const Partial = withShell(scenario.stories.Partial);
export const Received = withShell(scenario.stories.Received);
export const Closed = withShell(scenario.stories.Closed);
export const SupplierCredit = withShell(scenario.stories.SupplierCredit);
export const Voided = withShell(scenario.stories.Voided);
export const Loading = withShell(scenario.stories.Loading);
export const NotFound = withShell(scenario.stories.NotFound);
