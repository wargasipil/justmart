import { Box, Stack } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router-dom";

import PageHeader from "../../components/PageHeader";
import RouteTabs from "../../components/RouteTabs";
import { POStatus } from "../../gen/purchasing_iface/v1/order_pb";
import RestockSummary from "./RestockSummary";
import { PO_STATUS_BY_TAB, RestockFiltersProvider } from "./restockFilters";

// One route per PO status (so the tabs deep-link), plus a "Pemasok" (suppliers
// ledger) tab. The detail + create pages are subpages that hide the tab strip.
const TAB_PATHS = [...Object.keys(PO_STATUS_BY_TAB), "suppliers"];

export default function Purchasing() {
  const { t } = useTranslation();
  const location = useLocation();

  // Subpage = /purchasing/new or /purchasing/<id> — anything single-segment
  // that isn't one of the known tab routes (which must keep showing the tabs).
  const seg = location.pathname.replace(/^\/purchasing\/?/, "").replace(/\/$/, "");
  const isSubpage = seg === "new" || (seg !== "" && !TAB_PATHS.includes(seg));

  // The status tab drives BOTH the stat row and the list, so it's derived once
  // here and handed down through the filter context (see restockFilters.tsx).
  const tab = seg === "" ? "all" : seg;
  const status = PO_STATUS_BY_TAB[tab] ?? POStatus.PO_STATUS_UNSPECIFIED;
  // The ledger tab has no order list, so nothing to summarize; subpages own
  // their own chrome.
  const showSummary = !isSubpage && tab in PO_STATUS_BY_TAB;

  const tabs = [
    { value: "all", to: "/purchasing/all", label: t("purchasing.statusTabs.all") },
    { value: "draft", to: "/purchasing/draft", label: t("purchasing.states.draft") },
    { value: "sent", to: "/purchasing/sent", label: t("purchasing.states.sent") },
    { value: "partial", to: "/purchasing/partial", label: t("purchasing.states.partiallyReceived") },
    { value: "received", to: "/purchasing/received", label: t("purchasing.states.received") },
    { value: "closed", to: "/purchasing/closed", label: t("purchasing.states.closed") },
    { value: "voided", to: "/purchasing/voided", label: t("purchasing.states.voided") },
    { value: "suppliers", to: "/purchasing/suppliers", label: t("purchasing.tabs.suppliersLedger") },
  ];

  return (
    <Box>
      <PageHeader
        title={t("purchasing.title")}
        description={t("purchasing.description")}
      />
      <RestockFiltersProvider status={status}>
        {/* minW=0: the tab content is a nested flex column, and a column flex
            item defaults to min-width:auto — without this the wide orders table
            grows this Stack instead of scrolling inside its own TableScroll,
            and the whole page scrolls sideways. */}
        <Stack gap={4} minW={0}>
          {showSummary && <RestockSummary />}
          {!isSubpage && <RouteTabs items={tabs} />}
          <Outlet />
        </Stack>
      </RestockFiltersProvider>
    </Box>
  );
}
