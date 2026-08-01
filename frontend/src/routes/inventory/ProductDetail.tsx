import { Fragment, useMemo, useState, type ReactNode } from "react";
import {
  Badge,
  Box,
  Button,
  Heading,
  HStack,
  SimpleGrid,
  Spinner,
  Stack,
  Table,
  Tabs,
  Text,
} from "@chakra-ui/react";
import { Archive, ArchiveRestore, Pencil, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useCrumbLabel } from "../../lib/breadcrumbs";
import BackButton from "../../components/BackButton";
import ProductImagePicker from "./ProductImagePicker";
import ConfirmDialog from "../../components/ConfirmDialog";
import ExpiryBadge from "../../components/ExpiryBadge";
import PageHeader from "../../components/PageHeader";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { formatDiscount, formatMoney, formatUnix } from "../../lib/format";
import { marginPct, marginValue } from "../../lib/pricing";
import { usePageState } from "../../lib/pagination";
import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { toast } from "../../lib/toaster";
import { useBatchesQuery } from "../../queries/batches";
import {
  useArchiveProductMutation,
  useProductRestockLogsQuery,
  useProductUnitPricesQuery,
  useProductQuery,
  useUnarchiveProductMutation,
} from "../../queries/products";
import { useSupplierRefs } from "../../queries/refs";
import { useMovementsQuery } from "../../queries/stock";
import { ProductDiscount } from "../../gen/inventory_iface/v1/product_discount_pb";
import {
  formatDiscountValue,
  useDeleteProductDiscountMutation,
  useProductDiscountsQuery,
} from "../../queries/productDiscounts";
import { ProductPriceTier } from "../../gen/inventory_iface/v1/product_price_tier_pb";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import {
  groupTiersByUnit,
  useDeleteProductPriceTierMutation,
  useProductPriceTiersQuery,
} from "../../queries/productPriceTiers";
import { EditProductDialog } from "./productDrawers";
import ProductDiscountDrawer from "./ProductDiscountDrawer";
import ProductPriceTierDrawer from "./ProductPriceTierDrawer";

function fmtVariance(v: bigint): string {
  if (v === 0n) return "±0";
  return v > 0n ? `+${v.toString()}` : v.toString();
}

function movementTypeKey(type: MovementType): string {
  switch (type) {
    case MovementType.PURCHASE:
      return "purchase";
    case MovementType.SALE:
      return "sale";
    case MovementType.ADJUSTMENT:
      return "adjustment";
    case MovementType.WRITE_OFF:
      return "writeOff";
    default:
      return "unspecified";
  }
}

export default function ProductDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const [editing, setEditing] = useState(false);
  const [pendingArchive, setPendingArchive] = useState(false);
  const [pendingUnarchive, setPendingUnarchive] = useState(false);

  const medQ = useProductQuery(id);
  useCrumbLabel(medQ.data?.name);
  const archive = useArchiveProductMutation();
  const unarchive = useUnarchiveProductMutation();
  // Each tab owns its own page state, keyed by product id so navigating to a
  // different product resets every pager rather than landing on page 3 of a
  // shorter list.
  const pricesPage = usePageState(`unitPrices:${id}`);
  const batchesPage = usePageState(`batches:${id}`);
  const movementsPage = usePageState(`movements:${id}`);
  const restockPage = usePageState(`restocks:${id}`);

  const unitPricesQ = useProductUnitPricesQuery(id, {
    page: pricesPage.page,
    pageSize: pricesPage.pageSize,
    enabled: !!id,
  });
  const batchesQ = useBatchesQuery({
    productId: id,
    onlyInStock: true,
    page: batchesPage.page,
    pageSize: batchesPage.pageSize,
  });
  const batchSupplierRefs = useSupplierRefs(
    useMemo(
      () =>
        Array.from(
          new Set(
            (batchesQ.rows ?? [])
              .map((b) => b.supplierId)
              .filter((s): s is string => !!s),
          ),
        ),
      [batchesQ.rows],
    ),
  );
  const movementsQ = useMovementsQuery({
    productId: id,
    page: movementsPage.page,
    pageSize: movementsPage.pageSize,
  });
  const restockQ = useProductRestockLogsQuery(id, {
    page: restockPage.page,
    pageSize: restockPage.pageSize,
    enabled: !!id,
  });
  const restockSupplierRefs = useSupplierRefs(
    useMemo(
      () =>
        Array.from(
          new Set(
            (restockQ.rows ?? []).map((r) => r.supplierId).filter(Boolean),
          ),
        ),
      [restockQ.rows],
    ),
  );

  if (medQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const med = medQ.data;
  if (!med) {
    return (
      <Box p={8}>
        <Text color="fg.muted">{t("common.noResults")}</Text>
      </Box>
    );
  }

  const onArchive = async () => {
    try {
      await archive.mutateAsync({ id: med.id });
      toast.success(t("common.archive") + " ✓");
      setPendingArchive(false);
      navigate("/products");
    } catch {
      /* toast handled globally */
    }
  };

  const onUnarchive = async () => {
    try {
      await unarchive.mutateAsync({ id: med.id });
      toast.success(t("inventory.products.unarchive") + " ✓");
      setPendingUnarchive(false);
      navigate("/products");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <BackButton to="/products" />
      <PageHeader
        title={med.name}
        description={t("inventory.products.detailDescription")}
        titleBadge={
          <Badge colorPalette={med.active ? "green" : "gray"}>
            {med.active ? t("common.active") : t("common.inactive")}
          </Badge>
        }
        actions={
          <HStack>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setEditing(true)}
            >
              <Pencil size={14} />
              {t("common.edit")}
            </Button>
            {med.active ? (
              <Button
                size="sm"
                variant="outline"
                colorPalette="red"
                onClick={() => setPendingArchive(true)}
              >
                <Archive size={14} />
                {t("common.archive")}
              </Button>
            ) : (
              <Button
                size="sm"
                variant="outline"
                colorPalette="green"
                onClick={() => setPendingUnarchive(true)}
              >
                <ArchiveRestore size={14} />
                {t("inventory.products.unarchive")}
              </Button>
            )}
          </HStack>
        }
      />

      <Stack gap={6}>
        {/* Info and unit pricing sit side by side: they answer two different
            questions ("what/how much do I hold" vs "what do I sell it for"),
            and pairing them keeps both above the fold. Stacks on narrow. */}
        <SimpleGrid columns={{ base: 1, lg: 2 }} gap={6} alignItems="start">
          {/* Left column = what this product IS: the photo identifies it at a
              glance, then the facts. Card vocabulary shared with the tab strip
              below: a bg.subtle body under a bg.muted header bar. */}
          <Stack gap={6}>
            <Card title={t("inventory.products.imageSection")}>
              <Box p={4}>
                <ProductImagePicker
                  productId={med.id}
                  name={med.name}
                  version={Number(med.imageUpdatedAt)}
                />
              </Box>
            </Card>
            <Card title={t("inventory.products.infoSection")}>
              <Box p={4}>
                <SimpleGrid columns={{ base: 2, xl: 3 }} gap={3}>
                  <Field
                    label={t("inventory.products.sku")}
                    value={med.sku}
                    mono
                  />
                  <Field
                    label={t("inventory.products.unit")}
                    value={med.unit}
                  />
                  <Field
                    label={t("inventory.products.unitPrice")}
                    value={formatMoney(med.unitPrice)}
                  />
                  <Field
                    label={t("inventory.products.lastCost")}
                    value={
                      med.referenceCost > 0n
                        ? formatMoney(med.referenceCost)
                        : "—"
                    }
                  />
                  <Field
                    label={t("inventory.products.lastRestock")}
                    value={
                      med.lastRestockDate
                        ? med.lastRestockDate +
                          (med.lastRestockSupplier
                            ? ` · ${med.lastRestockSupplier}`
                            : "")
                        : "—"
                    }
                  />
                  <Field
                    label={t("inventory.products.lastStocktake")}
                    value={
                      med.lastStocktakeDate
                        ? `${med.lastStocktakeDate} · ${fmtVariance(med.lastStocktakeVariance)}`
                        : "—"
                    }
                  />
                  {/* Each stock tile carries its own valuation, so "how much do I
                    hold / have coming" reads as one figure pair instead of a
                    count here and a lone total elsewhere. */}
                  <Tile
                    label={t("inventory.products.readyStock")}
                    value={med.readyStock.toString()}
                    sub={formatMoney(med.stockValuation)}
                    palette="blue"
                  />
                  <Tile
                    label={t("inventory.products.onOrder")}
                    value={
                      med.onOrderStock > 0n ? med.onOrderStock.toString() : "—"
                    }
                    sub={
                      med.onOrderStock > 0n
                        ? formatMoney(med.onOrderValuation)
                        : undefined
                    }
                    palette="orange"
                  />
                </SimpleGrid>
              </Box>
            </Card>
          </Stack>

          {/* Right column = the two pricing cards, stacked. Grosir follows
              Satuan because a tier prices ONE unit: the ladder is only readable
              directly under the unit list it keys off. */}
          <Stack gap={6}>
            <UnitsCard product={med} />
            <Card title={t("priceTiers.section")}>
              <Box p={4}>
                <GrosirPanel product={med} />
              </Box>
            </Card>
          </Stack>
        </SimpleGrid>

        {/* Batches / Price history / Movements as tabs. The card wraps the whole
            strip so the triggers and the active panel read as one surface — the
            panel tables therefore drop their own bg/border (they'd be a card in
            a card) and each scrolls in its own container, since the card clips. */}
        <Box
          bg="bg.subtle"
          borderWidth="1px"
          borderRadius="lg"
          overflow="hidden"
        >
          <Tabs.Root defaultValue="batches" variant="line">
            <Tabs.List bg="bg.muted" px={2}>
              <Tabs.Trigger value="batches">
                {t("inventory.products.batchesSection")}
              </Tabs.Trigger>
              <Tabs.Trigger value="prices">
                {t("inventory.products.soldPriceHistory")}
              </Tabs.Trigger>
              <Tabs.Trigger value="restocks">
                {t("inventory.products.restockPriceHistory")}
              </Tabs.Trigger>
              <Tabs.Trigger value="movements">
                {t("inventory.products.movementsSection")}
              </Tabs.Trigger>
              <Tabs.Trigger value="discount">
                {t("productDiscounts.section")}
              </Tabs.Trigger>
            </Tabs.List>

            <Tabs.Content value="batches" p={4}>
              <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                <Table.Root size="sm" stickyHeader>
                  <Table.Header bg="bg.muted">
                    <Table.Row>
                      <Table.ColumnHeader>
                        {t("inventory.batches.batchNumber")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader>
                        {t("inventory.batches.supplier")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader>
                        {t("inventory.batches.expiry")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader>
                        {t("inventory.batches.cost")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">
                        {t("inventory.batches.qty")}
                      </Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {batchesQ.rows.map((b) => (
                      <Table.Row key={b.id}>
                        <Table.Cell>{b.batchNumber || "—"}</Table.Cell>
                        <Table.Cell>
                          {b.supplierId
                            ? (() => {
                                const s = batchSupplierRefs.get(b.supplierId);
                                return s ? `${s.code} · ${s.name}` : "—";
                              })()
                            : "—"}
                        </Table.Cell>
                        <Table.Cell>
                          <HStack gap={2}>
                            <Text>{b.expiryDate}</Text>
                            <ExpiryBadge expiry={b.expiryDate} />
                          </HStack>
                        </Table.Cell>
                        <Table.Cell>{formatMoney(b.costPrice)}</Table.Cell>
                        <Table.Cell textAlign="end">
                          {b.currentQuantity.toString()}
                        </Table.Cell>
                      </Table.Row>
                    ))}
                    {batchesQ.rows.length === 0 && (
                      <Table.Row>
                        <Table.Cell colSpan={5}>
                          <Text color="fg.muted" textAlign="center" py={4}>
                            {t("common.noResults")}
                          </Text>
                        </Table.Cell>
                      </Table.Row>
                    )}
                  </Table.Body>
                </Table.Root>
              </TableScroll>
              <Pagination
                page={batchesPage.page}
                pageSize={batchesPage.pageSize}
                total={batchesQ.total}
                onPageChange={batchesPage.setPage}
                onPageSizeChange={batchesPage.setPageSize}
              />
            </Tabs.Content>

            <Tabs.Content value="prices" p={4}>
              {unitPricesQ.rows.length === 0 ? (
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
                      {unitPricesQ.rows.map((p) => (
                        <Table.Row key={p.id}>
                          <Table.Cell>{p.unitName}</Table.Cell>
                          <Table.Cell>{formatUnix(p.effectiveFrom)}</Table.Cell>
                          <Table.Cell>
                            {p.effectiveTo > 0n
                              ? formatUnix(p.effectiveTo)
                              : t("inventory.products.priceCurrent")}
                          </Table.Cell>
                          <Table.Cell>
                            {formatMoney(p.unitSellPrice)}
                          </Table.Cell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </Table.Root>
                </TableScroll>
              )}
              <Pagination
                page={pricesPage.page}
                pageSize={pricesPage.pageSize}
                total={unitPricesQ.total}
                onPageChange={pricesPage.setPage}
                onPageSizeChange={pricesPage.setPageSize}
              />
            </Tabs.Content>

            <Tabs.Content value="restocks" p={4}>
              {restockQ.rows.length === 0 ? (
                <Text fontSize="sm" color="fg.muted">
                  {t("inventory.products.restockHistoryEmpty")}
                </Text>
              ) : (
                <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                  <Table.Root size="sm" stickyHeader>
                    <Table.Header bg="bg.muted">
                      <Table.Row>
                        <Table.ColumnHeader>
                          {t("inventory.products.restockSupplier")}
                        </Table.ColumnHeader>
                        <Table.ColumnHeader textAlign="end">
                          {t("inventory.products.restockPrice")}
                        </Table.ColumnHeader>
                        <Table.ColumnHeader textAlign="end">
                          {t("inventory.products.restockQty")}
                        </Table.ColumnHeader>
                        <Table.ColumnHeader textAlign="end">
                          {t("inventory.products.restockDiscount")}
                        </Table.ColumnHeader>
                        <Table.ColumnHeader>
                          {t("inventory.products.restockCreated")}
                        </Table.ColumnHeader>
                        <Table.ColumnHeader>
                          {t("inventory.products.restockArrived")}
                        </Table.ColumnHeader>
                      </Table.Row>
                    </Table.Header>
                    <Table.Body>
                      {restockQ.rows.map((r) => (
                        <Table.Row key={r.id}>
                          <Table.Cell>
                            {restockSupplierRefs.get(r.supplierId)?.name ?? "—"}
                          </Table.Cell>
                          <Table.Cell textAlign="end">
                            {formatMoney(r.price)}
                          </Table.Cell>
                          <Table.Cell textAlign="end">
                            {r.qty.toString()}
                          </Table.Cell>
                          <Table.Cell textAlign="end">
                            {formatDiscount(r.discountType, r.discountValue)}
                            {r.discountValue > 0n && r.discountPerItem
                              ? ` ${t("purchasing.perItemSuffix")}`
                              : ""}
                          </Table.Cell>
                          <Table.Cell>
                            {r.restockCreatedAt > 0n
                              ? formatUnix(r.restockCreatedAt)
                              : "—"}
                          </Table.Cell>
                          <Table.Cell>
                            {r.restockArrivedAt > 0n
                              ? formatUnix(r.restockArrivedAt)
                              : "—"}
                          </Table.Cell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </Table.Root>
                </TableScroll>
              )}
              <Pagination
                page={restockPage.page}
                pageSize={restockPage.pageSize}
                total={restockQ.total}
                onPageChange={restockPage.setPage}
                onPageSizeChange={restockPage.setPageSize}
              />
            </Tabs.Content>

            <Tabs.Content value="movements" p={4}>
              <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                <Table.Root size="sm" stickyHeader>
                  <Table.Header bg="bg.muted">
                    <Table.Row>
                      <Table.ColumnHeader>
                        {t("inventory.movements.when")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader>
                        {t("inventory.movements.type")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader textAlign="end">
                        {t("inventory.movements.qty")}
                      </Table.ColumnHeader>
                      <Table.ColumnHeader>
                        {t("inventory.movements.reason")}
                      </Table.ColumnHeader>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {movementsQ.rows.map((m) => (
                      <Table.Row key={m.id}>
                        <Table.Cell>{formatUnix(m.createdAt)}</Table.Cell>
                        <Table.Cell>
                          {t(
                            `inventory.movements.types.${movementTypeKey(m.type)}`,
                          )}
                        </Table.Cell>
                        <Table.Cell textAlign="end">
                          {m.qty > 0 ? `+${m.qty}` : m.qty}
                        </Table.Cell>
                        <Table.Cell>{m.reason}</Table.Cell>
                      </Table.Row>
                    ))}
                    {movementsQ.rows.length === 0 && (
                      <Table.Row>
                        <Table.Cell colSpan={4}>
                          <Text color="fg.muted" textAlign="center" py={4}>
                            {t("common.noResults")}
                          </Text>
                        </Table.Cell>
                      </Table.Row>
                    )}
                  </Table.Body>
                </Table.Root>
              </TableScroll>
              <Pagination
                page={movementsPage.page}
                pageSize={movementsPage.pageSize}
                total={movementsQ.total}
                onPageChange={movementsPage.setPage}
                onPageSizeChange={movementsPage.setPageSize}
              />
            </Tabs.Content>

            <Tabs.Content value="discount" p={4}>
              <DiscountTab productId={id} />
            </Tabs.Content>
          </Tabs.Root>
        </Box>
      </Stack>

      <EditProductDialog
        product={editing ? med : null}
        onClose={() => setEditing(false)}
      />

      <ConfirmDialog
        open={pendingArchive}
        title={t("common.archive")}
        body={t("inventory.products.confirmArchive")}
        confirmLabel={t("common.archive")}
        loading={archive.isPending}
        onConfirm={onArchive}
        onCancel={() => setPendingArchive(false)}
      />
      <ConfirmDialog
        open={pendingUnarchive}
        title={t("inventory.products.unarchive")}
        body={t("inventory.products.confirmUnarchive")}
        confirmLabel={t("inventory.products.unarchive")}
        confirmColorPalette="green"
        loading={unarchive.isPending}
        onConfirm={onUnarchive}
        onCancel={() => setPendingUnarchive(false)}
      />
    </Box>
  );
}

// The page's card frame: a bg.subtle body under a bg.muted header bar. Same
// vocabulary as the tab strip below, whose Tabs.List *is* its header bar — so
// the caller owns the body padding (a table wants its own overflow container).
function Card({ title, children }: { title: string; children: ReactNode }) {
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
function MarginCell({ sell, cost }: { sell: bigint; cost: bigint }) {
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
function UnitsCard({ product }: { product: Product }) {
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
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.marginCol")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("priceTiers.section")}
                </Table.ColumnHeader>
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
                    <Table.Cell textAlign="end">
                      <MarginCell
                        sell={u.sellPrice}
                        cost={product.referenceCost * u.factor}
                      />
                    </Table.Cell>
                    <Table.Cell textAlign="end">
                      {tierCount > 0 ? (
                        <Text color="purple.fg">
                          {t("priceTiers.unitTierCount", { count: tierCount })}
                        </Text>
                      ) : (
                        <Text color="fg.muted">—</Text>
                      )}
                    </Table.Cell>
                  </Table.Row>
                );
              })}
              {product.units.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={5}>
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

function Field({
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

function Tile({
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
    <Box
      colorPalette={palette}
      bg="colorPalette.subtle"
      borderWidth="1px"
      borderColor="colorPalette.muted"
      borderRadius="lg"
      px={3}
      py={2}
    >
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontSize="lg" fontWeight="semibold" color="colorPalette.fg">
        {value}
      </Text>
      {sub && (
        <Text fontSize="xs" color="fg.muted">
          {sub}
        </Text>
      )}
    </Box>
  );
}

// Local YYYY-MM-DD for the "expired" check (expires_at is a date string).
function todayStr() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// Per-product discount manager: the Product detail "Discount" tab.
function DiscountTab({ productId }: { productId: string }) {
  const { t } = useTranslation();
  const page = usePageState(`discounts:${productId}`);
  const q = useProductDiscountsQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
  });
  const del = useDeleteProductDiscountMutation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<ProductDiscount | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ProductDiscount | null>(
    null,
  );
  const today = useMemo(() => todayStr(), []);
  const rows = q.rows;

  const modeLabel = (d: ProductDiscount) => {
    if (d.perItem)
      return d.discountType === "PERCENT"
        ? t("productDiscounts.modePercentItem")
        : t("productDiscounts.modeFixedItem");
    return d.discountType === "PERCENT"
      ? t("productDiscounts.modePercent")
      : t("productDiscounts.modeFixed");
  };

  return (
    <Box>
      <HStack justify="flex-end" mb={3}>
        <Button
          size="sm"
          colorPalette="blue"
          onClick={() => {
            setEditing(null);
            setDrawerOpen(true);
          }}
        >
          <Plus size={16} />
          {t("productDiscounts.add")}
        </Button>
      </HStack>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>
                {t("productDiscounts.mode")}
              </Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("productDiscounts.value")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>
                {t("productDiscounts.rule")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>
                {t("productDiscounts.expiresAt")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {rows.map((d) => {
              const expired = d.expiresAt !== "" && d.expiresAt < today;
              return (
                <Table.Row key={d.id}>
                  <Table.Cell>{modeLabel(d)}</Table.Cell>
                  <Table.Cell textAlign="end" fontFamily="mono">
                    {formatDiscountValue(
                      d.discountType,
                      d.perItem,
                      d.value,
                      formatMoney,
                      "/item",
                    )}
                  </Table.Cell>
                  <Table.Cell color="fg.muted">
                    {d.minQty > 0 && d.minQtyUnitName
                      ? t("productDiscounts.minQtyRuleUnit", {
                          count: d.minQty,
                          unit: d.minQtyUnitName,
                        })
                      : d.minQty > 1
                        ? t("productDiscounts.minQtyRule", { count: d.minQty })
                        : t("productDiscounts.noRule")}
                  </Table.Cell>
                  <Table.Cell>
                    {d.expiresAt ? (
                      <HStack gap={2}>
                        <Text>{d.expiresAt}</Text>
                        {expired && (
                          <Badge colorPalette="red">
                            {t("productDiscounts.expired")}
                          </Badge>
                        )}
                      </HStack>
                    ) : (
                      <Text color="fg.muted">
                        {t("productDiscounts.noExpiry")}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    <HStack gap={1}>
                      <Button
                        size="xs"
                        variant="ghost"
                        onClick={() => {
                          setEditing(d);
                          setDrawerOpen(true);
                        }}
                      >
                        <Pencil size={14} />
                      </Button>
                      <Button
                        size="xs"
                        variant="ghost"
                        colorPalette="red"
                        onClick={() => setPendingDelete(d)}
                      >
                        <Trash2 size={14} />
                      </Button>
                    </HStack>
                  </Table.Cell>
                </Table.Row>
              );
            })}
            {rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={5}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("productDiscounts.empty")}
                  </Text>
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

      <ProductDiscountDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        productId={productId}
        editing={editing}
      />
      <ConfirmDialog
        open={pendingDelete != null}
        title={t("productDiscounts.deleteTitle")}
        body={t("productDiscounts.deleteBody")}
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

// Per-product grosir (wholesale) ladder manager: the Product detail "Grosir" card,
// which sits beside the unit list. Grouped by unit because a tier prices ONE unit
// — a pcs ladder and a box ladder are independent, and a flat list would hide that.
function GrosirPanel({ product }: { product: Product }) {
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
