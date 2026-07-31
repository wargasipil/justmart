import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router-dom";

import PageHeader from "../components/PageHeader";

// Thin layout for the Inventaris sub-pages. Sub-navigation lives in the sidebar
// (the expandable "Inventaris" group), so there is no in-page tab strip, and the
// "you are here" trail is the TopBar breadcrumb — this just provides a
// consistent PageHeader. Products moved out to the top-level /products route.
export default function Inventory() {
  const { t } = useTranslation();

  return (
    <Box>
      <PageHeader title={t("inventory.title")} description={t("inventory.description")} />
      <Stack gap={4}>
        <Outlet />
      </Stack>
    </Box>
  );
}
