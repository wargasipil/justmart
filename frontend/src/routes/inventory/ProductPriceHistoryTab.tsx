import { SegmentGroup, Stack, Table, Text } from "@chakra-ui/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { formatMoney, formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useProductUnitPricesQuery } from "../../queries/products";
import { useProductTierPricesQuery } from "../../queries/productPriceTiers";

type Scope = "unit" | "tier";

// Sell-price history for a product, in two flavours behind one segmented
// control: the per-unit catalog price (product_unit_prices) and the grosir
// ladder (product_tier_prices). They are separate tables with different keys —
// a rung is (unit, min_qty) — so they page independently rather than sharing one
// merged pager.
//
// Manager-only: both RPCs are OWNER+PHARMACIST, so this tab is only mounted when
// the caller may see it.
export default function ProductPriceHistoryTab({
  productId,
}: {
  productId: string;
}) {
  const { t } = useTranslation();
  const [scope, setScope] = useState<Scope>("unit");

  return (
    <Stack gap={3}>
      <SegmentGroup.Root
        size="sm"
        value={scope}
        onValueChange={(e) => setScope((e.value as Scope) ?? "unit")}
        alignSelf="flex-start"
      >
        <SegmentGroup.Indicator />
        <SegmentGroup.Item value="unit">
          <SegmentGroup.ItemText>
            {t("inventory.products.priceHistoryUnitScope")}
          </SegmentGroup.ItemText>
          <SegmentGroup.ItemHiddenInput />
        </SegmentGroup.Item>
        <SegmentGroup.Item value="tier">
          <SegmentGroup.ItemText>
            {t("inventory.products.priceHistoryTierScope")}
          </SegmentGroup.ItemText>
          <SegmentGroup.ItemHiddenInput />
        </SegmentGroup.Item>
      </SegmentGroup.Root>

      {/* Each panel owns its own page state, so switching scope doesn't carry a
          page offset over to a table with a different row count. */}
      {scope === "unit" ? (
        <UnitPriceHistory productId={productId} />
      ) : (
        <TierPriceHistory productId={productId} />
      )}
    </Stack>
  );
}

// Per-unit catalog sell price (one open row per unit; effectiveTo 0 = current).
function UnitPriceHistory({ productId }: { productId: string }) {
  const { t } = useTranslation();
  const page = usePageState(`unitPrices:${productId}`);
  const q = useProductUnitPricesQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
    enabled: !!productId,
  });

  return (
    <>
      {q.rows.length === 0 ? (
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.products.priceHistoryEmpty")}
        </Text>
      ) : (
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>
                  {t("inventory.products.priceHistoryUnitCol")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.priceFrom")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.priceTo")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.pricePrice")}
                </Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {q.rows.map((p) => (
                <Table.Row key={p.id}>
                  <Table.Cell>{p.unitName}</Table.Cell>
                  <Table.Cell>{formatUnix(p.effectiveFrom)}</Table.Cell>
                  <Table.Cell>
                    {p.effectiveTo > 0n
                      ? formatUnix(p.effectiveTo)
                      : t("inventory.products.priceCurrent")}
                  </Table.Cell>
                  <Table.Cell>{formatMoney(p.unitSellPrice)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      )}
      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={q.total}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />
    </>
  );
}

// Grosir ladder history. A row is one price of one RUNG ("buy >= minQty of this
// unit"), so a rung that was deleted still shows its prices — closed, never
// current.
function TierPriceHistory({ productId }: { productId: string }) {
  const { t } = useTranslation();
  const page = usePageState(`tierPrices:${productId}`);
  const q = useProductTierPricesQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
    enabled: !!productId,
  });

  return (
    <>
      {q.rows.length === 0 ? (
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.products.priceHistoryTierEmpty")}
        </Text>
      ) : (
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>
                  {t("inventory.products.priceHistoryUnitCol")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.tierMinQty")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.priceFrom")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.priceTo")}
                </Table.ColumnHeader>
                <Table.ColumnHeader>
                  {t("inventory.products.pricePrice")}
                </Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {q.rows.map((p) => (
                <Table.Row key={p.id}>
                  <Table.Cell>{p.unitName}</Table.Cell>
                  <Table.Cell textAlign="end">
                    {t("inventory.products.tierRung", { qty: p.minQty })}
                  </Table.Cell>
                  <Table.Cell>{formatUnix(p.effectiveFrom)}</Table.Cell>
                  <Table.Cell>
                    {p.effectiveTo > 0n
                      ? formatUnix(p.effectiveTo)
                      : t("inventory.products.priceCurrent")}
                  </Table.Cell>
                  <Table.Cell>{formatMoney(p.price)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      )}
      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={q.total}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />
    </>
  );
}
