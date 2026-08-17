import { HStack, Stack, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import UserAvatar from "./UserAvatar";
import { storyDocs } from "../routes/dev/storyDocs";

// `version={0}` means "no picture", which disables the fetch entirely — so
// these stories render the initials fallback and make no request. A story with
// a real version would need the backend and a real user id.

const meta = {
  title: "Data display/UserAvatar",
  component: UserAvatar,
  parameters: storyDocs("user-avatar"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["2xs", "xs", "sm", "md", "lg", "xl", "2xl"] },
  },
  args: { userId: "demo-1", name: "Siti Rahmawati", version: 0 },
} satisfies Meta<typeof UserAvatar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** No picture (`version: 0`) → initials, and zero requests. */
export const InitialsFallback: Story = {};

export const Sizes: Story = {
  render: () => (
    <HStack gap={4} align="center" flexWrap="wrap">
      {(["2xs", "xs", "sm", "md", "lg", "xl", "2xl"] as const).map((size) => (
        <Stack key={size} gap={1} align="center">
          <UserAvatar userId={`demo-${size}`} name="Siti Rahmawati" version={0} size={size} />
          <Text fontSize="xs" color="fg.muted">
            {size}
          </Text>
        </Stack>
      ))}
    </HStack>
  ),
};

/** Initials are derived from the name — one word, two, or an unusual one. */
export const NameVariants: Story = {
  render: () => (
    <HStack gap={4} align="center" flexWrap="wrap">
      {["Budi", "Siti Rahmawati", "Ahmad Fauzi Wijaya", "R."].map((name) => (
        <Stack key={name} gap={1} align="center">
          <UserAvatar userId={name} name={name} version={0} size="md" />
          <Text fontSize="xs" color="fg.muted">
            {name}
          </Text>
        </Stack>
      ))}
    </HStack>
  ),
};

/**
 * `full` loads the ORIGINAL rendition instead of the thumbnail. Leave it OFF
 * everywhere except a genuine full-size view — a 25-row user table on originals
 * is megabytes; on thumbs it is tens of KB. (With `version: 0` neither is
 * fetched, so this story shows the API, not a difference.)
 */
export const FullRendition: Story = {
  args: { size: "2xl", full: true },
};
