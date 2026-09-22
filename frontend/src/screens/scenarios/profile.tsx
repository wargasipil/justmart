import type { Decorator, StoryObj } from "@storybook/react";
import { useContext, useMemo } from "react";
import { Route } from "react-router-dom";

import { UserService } from "../../gen/user_iface/v1/users_connect";
import { WarehouseService } from "../../gen/warehouse_iface/v1/warehouse_connect";
import { AuthContext } from "../../lib/auth";
import { WAREHOUSES, avatarJpeg, daysAgo } from "../../routes/dev/fixtures";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc } from "../../routes/dev/storyMocks";
import Profile from "../../routes/Profile";

// /profile — the signed-in user's own account page. It reads the user from
// AuthContext (withPageContext), the caller's warehouse memberships (the
// Default-warehouse tab only exists with 2+), and — only when the user has a
// picture — GetAvatar. Everything else on it is local preference state.
//
// Rendered by screens/{desktop,mobile}/pages/users/Profile.stories.tsx.

function myWarehouses(warehouses: typeof WAREHOUSES) {
  return mockRpc(WarehouseService, "listUserWarehouses", {
    warehouses,
    memberships: warehouses.map((w) => ({ userId: "", warehouseId: w.id, isDefault: w.isDefault })),
    total: warehouses.length,
  });
}

/**
 * Give the signed-in story user a picture. STORY_USERS carry none (so every
 * other page story renders initials with zero avatar requests); this nests an
 * AuthContext inside withPageContext's, so the page AND the TopBar avatar see
 * the same user with an `avatarUpdatedAt`.
 */
const withPicture: Decorator = (Story) => {
  const auth = useContext(AuthContext);
  const value = useMemo(() => {
    if (!auth?.user) return auth;
    const user = auth.user.clone();
    user.avatarUpdatedAt = daysAgo(3);
    return { ...auth, user };
  }, [auth]);
  return (
    <AuthContext.Provider value={value}>
      <Story />
    </AuthContext.Provider>
  );
};

// --- stories ----------------------------------------------------------------

export const routes = <Route path="/profile" element={<Profile />} />;

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: Profile,
  parameters: {
    router: { initialEntries: ["/profile"] },
    msw: mockApi(myWarehouses(WAREHOUSES)),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Owner: story(
    "An owner with access to two warehouses: four tabs (Account · Preferences · Default " +
      "warehouse · Security). No picture yet, so the avatar is initials and the button reads " +
      "Upload — and no GetAvatar request is made. On desktop the sections are tabs in a rail " +
      "beside the panel; on a phone there are no tabs — the four sections stack as cards.",
  ),
  Cashier: story(
    "A cashier who works a single warehouse: the Default-warehouse tab is gone (there is " +
      "nothing to pick), leaving three sections. Identity stays read-only for every role.",
    {
      parameters: {
        pageContext: { user: "cashier" },
        msw: mockApi(myWarehouses(WAREHOUSES.filter((w) => w.isDefault))),
      },
    },
  ),
  WithPicture: story(
    "The user has uploaded a picture: the avatar loads the THUMB rendition via GetAvatar, the " +
      "button reads Change, and the remove (trash) action appears.",
    {
      decorators: [withPicture],
      parameters: {
        msw: mockApi(
          myWarehouses(WAREHOUSES),
          mockRpc(UserService, "getAvatar", async (req) => ({
            imageData: await avatarJpeg(),
            contentType: "image/jpeg",
            variant: req.variant,
            avatarUpdatedAt: daysAgo(3),
          })),
        ),
      },
    },
  ),
};
