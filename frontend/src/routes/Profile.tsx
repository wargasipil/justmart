import { Box, Button, Stack, Tabs } from "@chakra-ui/react";
import { KeyRound } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import ChangePasswordDialog from "../components/ChangePasswordDialog";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../lib/auth";
import { useTabsOrientation } from "../lib/tabs";
import AccountSection from "./profile/AccountSection";
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
 * On `md`+ the sections sit behind a vertical tab rail. These are
 * state-driven tabs (`Tabs.Root` directly, per the Tabs HARD RULE) — /profile
 * is a single route with no sub-routes, so there is nothing to deep-link to.
 * On a phone there is no rail to put beside a panel, and a sideways-scrolling
 * strip hid the last tab off screen, so the sections simply stack: each panel
 * is already a titled <Section> card, so the page reads the same with or
 * without the tabs around it.
 */
export default function Profile() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [tab, setTab] = useState("account");
  const showWarehouse = useHasMultipleWarehouses();
  // Tab rail beside the panel on a wide viewport; stacked sections below.
  // Same breakpoint as /settings, which drives its own layout off this hook.
  const tabbed = useTabsOrientation() === "vertical";

  if (!user) return null;

  const tabs: { value: string; label: string; panel: ReactNode }[] = [
    {
      value: "account",
      label: t("profile.account"),
      panel: <AccountSection user={user} />,
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

      {tabbed ? (
        <Tabs.Root
          orientation="vertical"
          value={active}
          onValueChange={(d) => setTab(d.value)}
          variant="line"
          maxW="4xl"
        >
          <Tabs.List minW="180px" flexShrink={0}>
            {tabs.map((it) => (
              <Tabs.Trigger key={it.value} value={it.value} justifyContent="flex-start" w="full">
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
      ) : (
        <Stack gap={4}>
          {tabs.map((it) => (
            <Box key={it.value}>{it.panel}</Box>
          ))}
        </Stack>
      )}

      <ChangePasswordDialog open={passwordOpen} onClose={() => setPasswordOpen(false)} isSelf />
    </Box>
  );
}
