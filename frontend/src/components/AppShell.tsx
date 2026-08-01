import { Box, Flex } from "@chakra-ui/react";
import { Outlet } from "react-router-dom";

import Sidebar from "./Sidebar";
import TopBar from "./TopBar";
import { usePreferencesStore } from "../stores/preferences";

export default function AppShell() {
  const collapsed = usePreferencesStore((s) => s.sidebarCollapsed);
  const sidebarWidth = collapsed ? "64px" : "240px";

  return (
    <Flex direction="row" minH="100vh">
      <Sidebar />
      <Box
        flex="1"
        // A flex item defaults to min-width:auto, so this column refuses to
        // shrink below its content's min-content width — a wide table (or a
        // crowded TopBar) then pushes the whole document sideways, sliding the
        // sticky TopBar off-screen while the fixed Sidebar overlaps the content.
        // minW={0} lets it shrink; overflow is each surface's own business
        // (see <TableScroll>).
        minW={0}
        ml={sidebarWidth}
        transition="margin-left 150ms ease-out"
        display="flex"
        flexDirection="column"
      >
        <TopBar />
        <Box as="main" flex="1" px={6} py={4}>
          <Outlet />
        </Box>
      </Box>
    </Flex>
  );
}
