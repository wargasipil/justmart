import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  IconButton,
  Menu,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";

import Breadcrumbs from "./Breadcrumbs";
import ChangePasswordDialog from "./ChangePasswordDialog";
import UserAvatar from "./UserAvatar";
import LowStockBell from "./topbar/LowStockBell";
import WarehouseSelector from "./topbar/WarehouseSelector";
import { KeyRound, Languages, LogOut, Menu as MenuIcon, Moon, Sun, User as UserIcon } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import { Role } from "../gen/auth_iface/v1/policy_pb";
import { useAuth } from "../lib/auth";
import { displayName, roleKey } from "../lib/roles";
import { usePreferencesStore, type Locale } from "../stores/preferences";

export default function TopBar() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const theme = usePreferencesStore((s) => s.theme);
  const setTheme = usePreferencesStore((s) => s.setTheme);
  const locale = usePreferencesStore((s) => s.locale);
  const setLocale = usePreferencesStore((s) => s.setLocale);
  const toggleSidebar = usePreferencesStore((s) => s.toggleSidebar);

  const [passwordOpen, setPasswordOpen] = useState(false);

  const flipTheme = () => setTheme(theme === "dark" ? "light" : "dark");

  const flipLocale = () => {
    const next: Locale = locale === "id" ? "en" : "id";
    setLocale(next);
    void i18n.changeLanguage(next);
  };

  const who = user ? displayName(user) : "";
  const roleLabel = user ? t(`dashboard.roles.${roleKey(user.role)}`) : "";

  return (
    <Box
      as="header"
      position="sticky"
      top={0}
      zIndex={10}
      bg="bg"
      borderBottomWidth="1px"
      h="56px"
    >
      <Flex align="center" justify="space-between" h="100%" px={4}>
        {/* Left slot: hamburger + the app-wide "you are here" trail. The trail
            is derived from the URL (lib/breadcrumbs.ts) — pages never pass it. */}
        <HStack gap={2} minW={0} flex="1">
          <IconButton
            aria-label="toggle sidebar"
            variant="ghost"
            size="sm"
            onClick={toggleSidebar}
            display={{ base: "inline-flex", md: "none" }}
          >
            <MenuIcon size={18} />
          </IconButton>
          {user && <Breadcrumbs />}
        </HStack>

        <HStack gap={1} flexShrink={0}>
          {user && <WarehouseSelector />}
          {user && (user.role === Role.OWNER || user.role === Role.PHARMACIST) && (
            <LowStockBell isOwner={user.role === Role.OWNER} />
          )}
          {user && (
            <Menu.Root>
              <Menu.Trigger asChild>
                <Button
                  aria-label={t("nav.userMenu")}
                  variant="ghost"
                  size="sm"
                  h="40px"
                  px={2}
                  gap={2}
                >
                  <UserAvatar
                    userId={user.id}
                    name={who}
                    version={Number(user.avatarUpdatedAt)}
                    size="xs"
                  />
                  {/* Name + role read out loud on desktop; the avatar alone
                      carries the identity on narrow screens. */}
                  <Stack gap={0} textAlign="left" display={{ base: "none", md: "flex" }}>
                    <Text fontSize="xs" fontWeight="medium" lineHeight="1.2" maxW="160px" truncate>
                      {who}
                    </Text>
                    <Text fontSize="2xs" color="fg.muted" lineHeight="1.2">
                      {roleLabel}
                    </Text>
                  </Stack>
                </Button>
              </Menu.Trigger>
              <Portal>
                <Menu.Positioner>
                  <Menu.Content minW="220px">
                    <Menu.Item value="identity" disabled>
                      <Stack gap={0.5} w="100%" minW={0}>
                        <HStack justify="space-between" gap={2}>
                          <Text fontSize="sm" fontWeight="medium" truncate>
                            {who}
                          </Text>
                          <Badge size="sm" colorPalette="blue" variant="subtle" flexShrink={0}>
                            {roleLabel}
                          </Badge>
                        </HStack>
                        <Text fontSize="xs" color="fg.muted" truncate>
                          {user.email}
                        </Text>
                      </Stack>
                    </Menu.Item>
                    <Menu.Separator />
                    <Menu.Item value="profile" onClick={() => navigate("/profile")}>
                      <HStack gap={2}>
                        <UserIcon size={14} />
                        <Text fontSize="sm">{t("nav.profile")}</Text>
                      </HStack>
                    </Menu.Item>
                    <Menu.Separator />
                    {/* Preferences live here rather than as their own topbar
                        buttons — closeOnSelect={false} so the menu stays open
                        and the flip is visible immediately. */}
                    <Menu.Item value="language" closeOnSelect={false} onClick={flipLocale}>
                      <HStack justify="space-between" w="100%" gap={4}>
                        <HStack gap={2}>
                          <Languages size={14} />
                          <Text fontSize="sm">{t("nav.language")}</Text>
                        </HStack>
                        {/* Endonyms: a language is named in its own language,
                            so these are not translated. */}
                        <Text fontSize="xs" color="fg.muted">
                          {locale === "id" ? "Indonesia" : "English"}
                        </Text>
                      </HStack>
                    </Menu.Item>
                    <Menu.Item value="theme" closeOnSelect={false} onClick={flipTheme}>
                      <HStack justify="space-between" w="100%" gap={4}>
                        <HStack gap={2}>
                          {theme === "dark" ? <Moon size={14} /> : <Sun size={14} />}
                          <Text fontSize="sm">{t("nav.theme")}</Text>
                        </HStack>
                        <Text fontSize="xs" color="fg.muted">
                          {t(theme === "dark" ? "nav.themeDark" : "nav.themeLight")}
                        </Text>
                      </HStack>
                    </Menu.Item>
                    <Menu.Separator />
                    <Menu.Item value="changepw" onClick={() => setPasswordOpen(true)}>
                      <HStack gap={2}>
                        <KeyRound size={14} />
                        <Text fontSize="sm">{t("users.changeMyPassword")}</Text>
                      </HStack>
                    </Menu.Item>
                    <Menu.Item value="signout" onClick={logout}>
                      <HStack gap={2}>
                        <LogOut size={14} />
                        <Text fontSize="sm">{t("nav.signOut")}</Text>
                      </HStack>
                    </Menu.Item>
                  </Menu.Content>
                </Menu.Positioner>
              </Portal>
            </Menu.Root>
          )}
        </HStack>
      </Flex>
      <ChangePasswordDialog
        open={passwordOpen}
        onClose={() => setPasswordOpen(false)}
        isSelf
      />
    </Box>
  );
}
