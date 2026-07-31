import { Badge, Box, Button, HStack, Stack, Tabs, Text } from "@chakra-ui/react";
import { KeyRound } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import ChangePasswordDialog from "../components/ChangePasswordDialog";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../lib/auth";
import { displayName, roleKey } from "../lib/roles";
import { useTabsOrientation } from "../lib/tabs";
import AvatarPicker from "./profile/AvatarPicker";
import {
  DefaultWarehouseSection,
  PreferencesSection,
  Section,
  useHasMultipleWarehouses,
} from "./profile/ProfileSections";

/**
 * /profile — the signed-in user's own account page, reachable from the TopBar
 * avatar menu. Open to every authenticated role, which is why it is a
 * top-level route and NOT a /settings tab (/settings is OWNER-gated, so a
 * cashier clicking "My profile" would hit a permission wall).
 *
 * Identity is read-only: the backend has no self-service name/email RPC
 * (CreateUser / UpdateUserRole / SetUserActive are all OWNER-only), so the
 * page says who to ask rather than rendering inputs that cannot save. The
 * profile picture is the one thing a user can change about their own row.
 *
 * The sections sit behind a vertical tab rail rather than stacking. These are
 * state-driven tabs (`Tabs.Root` directly, per the Tabs HARD RULE) — /profile
 * is a single route with no sub-routes, so there is nothing to deep-link to.
 */
export default function Profile() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [tab, setTab] = useState("account");
  const showWarehouse = useHasMultipleWarehouses();
  // Rail beside the panel on a wide viewport, plain strip below — shared with
  // /settings, which drives its own layout off the same hook.
  const orientation = useTabsOrientation();
  const vertical = orientation === "vertical";

  if (!user) return null;

  const who = displayName(user);
  const roleLabel = t(`dashboard.roles.${roleKey(user.role)}`);

  const tabs: { value: string; label: string; panel: ReactNode }[] = [
    {
      value: "account",
      label: t("profile.account"),
      panel: (
        <Section title={t("profile.account")} help={t("profile.accountHelp")}>
          <HStack gap={4} align="flex-start">
            <AvatarPicker userId={user.id} name={who} version={Number(user.avatarUpdatedAt)} />
            <Stack gap={1} minW={0} pt={1}>
              <HStack gap={2}>
                <Text fontWeight="medium" truncate>
                  {who}
                </Text>
                <Badge size="sm" colorPalette="blue" variant="subtle">
                  {roleLabel}
                </Badge>
              </HStack>
              <Text fontSize="sm" color="fg.muted" truncate>
                {user.email}
              </Text>
            </Stack>
          </HStack>
        </Section>
      ),
    },
    {
      value: "preferences",
      label: t("profile.preferences"),
      panel: <PreferencesSection />,
    },
    ...(showWarehouse
      ? [
          {
            value: "warehouse",
            label: t("profile.defaultWarehouse"),
            panel: <DefaultWarehouseSection />,
          },
        ]
      : []),
    {
      value: "security",
      label: t("profile.security"),
      panel: (
        <Section title={t("profile.security")} help={t("profile.securityHelp")}>
          <Button variant="outline" size="sm" onClick={() => setPasswordOpen(true)}>
            <KeyRound size={14} />
            {t("users.changeMyPassword")}
          </Button>
        </Section>
      ),
    },
  ];

  // The warehouse tab appears only once the membership query resolves, and can
  // vanish again if access is revoked mid-session. Fall back to the first tab
  // instead of leaving the selection pointing at a value no trigger owns.
  const active = tabs.some((it) => it.value === tab) ? tab : tabs[0].value;

  return (
    <Box>
      <PageHeader title={t("profile.title")} description={t("profile.subtitle")} />

      <Tabs.Root
        orientation={orientation}
        value={active}
        onValueChange={(d) => setTab(d.value)}
        variant="line"
        maxW="4xl"
      >
        {/* Same sizing rules as <RouteTabs>: triggers never shrink (squeezing
            four labels overlaps them — "Default warehouse" alone is wide) and
            a degraded strip scrolls instead. */}
        <Tabs.List
          minW={vertical ? "180px" : undefined}
          flexShrink={0}
          overflowX={vertical ? undefined : "auto"}
        >
          {tabs.map((it) => (
            <Tabs.Trigger
              key={it.value}
              value={it.value}
              flexShrink={0}
              justifyContent={vertical ? "flex-start" : undefined}
              w={vertical ? "full" : undefined}
            >
              {it.label}
            </Tabs.Trigger>
          ))}
        </Tabs.List>
        {tabs.map((it) => (
          <Tabs.Content key={it.value} value={it.value} flex="1" minW={0}>
            {it.panel}
          </Tabs.Content>
        ))}
      </Tabs.Root>

      <ChangePasswordDialog open={passwordOpen} onClose={() => setPasswordOpen(false)} isSelf />
    </Box>
  );
}
