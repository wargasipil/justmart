import { Badge, Stack, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import type { User } from "../../gen/user_iface/v1/users_pb";
import { displayName, roleKey } from "../../lib/roles";
import AvatarPicker from "./AvatarPicker";
import { Section } from "./ProfileSections";

// The Akun card: the user's picture (the one thing they can change about their
// own row) beside their read-only name, role and email. Takes the user as a
// prop rather than calling useAuth() so it can be rendered on its own.
export default function AccountSection({ user }: { user: User }) {
  const { t } = useTranslation();
  const who = displayName(user);

  return (
    <Section title={t("profile.account")} help={t("profile.accountHelp")}>
      {/* Below md (where /profile drops its tab rail) the card is phone-width:
          beside the avatar the name column was ~140px, enough to cut an
          ordinary name to "Budi San…". So on a phone the avatar sits on top
          and the identity underneath gets the full width and WRAPS — who you
          are signed in as is the one thing this card must not hide. From md
          up it goes back to side by side, with the name truncating. */}
      <Stack
        direction={{ base: "column", md: "row" }}
        gap={4}
        align={{ base: "center", md: "flex-start" }}
        textAlign={{ base: "center", md: "start" }}
      >
        <AvatarPicker userId={user.id} name={who} version={Number(user.avatarUpdatedAt)} />
        <Stack gap={1} minW={0} w={{ base: "full", md: "auto" }} pt={{ md: 1 }} align={{ base: "center", md: "flex-start" }}>
          <Stack direction={{ base: "column", md: "row" }} gap={{ base: 1, md: 2 }} align="center" minW={0} maxW="full">
            <Text
              fontWeight="medium"
              fontSize={{ base: "lg", md: "md" }}
              truncate={{ base: false, md: true }}
              overflowWrap="anywhere"
              maxW="full"
            >
              {who}
            </Text>
            <Badge size="sm" colorPalette="blue" variant="subtle">
              {t(`dashboard.roles.${roleKey(user.role)}`)}
            </Badge>
          </Stack>
          <Text
            fontSize="sm"
            color="fg.muted"
            truncate={{ base: false, md: true }}
            overflowWrap="anywhere"
            maxW="full"
          >
            {user.email}
          </Text>
        </Stack>
      </Stack>
    </Section>
  );
}
