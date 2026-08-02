import { Box, HStack, IconButton, Spacer, Stack, Text } from "@chakra-ui/react";
import {
  ChevronDown,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  LogOut,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { NavLink, useLocation } from "react-router-dom";

import { useAuth } from "../lib/auth";
import { brandIcon } from "../lib/brand";
// WHAT the rail contains lives in lib/navItems.ts; this file is HOW it renders.
import {
  buildItems,
  isGroup,
  type ModeOnly,
  type NavGroup,
  type NavLeaf,
} from "../lib/navItems";
import { useAppTitle, useBusinessMode } from "../queries/settings";
import { usePreferencesStore } from "../stores/preferences";

export default function Sidebar() {
  const { t } = useTranslation();
  const collapsed = usePreferencesStore((s) => s.sidebarCollapsed);
  const toggle = usePreferencesStore((s) => s.toggleSidebar);
  const { user, logout } = useAuth();
  const { mode, isPharmacy, isRestaurant } = useBusinessMode();
  // Top-left brand = the app title configured in Settings ▸ General, falling
  // back to the built-in brand for the active mode.
  const brandName = useAppTitle();
  const BrandIcon = brandIcon(mode);

  const modeAllows = (entry: { onlyIn?: ModeOnly }) =>
    !entry.onlyIn ||
    (entry.onlyIn === "pharmacy" ? isPharmacy : isRestaurant);

  const items = buildItems(t, { isPharmacy, isRestaurant })
    .filter(
      (item) =>
        (!item.roles || (user && item.roles.includes(user.role))) &&
        modeAllows(item) &&
        (!("devOnly" in item && item.devOnly) || import.meta.env.DEV),
    )
    // A group's children carry their own mode gate (e.g. Tables is
    // restaurant-only inside the shared Order group), so filter them too.
    .map((item) =>
      isGroup(item) ? { ...item, children: item.children.filter(modeAllows) } : item,
    )
    .filter((item) => !isGroup(item) || item.children.length > 0);

  const width = collapsed ? "64px" : "240px";

  return (
    <Box
      as="aside"
      colorPalette="blue"
      width={width}
      bg="bg"
      borderRightWidth="1px"
      height="100vh"
      position="fixed"
      left={0}
      top={0}
      transition="width 150ms ease-out"
      display="flex"
      flexDirection="column"
    >
      {/* Brand */}
      <HStack gap={2} px={4} h="56px" borderBottomWidth="1px">
        <Box color="colorPalette.solid">
          <BrandIcon size={22} />
        </Box>
        {!collapsed && (
          <Text
            fontWeight="semibold"
            color="colorPalette.solid"
            fontSize="lg"
            truncate
            title={brandName}
          >
            {brandName}
          </Text>
        )}
      </HStack>

      {/* Nav items */}
      <Stack gap={1} px={2} py={3} flex="1" overflowY="auto">
        {items.map((item) =>
          isGroup(item) ? (
            <NavGroupItem key={item.label} group={item} collapsed={collapsed} />
          ) : (
            <NavItemLink key={item.to} item={item} collapsed={collapsed} />
          ),
        )}
      </Stack>

      {/* Footer: user + sign out + collapse */}
      <Stack gap={1} px={2} py={3} borderTopWidth="1px">
        {user && !collapsed && (
          <Box px={2} pb={2}>
            <Text fontSize="sm" fontWeight="medium">
              {user.name || user.email}
            </Text>
            <Text fontSize="xs" color="fg.muted">
              {user.email}
            </Text>
          </Box>
        )}
        {user && (
          <HStack
            as="button"
            gap={3}
            px={3}
            py={2}
            borderRadius="md"
            color="fg.muted"
            _hover={{ bg: "bg.muted" }}
            onClick={logout}
            cursor="pointer"
          >
            <LogOut size={18} />
            {!collapsed && <Text fontSize="sm">{t("nav.signOut")}</Text>}
          </HStack>
        )}
        <IconButton
          aria-label="toggle sidebar"
          variant="ghost"
          size="sm"
          onClick={toggle}
          alignSelf={collapsed ? "center" : "flex-end"}
        >
          {collapsed ? <ChevronsRight size={16} /> : <ChevronsLeft size={16} />}
        </IconButton>
      </Stack>
    </Box>
  );
}

function NavItemLink({ item, collapsed }: { item: NavLeaf; collapsed: boolean }) {
  const Icon = item.icon;
  return (
    <NavLink to={item.to} end={item.to === "/"}>
      {({ isActive }) => (
        <HStack
          gap={3}
          px={3}
          py={2}
          borderRadius="md"
          bg={isActive ? "bg.muted" : "transparent"}
          color={isActive ? "colorPalette.solid" : "fg"}
          borderLeftWidth={isActive ? "3px" : "0px"}
          borderLeftColor="colorPalette.solid"
          _hover={{ bg: "bg.muted" }}
          title={collapsed ? item.label : undefined}
        >
          <Icon size={18} />
          {!collapsed && <Text fontSize="sm">{item.label}</Text>}
        </HStack>
      )}
    </NavLink>
  );
}

// Expandable sidebar group (e.g. Inventaris). Auto-expands when any child route
// is active. When the rail is collapsed (icon-only) the children render as flat
// icon links so every destination stays reachable.
function NavGroupItem({ group, collapsed }: { group: NavGroup; collapsed: boolean }) {
  const { user } = useAuth();
  const location = useLocation();
  const Icon = group.icon;

  const children = group.children.filter(
    (c) => !c.roles || (user && c.roles.includes(user.role)),
  );
  const anyChildActive = children.some((c) => location.pathname.startsWith(c.to));
  const [open, setOpen] = useState(anyChildActive);

  useEffect(() => {
    if (anyChildActive) setOpen(true);
  }, [anyChildActive]);

  if (collapsed) {
    return (
      <>
        {children.map((c) => (
          <NavItemLink key={c.to} item={c} collapsed />
        ))}
      </>
    );
  }

  return (
    <Box>
      <HStack
        as="button"
        width="100%"
        gap={3}
        px={3}
        py={2}
        borderRadius="md"
        color={anyChildActive ? "colorPalette.solid" : "fg"}
        _hover={{ bg: "bg.muted" }}
        cursor="pointer"
        onClick={() => setOpen((o) => !o)}
      >
        <Icon size={18} />
        <Text fontSize="sm">{group.label}</Text>
        <Spacer />
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
      </HStack>
      {open && (
        <Stack gap={1} pl={4} mt={1}>
          {children.map((c) => (
            <NavItemLink key={c.to} item={c} collapsed={false} />
          ))}
        </Stack>
      )}
    </Box>
  );
}
