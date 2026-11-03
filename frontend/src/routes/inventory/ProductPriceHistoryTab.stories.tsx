import type { Meta } from "@storybook/react";

import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import ProductPriceHistoryTab from "./ProductPriceHistoryTab";
import * as scenario from "./ProductPriceHistoryTab.scenario";

// The "Riwayat Harga Jual" tab of /products/:id, on the bench. States live in
// ProductPriceHistoryTab.scenario.tsx; two of them reach the Grosir panel
// through a play() click, since the segment is local state.

const meta = {
  title: "components/products/ProductPriceHistoryTab",
  ...scenario.meta,
  component: ProductPriceHistoryTab,
  globals: DESKTOP,
  parameters: { ...scenario.meta.parameters, ...DESKTOP_PAGE },
} satisfies Meta<typeof ProductPriceHistoryTab>;

export default meta;

export const PerUnit = scenario.stories.PerUnit;
export const Wholesale = scenario.stories.Wholesale;
export const WholesaleEmpty = scenario.stories.WholesaleEmpty;
export const Empty = scenario.stories.Empty;
export const Loading = scenario.stories.Loading;
export const LoadFailed = scenario.stories.LoadFailed;
