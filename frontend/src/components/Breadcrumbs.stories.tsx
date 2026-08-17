import { Box } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import Breadcrumbs from "./Breadcrumbs";
import { storyDocs } from "../routes/dev/storyDocs";
import { withBusinessMode } from "../routes/dev/storyDecorators";
import { useCrumbLabel } from "../lib/breadcrumbs";

// Unlike the in-app gallery — which shows a static stand-in, since the real
// component reads the live URL — Storybook can mount the REAL <Breadcrumbs/>:
// the preview's MemoryRouter takes its entry from parameters.router, so each
// story below is the genuine trail for that path.

const meta = {
  title: "Layout & page chrome/Breadcrumbs",
  component: Breadcrumbs,
  parameters: storyDocs("breadcrumbs"),
  tags: ["autodocs"],
  // Framed like the TopBar slot it actually lives in.
  decorators: [
    (Story) => (
      <Box borderWidth="1px" borderRadius="md" px={4} h="56px" display="flex" alignItems="center">
        <Story />
      </Box>
    ),
  ],
} satisfies Meta<typeof Breadcrumbs>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A top-level page: just the leaf, unlinked, because you are already there. */
export const TopLevel: Story = {
  parameters: { router: { initialEntries: ["/customers"] } },
};

/**
 * A nested page under a nav GROUP. "Inventaris" is a `@`-prefixed virtual crumb
 * — a sidebar section with no page of its own — so it renders unlinked while
 * the real ancestors are links.
 */
export const UnderNavGroup: Story = {
  parameters: { router: { initialEntries: ["/inventory/batches"] } },
};

export const Deep: Story = {
  parameters: { router: { initialEntries: ["/inventory/stocktake"] } },
};

/**
 * A detail page contributes its leaf label at runtime via `useCrumbLabel()`.
 * Pass `undefined` while the query is loading and the crumb is OMITTED — never
 * a raw UUID, never a "—" flash.
 */
export const DetailPageLeaf: Story = {
  parameters: { router: { initialEntries: ["/products/9c3f-uuid"] } },
  render: () => {
    function WithLabel() {
      useCrumbLabel("Paracetamol 500mg");
      return <Breadcrumbs />;
    }
    return <WithLabel />;
  },
};

export const DetailPageLoading: Story = {
  parameters: { router: { initialEntries: ["/products/9c3f-uuid"] } },
  render: () => {
    function WhileLoading() {
      useCrumbLabel(undefined);
      return <Breadcrumbs />;
    }
    return <WhileLoading />;
  },
};

/**
 * The catalog noun is mode-aware: the same `/products` route crumbs as "Produk"
 * in retail and "Obat" in pharmacy mode, via `pharmacyLabelKey`.
 */
export const RetailNoun: Story = {
  parameters: { router: { initialEntries: ["/products"] } },
  decorators: [withBusinessMode("retail")],
};

export const PharmacyNoun: Story = {
  parameters: { router: { initialEntries: ["/products"] } },
  decorators: [withBusinessMode("pharmacy")],
};
