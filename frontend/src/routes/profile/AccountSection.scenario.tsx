import type { StoryObj } from "@storybook/react";

import { Role } from "../../gen/auth_iface/v1/policy_pb";
import { UserService } from "../../gen/user_iface/v1/users_connect";
import { User } from "../../gen/user_iface/v1/users_pb";
import { avatarJpeg, daysAgo } from "../dev/fixtures";
import { STORY_USERS, withPageContext } from "../dev/storyDecorators";
import { mockApi, mockRpc } from "../dev/storyMocks";
import AccountSection from "./AccountSection";

// The Akun card of /profile on its own — the user's picture beside their
// read-only name, role and email. Profile passes it the signed-in user; here
// the user is an arg, so each state is just a different fixture.
//
// With no picture it makes no request at all (UserAvatar's version-0 rule);
// only WithPicture reaches the network, for the THUMB rendition.
//
// Rendered by AccountSection.stories.tsx (desktop, "components/users/Account")
// and AccountSection.mobile.stories.tsx ("components/users/AccountMobile").

const withAvatar = (u: User) => {
  const c = u.clone();
  c.avatarUpdatedAt = daysAgo(3);
  return c;
};

const getAvatar = mockRpc(UserService, "getAvatar", async (req) => ({
  imageData: await avatarJpeg(),
  contentType: "image/jpeg",
  variant: req.variant,
  avatarUpdatedAt: daysAgo(3),
}));

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: AccountSection,
  args: { user: STORY_USERS.owner },
  // AvatarPicker's upload/delete mutations refresh the signed-in user through
  // useAuth(), so the card needs an AuthContext even though it takes `user`
  // as a prop. Upload/Delete are only reached by clicking; the catch-all
  // answers them by name if tried.
  decorators: [withPageContext],
  parameters: { msw: mockApi() },
};

const story = (description: string, s: StoryObj<typeof AccountSection> = {}): StoryObj<typeof AccountSection> => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  NoPicture: story(
    "No picture yet: initials in the avatar, an \"Unggah foto\" button, no remove action — and no " +
      "GetAvatar request.",
  ),
  WithPicture: story(
    "A picture is set: the THUMB rendition loads via GetAvatar, the button reads \"Ganti foto\", " +
      "and the trash action appears.",
    {
      args: { user: withAvatar(STORY_USERS.owner) },
      parameters: { msw: mockApi(getAvatar) },
    },
  ),
  Cashier: story("A cashier: the same card, with the Kasir role badge.", {
    args: { user: STORY_USERS.cashier },
  }),
  LongName: story(
    "A long name and email. On a phone the avatar sits on top so the identity gets the full " +
      "card width, and both WRAP rather than truncate. On desktop they sit beside the avatar " +
      "and truncate with an ellipsis.",
    {
      args: {
        user: new User({
          id: "user-long",
          name: "Maria Magdalena Kusumawardhani Sutanto",
          email: "maria.magdalena.kusumawardhani@tokosejahtera.test",
          role: Role.PHARMACIST,
          active: true,
        }),
      },
    },
  ),
};
