import type { StoryObj } from "@storybook/react";
import { Box, Stack, Text } from "@chakra-ui/react";
import { Route } from "react-router-dom";
import { expect, userEvent, within } from "storybook/test";

import AppShell from "../../components/AppShell";
import PageHeader from "../../components/PageHeader";
import { PHARMACY_CATALOG, RETAIL_CATALOG } from "../../routes/dev/fixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi } from "../../routes/dev/storyMocks";
import ProductDetail from "../../routes/inventory/ProductDetail";
import Products from "../../routes/inventory/Products";
import Profile from "../../routes/Profile";
import { listHandlers } from "./products";

// The app LAYOUT: the real AppShell (desktop: rail + TopBar; phone: bottom bar
// + Menu drawer, no TopBar) around the pages, so navigating is clickable.
// The pages that have a scenario of their own are mounted for real on the
// same fixtures — /products, /products/:id and /profile. Every other path
// (the Dashboard at `/` included) falls through to a stand-in probe, since
// those pages have no mocks yet; the probe is tall enough to show only
// <main> scrolls on a phone. A new page scenario = one more <Route> here.
//
// Rendered by screens/<device>/App.stories.tsx through screenRender(routes),
// which already wraps these routes in <AppShell>.

function LayoutProbe() {
  return (
    <>
      <PageHeader
        title="Layout"
        description="A stand-in: this page has no story mocks yet. The chrome around it is the real AppShell."
      />
      <Stack gap={3}>
        {Array.from({ length: 14 }, (_, i) => (
          <Box key={i} borderWidth="1px" borderRadius="md" bg="bg.subtle" p={4}>
            <Text fontSize="sm" color="fg.muted">
              Row {i + 1}
            </Text>
          </Box>
        ))}
        <Text fontSize="sm" fontWeight="semibold" textAlign="center">
          End of page — the scroll area ends at the bottom bar.
        </Text>
      </Stack>
    </>
  );
}

// A story picks the starting URL (and so the active bottom tab) purely through
// router.initialEntries.
export const routes = (
  <>
    <Route path="/products" element={<Products />} />
    <Route path="/products/:id" element={<ProductDetail />} />
    <Route path="/profile" element={<Profile />} />
    <Route path="*" element={<LayoutProbe />} />
  </>
);

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: AppShell,
  parameters: {
    router: { initialEntries: ["/"] },
    // The shell's own reads (ListUserWarehouses — which Profile shares — and
    // friends) are prepended by withShell in the story file.
    msw: mockApi(...listHandlers(RETAIL_CATALOG)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Home: story("On `/`: Home · Menu · Profile, Home active.", {
    parameters: { router: { initialEntries: ["/"] } },
  }),
  OtherPage: story(
    "On a page opened from the drawer (/products — the real Products page): that page takes " +
      "position 1 — Produk · Menu · Profile. Scroll it: only the page scrolls, and its last " +
      "row stops above the bottom bar. Tap a row to open the real detail page.",
    { parameters: { router: { initialEntries: ["/products"] } } },
  ),
  Profile: story("On `/profile`: Profile is active in its own slot; position 1 is Home.", {
    parameters: { router: { initialEntries: ["/profile"] } },
  }),
  MenuOpen: story(
    "Menu tapped: the complete nav rises in a bottom-sheet drawer, filtered by role and " +
      "business mode exactly like the desktop rail. Menu stays in its own slot.",
    {
      parameters: { router: { initialEntries: ["/products"] } },
      play: async ({ canvasElement }) => {
        const menu = await within(canvasElement).findByRole("button", { name: "Menu" });
        await userEvent.click(menu);
        // The drawer portals out of the canvas, so query the whole document.
        await expect(await within(document.body).findByRole("dialog")).toBeVisible();
      },
    },
  ),
  Cashier: story("A cashier's session: the drawer lists only the till's destinations.", {
    parameters: {
      pageContext: { user: "cashier" },
      router: { initialEntries: ["/"] },
      // Redacted rows and no manager-only reads, exactly as the till gets.
      msw: mockApi(...listHandlers(RETAIL_CATALOG, { till: true })),
    },
  }),
  Pharmacy: story("Pharmacy mode: pill brand in the drawer and the Resep entry appears.", {
    parameters: {
      pageContext: { mode: "pharmacy" },
      router: { initialEntries: ["/"] },
      msw: mockApi(...listHandlers(PHARMACY_CATALOG)),
    },
  }),
};
