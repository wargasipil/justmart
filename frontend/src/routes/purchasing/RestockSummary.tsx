import { SimpleGrid } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import SummaryTile from "../../components/SummaryTile";
import { formatMoney, formatThousands } from "../../lib/format";
import { usePurchaseOrdersSummaryQuery } from "../../queries/purchasing";
import { useRestockFilters } from "./restockFilters";

/**
 * The stat row above the restock tabs: how many restock orders match the active
 * filters, how many distinct products they cover, how much was ordered, and
 * what it comes to.
 *
 * Four carded <SummaryTile>s (Chakra Stat on the app's panel surface) in a
 * 4-column grid — the same vocabulary as the summary rows on /orders and
 * /products, so the three list pages read alike. Two columns on narrow screens
 * so the labels never truncate.
 *
 * Server-side aggregate over EVERY matching order (GetPurchaseOrdersSummary),
 * not a sum of the page below — otherwise the figures would shift as the user
 * pages. It sends the same `request` object the list sends, so the row and the
 * table can never describe different sets.
 *
 * The status tab is part of that request, so each tab summarizes itself.
 */
export default function RestockSummary() {
  const { t } = useTranslation();
  const { request } = useRestockFilters();
  const summaryQ = usePurchaseOrdersSummaryQuery(request);
  const summary = summaryQ.data;

  const tiles = [
    { key: "orders", value: formatThousands(summary?.orderCount ?? 0n) },
    { key: "products", value: formatThousands(summary?.productCount ?? 0n) },
    { key: "items", value: formatThousands(summary?.itemCount ?? 0n) },
    { key: "total", value: formatMoney(summary?.total ?? 0n) },
  ];

  return (
    <SimpleGrid columns={{ base: 2, md: 4 }} gap={{ base: 3, md: 4 }}>
      {tiles.map((tile) => (
        <SummaryTile
          key={tile.key}
          label={t(`purchasing.summary.${tile.key}`)}
          value={tile.value}
        />
      ))}
    </SimpleGrid>
  );
}
