import type { Meta } from "@storybook/react";

import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import ProductMovementsTab from "./ProductMovementsTab";
import * as scenario from "./ProductMovementsTab.scenario";

// The "Mutasi Terbaru" tab of /products/:id, on the bench. States live in
// ProductMovementsTab.scenario.tsx.

const meta = {
  title: "components/products/ProductMovementsTab",
  ...scenario.meta,
  component: ProductMovementsTab,
  globals: DESKTOP,
  parameters: { ...scenario.meta.parameters, ...DESKTOP_PAGE },
} satisfies Meta<typeof ProductMovementsTab>;

export default meta;

export const Manager = scenario.stories.Manager;
export const Busy = scenario.stories.Busy;
export const Empty = scenario.stories.Empty;
export const Loading = scenario.stories.Loading;
export const LoadFailed = scenario.stories.LoadFailed;
