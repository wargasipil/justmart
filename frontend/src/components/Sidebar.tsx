import { Box } from "@chakra-ui/react";

import { usePreferencesStore } from "../stores/preferences";
import SidebarNav from "./sidebar/SidebarNav";

// The desktop rail. Its parts live in ./sidebar/: the nav model (navItems.ts),
// the brand + links + footer shared with the phone drawer (SidebarNav.tsx),
// and the phone drawer itself (MobileNavDrawer.tsx).

/**
 * The desktop rail: fixed to the left edge, 240px or icon-only 64px.
 *
 * Hidden below `md` — a phone gets the same nav in <MobileNavDrawer> instead,
 * because even the 64px rail leaves a 375px screen too narrow for a table.
 */
export default function Sidebar() {
  const collapsed = usePreferencesStore((s) => s.sidebarCollapsed);
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
      display={{ base: "none", md: "flex" }}
      flexDirection="column"
    >
      <SidebarNav collapsed={collapsed} />
    </Box>
  );
}

