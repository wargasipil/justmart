import { Box, Drawer, IconButton, Portal } from "@chakra-ui/react";
import { X } from "lucide-react";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";

import SidebarNav from "./SidebarNav";

/**
 * The phone's full nav: the rail's contents in a bottom sheet, opened by the
 * Menu tab of <BottomNav> — it rises from the bar the thumb just tapped.
 * Always expanded (a drawer has the room), no collapse toggle, and it closes itself on navigation so a tap on a link lands you on
 * the page rather than behind the drawer.
 *
 * Stays mounted and is driven by `open` — the Dialog-mount HARD RULE applies
 * to Drawer too (it's the same Ark dialog underneath).
 */
export default function MobileNavDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const location = useLocation();

  // Close on any route change. onClose is deliberately not a dependency: the
  // parent passes a fresh closure every render, which would close the drawer
  // the moment it opened.
  useEffect(() => {
    onClose();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname]);

  return (
    <Drawer.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      placement="bottom"
    >
      <Portal>
        <Drawer.Backdrop />
        <Drawer.Positioner>
          {/* Capped below full height so the page stays visible behind it and
              a tap on the backdrop is always possible; the nav list scrolls. */}
          <Drawer.Content
            colorPalette="blue"
            maxH="85dvh"
            roundedTop="xl"
            pb="env(safe-area-inset-bottom)"
          >
            <Drawer.CloseTrigger asChild top="3" insetEnd="2">
              <IconButton aria-label={t("common.close")} variant="ghost" size="sm">
                <X size={18} />
              </IconButton>
            </Drawer.CloseTrigger>
            <Box display="flex" flexDirection="column" flex="1" minH={0}>
              <SidebarNav collapsed={false} inDrawer />
            </Box>
          </Drawer.Content>
        </Drawer.Positioner>
      </Portal>
    </Drawer.Root>
  );
}

