import type { Meta } from "@storybook/react";

import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import ProductBatchesTab from "./ProductBatchesTab";
import * as scenario from "./ProductBatchesTab.scenario";

// The Batch tab of /products/:id, on the bench. States live in
// ProductBatchesTab.scenario.tsx.
//
// Filed under components/products/ because that is where this tab is read
// about, even though the file sits beside the component in routes/inventory —
// the same arrangement routes/profile/AccountSection uses. It is page-local
// JSX, so it has no /components gallery entry and no storyDocs: the prose that
// would live in the registry lives in the state descriptions instead.

const meta = {
  title: "components/products/ProductBatchesTab",
  ...scenario.meta,
  component: ProductBatchesTab,
  globals: DESKTOP,
  parameters: { ...scenario.meta.parameters, ...DESKTOP_PAGE },
} satisfies Meta<typeof ProductBatchesTab>;

export default meta;

export const Manager = scenario.stories.Manager;
export const Till = scenario.stories.Till;
export const Expiring = scenario.stories.Expiring;
export const Paged = scenario.stories.Paged;
export const Empty = scenario.stories.Empty;
export const Loading = scenario.stories.Loading;
export const LoadFailed = scenario.stories.LoadFailed;
