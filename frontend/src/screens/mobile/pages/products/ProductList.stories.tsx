import type { Meta } from "@storybook/react";

import type Products from "../../../../routes/inventory/Products";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/products";
import { MOBILE, SCREEN } from "../../../viewports";

// Products as a whole screen of the mobile version (390px): no rail — the bottom bar ([Home | current page] · Menu · Profile) is the nav.
// The real AppShell wraps the page — the bottom bar and its Menu drawer; a
// phone has no TopBar — so this is the app as a user sees it.
// Data + states live in scenarios/products.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/mobile/pages/products/ProductList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: MOBILE,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Products>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Cashier = withShell(scenario.stories.Cashier);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
export const Empty = withShell(scenario.stories.Empty);
export const Loading = withShell(scenario.stories.Loading);
export const LoadFailed = withShell(scenario.stories.LoadFailed);
