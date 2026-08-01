import { Box, Flex } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";
import { useTabsOrientation } from "../lib/tabs";

export default function Settings() {
  const { t } = useTranslation();
  // Vertical rail on a wide viewport, horizontal strip below. The same value
  // drives the Flex direction, so the rail and the panel agree on which way
  // they are stacked without restating the breakpoint.
  const orientation = useTabsOrientation();
  const vertical = orientation === "vertical";

  const tabs = [
    { value: "general", to: "/settings/general", label: t("settings.tabs.general") },
    { value: "units", to: "/settings/units", label: t("settings.tabs.units") },
    { value: "printing", to: "/settings/printing", label: t("settings.tabs.printing") },
    { value: "backups", to: "/settings/backups", label: t("settings.tabs.backups") },
    { value: "updates", to: "/settings/updates", label: t("settings.tabs.updates") },
  ];

  return (
    <Box>
      <PageHeader
        title={t("settings.title")}
        description={t("settings.description")}
      />
      <Flex direction={vertical ? "row" : "column"} gap={vertical ? 6 : 4} align="stretch">
        <RouteTabs items={tabs} orientation={orientation} />
        {/* minW={0} lets the panel shrink below its content's min-content width
            — without it a wide table inside a tab pushes the rail off-layout
            (same reason AppShell's main column carries it). */}
        <Box flex="1" minW={0}>
          <Outlet />
        </Box>
      </Flex>
    </Box>
  );
}
