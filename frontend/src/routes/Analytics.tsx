import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";

export default function Analytics() {
  const { t } = useTranslation();
  const tabs = [
    { value: "daily", to: "/analytics/daily", label: t("analytics.menu.daily") },
    { value: "product", to: "/analytics/product", label: t("analytics.menu.product") },
    { value: "user", to: "/analytics/user", label: t("analytics.menu.user") },
  ];

  return (
    <Box>
      <PageHeader
        title={t("analytics.title")}
        description={t("analytics.description")}
      />
      <Stack gap={4}>
        <RouteTabs items={tabs} />
        <Outlet />
      </Stack>
    </Box>
  );
}
