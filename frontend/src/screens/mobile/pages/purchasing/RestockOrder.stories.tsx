import type { Meta } from "@storybook/react";

import type PurchaseOrderDetail from "../../../../routes/purchasing/PurchaseOrderDetail";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/purchaseOrderDetail";
import { MOBILE, SCREEN } from "../../../viewports";

// One restock order as a whole screen of the mobile version (390px): no rail,
// the bottom bar is the nav, no TopBar. This is the no-horizontal-scroll check
// for the page — its wide items / deliveries tables scroll inside
// <TableScroll> and the action row wraps, so the document itself must not move
// sideways at 390px.
// Data + states live in scenarios/purchaseOrderDetail.tsx; the shell's own
// reads are prepended by withShell.

const meta = {
  title: "screens/mobile/pages/purchasing/RestockOrder",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof PurchaseOrderDetail>;

export default meta;

export const Draft = withShell(scenario.stories.Draft);
export const Sent = withShell(scenario.stories.Sent);
export const Partial = withShell(scenario.stories.Partial);
export const Received = withShell(scenario.stories.Received);
export const Closed = withShell(scenario.stories.Closed);
export const SupplierCredit = withShell(scenario.stories.SupplierCredit);
export const UnrecordedMaker = withShell(scenario.stories.UnrecordedMaker);
export const Voided = withShell(scenario.stories.Voided);
export const Loading = withShell(scenario.stories.Loading);
export const NotFound = withShell(scenario.stories.NotFound);
