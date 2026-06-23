import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";
import RouteTabs from "../components/RouteTabs";

export default function Payroll() {
  const { t } = useTranslation();
  const location = useLocation();
  const tabs = [
    { value: "employees", to: "/payroll/employees", label: t("payroll.tabs.employees") },
    { value: "runs", to: "/payroll/runs", label: t("payroll.tabs.runs") },
  ];
  const activeKey = tabs.find((tab) => location.pathname.startsWith(tab.to))?.value ?? "employees";

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("nav.payroll") }, { label: t(`payroll.tabs.${activeKey}`) }]}
        title={t("payroll.title")}
      />
      <Stack gap={4}>
        <RouteTabs items={tabs} />
        <Outlet />
      </Stack>
    </Box>
  );
}
