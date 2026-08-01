import { Avatar } from "@chakra-ui/react";

import { AvatarVariant } from "../gen/user_iface/v1/users_pb";
import { initials } from "../lib/roles";
import { useAvatarQuery } from "../queries/users";

type Props = {
  userId: string;
  /** Display name — drives the initials fallback and the alt text. */
  name: string;
  /**
   * The user's `avatarUpdatedAt` (unix sec). 0 = no picture, which skips the
   * fetch entirely and renders initials. Also the cache key, so a re-upload
   * shows up without an invalidate.
   */
  version: number;
  size?: "2xs" | "xs" | "sm" | "md" | "lg" | "xl" | "2xl";
  /**
   * Load the full-size ORIGINAL instead of the thumbnail. Only for a deliberate
   * full-size view — see the two-rendition HARD RULE in CLAUDE.md. Every avatar,
   * table cell and list row leaves this off.
   */
  full?: boolean;
};

/**
 * A user's profile picture with an initials fallback.
 *
 * Reads the THUMB rendition by default, so a table of users pulls a few KB per
 * row instead of a few hundred. Users with no picture (`version === 0`) cost
 * zero requests — the query is disabled, not fetched-then-discarded.
 */
export default function UserAvatar({ userId, name, version, size = "sm", full }: Props) {
  const q = useAvatarQuery(
    userId,
    version,
    full ? AvatarVariant.ORIGINAL : AvatarVariant.THUMB,
  );
  const src = q.data ?? "";

  return (
    <Avatar.Root size={size} colorPalette="blue" variant="solid">
      <Avatar.Fallback>{initials(name)}</Avatar.Fallback>
      {/* Chakra keeps the fallback visible until the image actually loads, so
          there is no flash of an empty circle while the bytes arrive. */}
      {src && <Avatar.Image src={src} alt={name} />}
    </Avatar.Root>
  );
}
