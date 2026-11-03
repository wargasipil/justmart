import { Box, Flex } from "@chakra-ui/react";
import { useCallback, useState } from "react";
import { Outlet } from "react-router-dom";

import BottomNav from "./BottomNav";
import Sidebar from "./Sidebar";
import MobileNavDrawer from "./sidebar/MobileNavDrawer";
import TopBar from "./TopBar";
import { usePreferencesStore } from "../stores/preferences";

// Two layouts from one shell, switched purely by CSS breakpoint:
//   md and up — the fixed rail (240px / 64px collapsed) + content offset by it.
//   below md  — an app frame: the column is exactly one screen tall
//               (100dvh), no TopBar (a phone gives the page that 56px),
//               <BottomNav> in flow at the bottom, and ONLY <main> scrolls
//               above it. Scrolling the document
//               instead ran the scrollbar down behind a fixed bottom bar and
//               let content slide under it. The bar's Menu slot opens the
//               complete nav in <MobileNavDrawer>, a bottom sheet.
//               The phone drawer's open/closed state is
//               deliberately NOT the persisted `sidebarCollapsed` flag, which
//               belongs to the desktop rail and must survive a phone visit.
export default function AppShell() {
  const collapsed = usePreferencesStore((s) => s.sidebarCollapsed);
  const sidebarWidth = collapsed ? "64px" : "240px";
  const [navOpen, setNavOpen] = useState(false);
  const closeNav = useCallback(() => setNavOpen(false), []);

  return (
    <Flex direction="row" minH={{ base: "100dvh", md: "100vh" }}>
      <Sidebar />
      <MobileNavDrawer open={navOpen} onClose={closeNav} />
      <Box
        flex="1"
        // A flex item defaults to min-width:auto, so this column refuses to
        // shrink below its content's min-content width — a wide table (or a
        // crowded TopBar) then pushes the whole document sideways, sliding the
        // sticky TopBar off-screen while the fixed Sidebar overlaps the content.
        // minW={0} lets it shrink; overflow is each surface's own business
        // (see <TableScroll>).
        minW={0}
        ml={{ base: 0, md: sidebarWidth }}
        transition="margin-left 150ms ease-out"
        display="flex"
        flexDirection="column"
        h={{ base: "100dvh", md: "auto" }}
      >
        {/* Desktop only — a phone reaches profile, preferences and the
            warehouse through the bottom bar's Profile tab instead. */}
        <Box display={{ base: "none", md: "block" }} position="sticky" top={0} zIndex={10}>
          <TopBar />
        </Box>
        <Box
          as="main"
          flex="1"
          // minH 0 lets the flex child shrink below its content so it can
          // scroll; on md+ the document scrolls as before.
          minH={0}
          overflowY={{ base: "auto", md: "visible" }}
          // The containing block for anything absolutely positioned inside a
          // page — notably Ark's visually-hidden native <select> behind every
          // Select. Without it that element is placed against <body>, so one
          // low on a long page stretched the DOCUMENT and a second scrollbar
          // appeared beside <main>'s.
          position="relative"
          px={{ base: 3, md: 6 }}
          py={4}
        >
          <Outlet />
        </Box>
        <BottomNav menuOpen={navOpen} onOpenMenu={() => setNavOpen(true)} />
      </Box>
    </Flex>
  );
}
