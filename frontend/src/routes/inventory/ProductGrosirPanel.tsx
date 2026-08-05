import { Fragment, useMemo, useState } from "react";
import {
  Badge,
  Box,
  Button,
  HStack,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { ProductPriceTier } from "../../gen/inventory_iface/v1/product_price_tier_pb";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import {
  groupTiersByUnit,
  useDeleteProductPriceTierMutation,
  useProductPriceTiersQuery,
} from "../../queries/productPriceTiers";
import { MarginCell } from "./productDetailCards";
import ProductPriceTierDrawer from "./ProductPriceTierDrawer";

// Per-product grosir (wholesale) ladder manager: the Product detail "Grosir" card,
// which sits beside the unit list. Grouped by unit because a tier prices ONE unit
// — a pcs ladder and a box ladder are independent, and a flat list would hide that.
export default function GrosirPanel({ product }: { product: Product }) {
  const { t } = useTranslation();
  const productId = product.id;
  const units = product.units;
  const page = usePageState(`priceTiers:${productId}`);
  const q = useProductPriceTiersQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
  });
  const del = useDeleteProductPriceTierMutation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<ProductPriceTier | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ProductPriceTier | null>(
    null,
  );
  // Grouping runs over THIS PAGE's rows, so a unit whose ladder straddles a page
  // boundary shows its header again on the next page. That's the honest read of
  // a paged list — the alternative (hiding the repeat) would make page 2 look
  // like it belonged to whichever unit came before it.
  const groups = useMemo(
    () => groupTiersByUnit(q.rows, units),
    [q.rows, units],
  );

  return (
    <Box>
      <HStack justify="space-between" mb={3} gap={4}>
        <Text fontSize="xs" color="fg.muted">
          {t("priceTiers.overridesDiscount")}
        </Text>
        <Button
          size="sm"
          colorPalette="blue"
          flexShrink={0}
          onClick={() => {
            setEditing(null);
            setDrawerOpen(true);
          }}
        >
          <Plus size={16} />
          {t("priceTiers.add")}
        </Button>
      </HStack>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("priceTiers.minQty")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("priceTiers.price")}
              </Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("inventory.products.marginCol")}
              </Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("priceTiers.saving")}
              </Table.ColumnHeader>
              <Table.ColumnHeader />
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {groups.map(({ unit, tiers }) => (
              <Fragment key={unit.id}>
                <Table.Row bg="bg.muted">
                  <Table.Cell colSpan={6} py={1}>
                    <HStack gap={2}>
                      <Text fontSize="xs" fontWeight="medium">
                        {unit.name}
                      </Text>
                      {!unit.isBase && (
                        <Text fontSize="xs" color="fg.muted">
                          ×{unit.factor.toString()}
                        </Text>
                      )}
                      <Text fontSize="xs" color="fg.muted">
                        {t("priceTiers.normalPrice")}{" "}
                        {formatMoney(unit.sellPrice)}
                      </Text>
                    </HStack>
                  </Table.Cell>
                </Table.Row>
                {tiers.map((tier, i) => {
                  const saving = unit.sellPrice - tier.price;
                  const pct =
                    unit.sellPrice > 0n
                      ? (Number(saving) * 100) / Number(unit.sellPrice)
                      : 0;
                  // A rung that isn't cheaper than the one below it never wins
                  // (POS takes the lowest qualifying price), so flag it rather
                  // than letting it look effective.
                  const notDescending =
                    i > 0 && tier.price >= tiers[i - 1].price;
                  return (
                    <Table.Row key={tier.id}>
                      <Table.Cell>
                        {t("priceTiers.rule", { count: tier.minQty })}
                      </Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(tier.price)}
                      </Table.Cell>
                      {/* Margin at the DISCOUNTED price — the number that says
                          whether this rung is still worth selling. */}
                      <Table.Cell textAlign="end">
                        <MarginCell
                          sell={tier.price}
                          cost={product.referenceCost * unit.factor}
                        />
                      </Table.Cell>
                      <Table.Cell
                        textAlign="end"
                        fontFamily="mono"
                        color={saving > 0n ? "green.fg" : "fg.muted"}
                      >
                        {saving > 0n
                          ? `${formatMoney(saving)} (${pct.toFixed(0)}%)`
                          : "—"}
                      </Table.Cell>
                      <Table.Cell>
                        <HStack gap={1}>
                          {saving <= 0n && (
                            <Badge colorPalette="red">
                              {t("priceTiers.notCheaper")}
                            </Badge>
                          )}
                          {notDescending && saving > 0n && (
                            <Badge colorPalette="orange">
                              {t("priceTiers.notDescending")}
                            </Badge>
                          )}
                        </HStack>
                      </Table.Cell>
                      <Table.Cell>
                        <HStack gap={1}>
                          <Button
                            size="xs"
                            variant="ghost"
                            onClick={() => {
                              setEditing(tier);
                              setDrawerOpen(true);
                            }}
                          >
                            <Pencil size={14} />
                          </Button>
                          <Button
                            size="xs"
                            variant="ghost"
                            colorPalette="red"
                            onClick={() => setPendingDelete(tier)}
                          >
                            <Trash2 size={14} />
                          </Button>
                        </HStack>
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
              </Fragment>
            ))}
            {groups.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={6}>
                  <Stack gap={1} py={4} align="center">
                    <Text color="fg.muted">{t("priceTiers.empty")}</Text>
                    <Text color="fg.muted" fontSize="xs">
                      {t("priceTiers.emptyHint")}
                    </Text>
                  </Stack>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      </TableScroll>

      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={q.total}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />

      <ProductPriceTierDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        productId={productId}
        editing={editing}
      />
      <ConfirmDialog
        open={pendingDelete != null}
        title={t("priceTiers.deleteTitle")}
        body={t("priceTiers.deleteBody")}
        confirmLabel={t("common.delete")}
        confirmColorPalette="red"
        loading={del.isPending}
        onConfirm={async () => {
          if (!pendingDelete) return;
          try {
            await del.mutateAsync(pendingDelete.id);
          } finally {
            setPendingDelete(null);
          }
        }}
        onCancel={() => setPendingDelete(null)}
      />
    </Box>
  );
}
