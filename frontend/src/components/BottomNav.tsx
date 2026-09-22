import { Box, Grid, Text } from "@chakra-ui/react";
import { House, Menu as MenuIcon, UserRound } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router-dom";

import { useBusinessMode } from "../queries/settings";
import { buildItems, findNavLeaf, type NavLeaf } from "./sidebar/navItems";

/** Height of the bar itself, excluding the device's safe-area inset. */
export const BOTTOM_NAV_HEIGHT = "64px";

const isProfile = (pathname: string) => pathname === "/profile" || pathname.startsWith("/profile/");

/**
 * What position 1 shows: the page you are on, taken from the same nav model
 * the drawer lists (so its label and icon match the drawer, Obat included in
 * pharmacy mode) — or Home on `/`, on `/profile` (Profile has its own tab and
 * never takes position 1) and on a route the nav doesn't list.
 */
export function firstSlot(pathname: string, entries: ReturnType<typeof buildItems>): NavLeaf | null {
  if (pathname === "/" || isProfile(pathname)) return null;
  const leaf = findNavLeaf(entries, pathname);
  return leaf && leaf.to !== "/" ? leaf : null;
}

/**
 * The phone nav: a fixed bottom bar with three slots — [Home | current page]
 * · Menu · Profile. Hidden from `md` up, where the Sidebar rail takes over.
 *
 * Position 1 is Home until you open a page from the drawer; then it becomes
 * that page (its icon + label, active). Menu and Profile never move: Menu is
 * active while its drawer is open, Profile on `/profile`. To get back Home,
 * open Menu → the first drawer entry (the dashboard).
 */
export default function BottomNav({
  menuOpen,
  onOpenMenu,
}: {
  menuOpen: boolean;
  onOpenMenu: () => void;
}) {
  const { t } = useTranslation();
  const { pathname } = useLocation();
  const { isPharmacy } = useBusinessMode();
  const page = firstSlot(pathname, buildItems(t, isPharmacy));
  const PageIcon = page?.icon;

  return (
    <Box
      as="nav"
      aria-label={t("nav.mobileNav")}
      colorPalette="blue"
      display={{ base: "block", md: "none" }}
      // In flow at the foot of AppShell's one-screen column (not fixed), so
      // the page's scroll area ends where the bar begins.
      flexShrink={0}
      bg="bg"
      borderTopWidth="1px"
      // Clear the iOS home indicator / Android gesture bar.
      pb="env(safe-area-inset-bottom)"
    >
      <Grid templateColumns="repeat(3, 1fr)" h={BOTTOM_NAV_HEIGHT}>
        {page && PageIcon ? (
          <TabLink to={page.to} active={!menuOpen} icon={<PageIcon size={22} />}>
            {page.label}
          </TabLink>
        ) : (
          <TabLink to="/" active={pathname === "/" && !menuOpen} icon={<House size={22} />}>
            {t("nav.home")}
          </TabLink>
        )}
        <Box
          as="button"
          onClick={onOpenMenu}
          aria-haspopup="dialog"
          aria-expanded={menuOpen}
          cursor="pointer"
          minW={0}
        >
          <TabBody active={menuOpen} icon={<MenuIcon size={22} />}>
            {t("nav.menu")}
          </TabBody>
        </Box>
        <TabLink to="/profile" active={isProfile(pathname) && !menuOpen} icon={<UserRound size={22} />}>
          {t("nav.profileShort")}
        </TabLink>
      </Grid>
    </Box>
  );
}

function TabLink({
  to,
  active,
  icon,
  children,
}: {
  to: string;
  active: boolean;
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <Link to={to} aria-current={active ? "page" : undefined} style={{ minWidth: 0 }}>
      <TabBody active={active} icon={icon}>
        {children}
      </TabBody>
    </Link>
  );
}

function TabBody({ active, icon, children }: { active: boolean; icon: ReactNode; children: ReactNode }) {
  return (
    <Box
      display="flex"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      gap={1}
      h="100%"
      px={1}
      color={active ? "colorPalette.solid" : "fg.muted"}
      _hover={{ bg: "bg.muted" }}
    >
      {icon}
      {/* A page label ("Riwayat order", "Stok opname") can outgrow a third of
          a phone; truncate rather than wrap the bar taller. */}
      <Text fontSize="xs" fontWeight={active ? "semibold" : "medium"} lineHeight="1" truncate maxW="100%">
        {children}
      </Text>
    </Box>
  );
}
