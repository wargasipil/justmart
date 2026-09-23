import type { Meta } from "@storybook/react";

import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import DiscountTab from "./ProductDiscountTab";
import * as scenario from "./ProductDiscountTab.scenario";

// The "Diskon" tab of /products/:id, on the bench. States live in
// ProductDiscountTab.scenario.tsx and run over the in-memory table in
// ProductDiscountTab.shop.ts — this is the one product-detail tab that writes.

const meta = {
  title: "components/products/ProductDiscountTab",
  ...scenario.meta,
  component: DiscountTab,
  globals: DESKTOP,
  parameters: { ...scenario.meta.parameters, ...DESKTOP_PAGE },
} satisfies Meta<typeof DiscountTab>;

export default meta;

export const Manager = scenario.stories.Manager;
export const Adding = scenario.stories.Adding;
export const Deleting = scenario.stories.Deleting;
export const Empty = scenario.stories.Empty;
export const Loading = scenario.stories.Loading;
export const LoadFailed = scenario.stories.LoadFailed;
