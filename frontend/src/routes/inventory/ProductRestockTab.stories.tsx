import type { Meta } from "@storybook/react";

import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import ProductRestockTab from "./ProductRestockTab";
import * as scenario from "./ProductRestockTab.scenario";

// The "Riwayat Harga Restok" tab of /products/:id, on the bench. States live in
// ProductRestockTab.scenario.tsx.

const meta = {
  title: "components/products/ProductRestockTab",
  ...scenario.meta,
  component: ProductRestockTab,
  globals: DESKTOP,
  parameters: { ...scenario.meta.parameters, ...DESKTOP_PAGE },
} satisfies Meta<typeof ProductRestockTab>;

export default meta;

export const Manager = scenario.stories.Manager;
export const MultipleManufacturers = scenario.stories.MultipleManufacturers;
export const SingleSupplier = scenario.stories.SingleSupplier;
export const Paged = scenario.stories.Paged;
export const Empty = scenario.stories.Empty;
export const Loading = scenario.stories.Loading;
export const LoadFailed = scenario.stories.LoadFailed;
