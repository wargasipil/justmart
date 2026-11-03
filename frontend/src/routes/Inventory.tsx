import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../components/PageHeader";

// Thin layout for the Inventaris sub-pages. Sub-navigation lives in the sidebar
// (the expandable "Inventaris" group), so there is no in-page tab strip, and the
// "you are here" trail is the TopBar breadcrumb — this just provides a
// consistent PageHeader. Products moved out to the top-level /products route.
export default function Inventory() {
  const { t } = useTranslation();
  const location = useLocation();

  // Subpage = a third segment, i.e. /inventory/<section>/<id|new> — a detail or
  // create page that brings its own BackButton + PageHeader. The section header
  // is suppressed there: stacked above them it pushed the back button down into
  // the gap BETWEEN two titles (115px into a phone screen, under the divider
  // that closes this header), where it reads as belonging to neither. A detail
  // page now opens with its back affordance at the top, like /products/:id,
  // which has no layout header above it. The "you are here" trail is the TopBar
  // breadcrumb on a desk; on a phone the back button IS it.
  const isSubpage = location.pathname.split("/").filter(Boolean).length > 2;

  return (
    <Box>
      {!isSubpage && (
        <PageHeader title={t("inventory.title")} description={t("inventory.description")} />
      )}
      <Stack gap={4}>
        <Outlet />
      </Stack>
    </Box>
  );
}
