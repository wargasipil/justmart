import { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  SimpleGrid,
  Spinner,
  Stack,
  Table,
  Tabs,
  Text,
} from "@chakra-ui/react";
import { Plus, Search, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ColumnsPopover from "../../components/ColumnsPopover";
import DatePickerField from "../../components/DatePicker";
import ExportButton from "../../components/ExportButton";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import StockUnitPopover from "../../components/StockUnitPopover";
import SummaryTile from "../../components/SummaryTile";
import TableScroll from "../../components/TableScroll";
import { formatCount, formatMoney } from "../../lib/format";
import { ALL_LIMIT, usePageState } from "../../lib/pagination";
import { canSeeCost } from "../../lib/roles";
import { useAuth } from "../../lib/auth";
import { unitGroupsFromCatalog } from "../../lib/stockUnit";
import {
  useProductsQuery,
  useProductsSummaryQuery,
} from "../../queries/products";
import { useSupplierRefs } from "../../queries/refs";
import { useBusinessMode } from "../../queries/settings";
import { useUnitBasesQuery } from "../../queries/units";
import {
  DEFAULT_PRODUCT_COLUMNS,
  usePreferencesStore,
} from "../../stores/preferences";
import { CreateProductDialog } from "./productDrawers";
import ImportProductsDialog from "./ImportProductsDialog";
import {
  buildProductColumns,
  exportProductsCsv,
} from "./productListColumns";

export default function Products() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { user } = useAuth();
  // The till reads this page: no cost columns, no valuation tiles, no writes.
  // Not cosmetic — the summary + supplier RPCs are manager-only, so firing them
  // here would toast a permission error on every load.
  const showCost = canSeeCost(user?.role);
  const { isPharmacy } = useBusinessMode();
  const catalogLabel = isPharmacy ? t("nav.medicines") : t("nav.products");
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [opnameBefore, setOpnameBefore] = useState("");
  const [tab, setTab] = useState<"active" | "archived">("active");
  const onlyArchived = tab === "archived";

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${opnameBefore}|${tab}`,
  );
  const productsQ = useProductsQuery({
    query,
    opnameBefore,
    onlyArchived,
    page,
    pageSize,
  });
  // Catalog-wide totals over EVERY matching product (server-side aggregate),
  // not a sum of the page — same filters as the list, minus paging, so the
  // tiles keep describing the table under them as the user pages.
  const summaryQ = useProductsSummaryQuery({
    query,
    opnameBefore,
    onlyArchived,
    enabled: showCost,
  });
  const summary = summaryQ.data;
  const stockUnitsByBase = usePreferencesStore(
    (s) => s.productStockUnitsByBase,
  );
  const setStockUnitByBase = usePreferencesStore(
    (s) => s.setProductStockUnitByBase,
  );
  const visibleCols = usePreferencesStore((s) => s.productListColumns);
  const setVisibleCols = usePreferencesStore((s) => s.setProductListColumns);
  const visibleSet = useMemo(() => new Set(visibleCols), [visibleCols]);
  // The whole catalog, not a page: this builds the stock-unit grouping map for
  // the list's Units popover, so a partial catalog would silently mis-group
  // rows rather than just showing fewer options.
  const unitsQ = useUnitBasesQuery({ pageSize: ALL_LIMIT });
  const stockUnitGroups = useMemo(
    () => unitGroupsFromCatalog(unitsQ.rows, productsQ.rows),
    [unitsQ.rows, productsQ.rows],
  );

  // Resolve the "last supplier that restocked" names for the page's rows. Empty
  // for the till: the ids are redacted server-side and ResolveSuppliers is
  // manager-only, so an empty list is what keeps the hook from firing.
  const supplierRefs = useSupplierRefs(
    useMemo(
      () =>
        showCost
          ? Array.from(
              new Set(
                productsQ.rows
                  .map((m) => m.lastRestockSupplierId)
                  .filter(Boolean),
              ),
            )
          : [],
      [productsQ.rows, showCost],
    ),
  );

  const { cols, columnGroups } = buildProductColumns({
    t,
    showCost,
    visibleSet,
    stockUnitsByBase,
    supplierRefs,
  });

  const onExport = () =>
    exportProductsCsv({ t, query, opnameBefore, onlyArchived, supplierRefs });

  return (
    <Box>
      <PageHeader
        title={catalogLabel}
        description={t("inventory.products.description")}
      />
      <Stack gap={4}>
        {/* Stock at a glance, above the tabs: it summarizes the whole catalog,
            so it sits outside the Active/Archived split rather than inside one
            tab's panel. Four equal cells — each count and its valuation read as
            peers rather than one subordinated under the other — collapsing to
            2-up then 1-up so the row never squeezes. The counts add BASE units
            across products, so a catalog holding both tablets and bottles adds
            them together; read them as "total units on hand", with the money as
            the comparable figure (the per-product columns carry the real unit). */}
        {/* Half of these tiles are valuations at cost, and the RPC behind all
            four is manager-only — so the row is dropped whole for the till
            rather than shown with two empty cells. */}
        {showCost && (
          <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
            <SummaryTile
              label={t("inventory.products.summary.ready")}
              value={formatCount(summary?.readyStock ?? 0n)}
            />
            <SummaryTile
              label={t("inventory.products.summary.readyValue")}
              value={formatMoney(summary?.readyValuation ?? 0n)}
            />
            <SummaryTile
              label={t("inventory.products.summary.onOrder")}
              value={formatCount(summary?.onOrderStock ?? 0n)}
            />
            <SummaryTile
              label={t("inventory.products.summary.onOrderValue")}
              value={formatMoney(summary?.onOrderValuation ?? 0n)}
            />
          </SimpleGrid>
        )}

        <Tabs.Root
          value={tab}
          onValueChange={(d) => setTab(d.value as "active" | "archived")}
          variant="line"
        >
          <Tabs.List>
            <Tabs.Trigger value="active">
              {t("inventory.products.tabActive")}
            </Tabs.Trigger>
            <Tabs.Trigger value="archived">
              {t("inventory.products.tabArchived")}
            </Tabs.Trigger>
          </Tabs.List>
        </Tabs.Root>

        <HStack justify="space-between" wrap="wrap" gap={2}>
          <Box position="relative">
            <Box
              position="absolute"
              left={2}
              top="50%"
              transform="translateY(-50%)"
              color="fg.muted"
            >
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="280px"
              placeholder={t("inventory.products.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <HStack gap={2}>
            <Text fontSize="sm" color="fg.muted">
              {t("inventory.products.opnameBefore")}
            </Text>
            <Box width="160px">
              <DatePickerField
                size="sm"
                value={opnameBefore}
                onChange={setOpnameBefore}
              />
            </Box>
            <StockUnitPopover
              byBase={stockUnitsByBase}
              onChangeBase={setStockUnitByBase}
              groups={stockUnitGroups}
            />
            <ColumnsPopover
              value={visibleSet}
              onChange={(next) => setVisibleCols(Array.from(next))}
              groups={columnGroups}
              defaults={new Set(DEFAULT_PRODUCT_COLUMNS)}
            />
            {/* Export carries the cost columns, and create/import are
                manager-only RPCs — the till's view is read-only. */}
            {showCost && (
              <>
                <ExportButton onExport={onExport} />
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setImportOpen(true)}
                >
                  <Upload size={16} />
                  {t("inventory.products.importTitle")}
                </Button>
                <Button
                  size="sm"
                  colorPalette="blue"
                  onClick={() => setCreateOpen(true)}
                >
                  <Plus size={16} />
                  {t("inventory.products.addTitle")}
                </Button>
              </>
            )}
          </HStack>
        </HStack>

        {productsQ.isLoading ? (
          <Box p={8} textAlign="center">
            <Spinner />
          </Box>
        ) : (
          <TableScroll>
            <Table.Root size="sm" stickyHeader>
              <Table.Header bg="bg.muted">
                <Table.Row>
                  {cols.map((c) => (
                    <Table.ColumnHeader
                      key={c.id}
                      textAlign={c.alignEnd ? "end" : undefined}
                    >
                      {c.header}
                    </Table.ColumnHeader>
                  ))}
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {productsQ.rows.map((m) => (
                  <Table.Row
                    key={m.id}
                    cursor="pointer"
                    _hover={{ bg: "bg.muted" }}
                    onClick={() => navigate(`/products/${m.id}`)}
                  >
                    {cols.map((c) => (
                      <Table.Cell
                        key={c.id}
                        textAlign={c.alignEnd ? "end" : undefined}
                      >
                        {c.render(m)}
                      </Table.Cell>
                    ))}
                  </Table.Row>
                ))}
                {productsQ.rows.length === 0 && (
                  <Table.Row>
                    <Table.Cell colSpan={cols.length}>
                      <Text color="fg.muted" textAlign="center" py={4}>
                        {t("common.noResults")}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                )}
              </Table.Body>
            </Table.Root>
          </TableScroll>
        )}

        <Pagination
          page={page}
          pageSize={pageSize}
          total={productsQ.total}
          onPageChange={setPage}
          onPageSizeChange={setPageSize}
        />

        <CreateProductDialog
          open={createOpen}
          onClose={() => setCreateOpen(false)}
        />
        <ImportProductsDialog
          open={importOpen}
          onClose={() => setImportOpen(false)}
        />
      </Stack>
    </Box>
  );
}
