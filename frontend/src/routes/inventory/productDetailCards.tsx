import { type ReactNode } from "react";
import {
  Badge,
  Box,
  Heading,
  HStack,
  Stat,
  Table,
  Text,
} from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../../lib/format";
import { marginPct, marginValue } from "../../lib/pricing";

// The presentational vocabulary of the Product detail page: its card frame, the
// two field/tile primitives inside it, the margin cell, and the units table.
// Page-local by design — these are this page's chrome, not shared components, so
// they live next to the route rather than in components/ (which is the shared
// vocabulary surfaced by /components). MarginCell is here because BOTH the units
// table and the grosir panel render it.

// The page's card frame: a bg.subtle body under a bg.muted header bar. Same
// vocabulary as the tab strip below, whose Tabs.List *is* its header bar — so
// the caller owns the body padding (a table wants its own overflow container).
export function Card({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" overflow="hidden">
      <Box bg="bg.muted" px={4} py={2}>
        <Heading size="sm">{title}</Heading>
      </Box>
      {children}
    </Box>
  );
}

// Margin against the reference cost, as "value (pct%)" — the same money-then-
// percent shape the Grosir Saving column uses, so the two read alike. Percent
// alone hides how much a unit actually earns; value alone hides whether that's
// healthy. Negative goes red: selling under cost is a real problem, not a nuance.
// Renders "—" when the cost is unknown (no batch received yet).
export function MarginCell({ sell, cost }: { sell: bigint; cost: bigint }) {
  const pct = marginPct(sell, cost);
  const val = marginValue(sell, cost);
  if (pct == null || val == null) return <Text color="fg.muted">—</Text>;
  return (
    <Text color={val < 0n ? "red.fg" : "fg.muted"}>
      {formatMoney(val)} ({pct.toFixed(0)}%)
    </Text>
  );
}

// Units of measure (base + larger packs) with their selling price. A table
// rather than the old chip row: every unit carries the same four figures, so
// columns let you compare down them.
export function UnitsCard({
  product,
  showCost,
}: {
  product: Product;
  /** Off for the till: margin is cost-derived, and the grosir card it points at isn't rendered. */
  showCost: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Card title={t("inventory.products.unitsSection")}>
      <Box p={4} overflowX="auto">
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>
                  {t("inventory.products.priceHistoryUnitCol")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.unitFactor")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.pricePrice")}
                </Table.ColumnHeader>
                {showCost && (
                  <>
                    <Table.ColumnHeader textAlign="end">
                      {t("inventory.products.marginCol")}
                    </Table.ColumnHeader>
                    <Table.ColumnHeader textAlign="end">
                      {t("priceTiers.section")}
                    </Table.ColumnHeader>
                  </>
                )}
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {product.units.map((u) => {
                // Surface the grosir ladder where prices already live, so the card
                // below is discoverable from the unit it applies to.
                const tierCount = product.priceTiers.filter(
                  (pt) => pt.productUnitId === u.id,
                ).length;
                return (
                  <Table.Row key={u.id}>
                    <Table.Cell>
                      <HStack gap={2}>
                        <Text fontWeight="medium">{u.name}</Text>
                        {u.isBase && (
                          <Badge size="sm" colorPalette="blue">
                            {t("inventory.products.baseUnit")}
                          </Badge>
                        )}
                      </HStack>
                    </Table.Cell>
                    <Table.Cell textAlign="end" color="fg.muted">
                      ×{u.factor.toString()}
                    </Table.Cell>
                    <Table.Cell textAlign="end" fontFamily="mono">
                      {formatMoney(u.sellPrice)}
                    </Table.Cell>
                    {showCost && (
                      <>
                        <Table.Cell textAlign="end">
                          <MarginCell
                            sell={u.sellPrice}
                            cost={product.referenceCost * u.factor}
                          />
                        </Table.Cell>
                        <Table.Cell textAlign="end">
                          {tierCount > 0 ? (
                            <Text color="purple.fg">
                              {t("priceTiers.unitTierCount", {
                                count: tierCount,
                              })}
                            </Text>
                          ) : (
                            <Text color="fg.muted">—</Text>
                          )}
                        </Table.Cell>
                      </>
                    )}
                  </Table.Row>
                );
              })}
              {product.units.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={showCost ? 5 : 3}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      {t("common.noResults")}
                    </Text>
                  </Table.Cell>
                </Table.Row>
              )}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      </Box>
    </Card>
  );
}

export function Field({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily={mono ? "mono" : undefined}>{value}</Text>
    </Box>
  );
}

export function Tile({
  label,
  value,
  sub,
  palette,
}: {
  label: string;
  value: string;
  /** Secondary figure under the count — the tile's valuation at cost. */
  sub?: string;
  /**
   * Chakra colorPalette, pinned per metric to match the analytics charts
   * (CHART_SERIES: ready = blue, ongoing = orange). Colour belongs to the
   * metric, so a tile keeps its hue across pages — don't reassign per layout.
   */
  palette: "blue" | "orange";
}) {
  return (
    // A tinted surface, not bg.muted: these two tiles sit ON a bg.subtle card,
    // and the palette is what distinguishes on-hand from incoming at a glance.
    // colorPalette.* are semantic tokens, so both hues flip with the theme.
    // The card chrome is ours; the label/value/sub inside are Chakra's Stat, so
    // a metric renders as the <dl>/<dt>/<dd> it actually is.
    <Box
      colorPalette={palette}
      bg="colorPalette.subtle"
      borderWidth="1px"
      borderColor="colorPalette.muted"
      borderRadius="lg"
      px={3}
      py={2}
    >
      <Stat.Root size="sm">
        <Stat.Label fontSize="xs" color="fg.muted" mb={1}>
          {label}
        </Stat.Label>
        <Stat.ValueText
          fontSize="lg"
          fontWeight="semibold"
          color="colorPalette.fg"
        >
          {value}
        </Stat.ValueText>
        {sub && (
          <Stat.HelpText fontSize="xs" color="fg.muted" mb={0}>
            {sub}
          </Stat.HelpText>
        )}
      </Stat.Root>
    </Box>
  );
}
