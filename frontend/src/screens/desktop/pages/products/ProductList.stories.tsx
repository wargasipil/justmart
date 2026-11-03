import type { Meta } from "@storybook/react";

import type Products from "../../../../routes/inventory/Products";
import { screenRender, withShell } from "../../../frames";
import * as scenario from "../../../scenarios/products";
import { DESKTOP, SCREEN } from "../../../viewports";

// Products as a whole screen of the desktop version (1440px): the fixed sidebar rail beside the page.
// The real AppShell wraps the page — sidebar, TopBar with the warehouse
// picker, low-stock bell and user menu — so this is the app as a user sees it.
// Data + states live in scenarios/products.tsx; the shell's own reads are
// prepended by withShell.

const meta = {
  title: "screens/desktop/pages/products/ProductList",
  ...scenario.meta,
  render: screenRender(scenario.routes),
  globals: DESKTOP,
  parameters: { ...withShell(scenario.meta).parameters, ...SCREEN },
} satisfies Meta<typeof Products>;

export default meta;

export const Owner = withShell(scenario.stories.Owner);
export const Cashier = withShell(scenario.stories.Cashier);
export const Pharmacy = withShell(scenario.stories.Pharmacy);
export const Empty = withShell(scenario.stories.Empty);
export const Loading = withShell(scenario.stories.Loading);
export const LoadFailed = withShell(scenario.stories.LoadFailed);
