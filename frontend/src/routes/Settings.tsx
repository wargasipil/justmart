import { Box } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";

// Settings shell. The submenu navigation now lives in the sidebar (the expandable
// "Settings" group → General / License / Integrations), so this page only renders
// the shared header + the active sub-page. The General sub-page adds its own tab
// strip (SettingsGeneralGroup).
export default function Settings() {
  const { t } = useTranslation();
  const { pathname } = useLocation();

  const submenu = pathname.startsWith("/settings/license")
    ? "license"
    : pathname.startsWith("/settings/integrations")
      ? "integrations"
      : "general";

  return (
    <Box>
      <PageHeader
        breadcrumbs={[{ label: t("nav.settings") }, { label: t(`settings.groups.${submenu}`) }]}
        title={t("settings.title")}
      />
      <Outlet />
    </Box>
  );
}
