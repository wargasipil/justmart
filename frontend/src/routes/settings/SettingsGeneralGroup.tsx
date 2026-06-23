import { Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router-dom";

import RouteTabs from "../../components/RouteTabs";

// The "General" settings submenu: a tabbed page (like the Restock/Purchasing
// page) whose tabs are the core settings panels. The outer <Settings> shell
// provides the PageHeader; this only renders the tab strip + the active panel.
export default function SettingsGeneralGroup() {
  const { t } = useTranslation();
  const tabs = [
    { value: "general", to: "/settings/general", label: t("settings.tabs.general") },
    { value: "units", to: "/settings/general/units", label: t("settings.tabs.units") },
    { value: "printing", to: "/settings/general/printing", label: t("settings.tabs.printing") },
    { value: "payroll", to: "/settings/general/payroll", label: t("settings.tabs.payroll") },
    { value: "backups", to: "/settings/general/backups", label: t("settings.tabs.backups") },
  ];
  return (
    <Stack gap={4}>
      <RouteTabs items={tabs} />
      <Outlet />
    </Stack>
  );
}
