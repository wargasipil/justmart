import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Box, Button, HStack, Input, Spinner, Stack, Table, Tabs, Text } from "@chakra-ui/react";
import { Plus, Search, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ColumnsPopover, { type GroupSpec } from "../../components/ColumnsPopover";
import DatePickerField from "../../components/DatePicker";
import ExportButton from "../../components/ExportButton";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import ProductImage from "../../components/ProductImage";
import StockUnitPopover from "../../components/StockUnitPopover";
import TableScroll from "../../components/TableScroll";
import { Product } from "../../gen/inventory_iface/v1/product_pb";
import { downloadCsv } from "../../lib/csv";
import { formatDiscount, formatMoney, formatUnixOrDash } from "../../lib/format";
import { ALL_LIMIT, usePageState } from "../../lib/pagination";
import { formatStock, unitGroupsFromCatalog } from "../../lib/stockUnit";
import { fetchProductsForExport, useProductsQuery } from "../../queries/products";
import { useSupplierRefs } from "../../queries/refs";
import { useBusinessMode } from "../../queries/settings";
import { useUnitBasesQuery } from "../../queries/units";
import { DEFAULT_PRODUCT_COLUMNS, usePreferencesStore } from "../../stores/preferences";
import { CreateProductDialog } from "./productDrawers";
import ImportProductsDialog from "./ImportProductsDialog";

export default function Products() {
  const { t } = useTranslation();
  const navigate = useNavigate();
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
  const productsQ = useProductsQuery({ query, opnameBefore, onlyArchived, page, pageSize });
  const stockUnitsByBase = usePreferencesStore((s) => s.productStockUnitsByBase);
  const setStockUnitByBase = usePreferencesStore((s) => s.setProductStockUnitByBase);
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

  // Resolve the "last supplier that restocked" names for the page's rows.
  const supplierRefs = useSupplierRefs(
    useMemo(
      () =>
        Array.from(
          new Set(productsQ.rows.map((m) => m.lastRestockSupplierId).filter(Boolean)),
        ),
      [productsQ.rows],
    ),
  );

  const stockCell = (qty: bigint, m: Product) =>
    formatStock(qty, m.units, m.unit, stockUnitsByBase);

  // Column registry, in display order. `always` columns ignore the visibility
  // toggle (name = identity). Restock columns render "—" until a receipt records
  // one (lastRestockArrivedAt is the "has restock" sentinel).
  type Col = { id: string; header: string; alignEnd?: boolean; always?: boolean; render: (m: Product) => ReactNode };
  const allCols: Col[] = [
    // Identity cell: photo + name + SKU read as one thing (which product is
    // this?), so they share a column rather than competing as three. Always
    // shown and not toggleable — a row with its identity hidden is unusable.
    {
      id: "name",
      header: t("inventory.products.name"),
      always: true,
      render: (m) => (
        <HStack gap={3}>
          <ProductImage
            productId={m.id}
            name={m.name}
            version={Number(m.imageUpdatedAt)}
            size={56}
            zoomable
          />
          <Stack gap={0} minW={0}>
            <Text>{m.name}</Text>
            <Text fontSize="xs" color="fg.muted" fontFamily="mono">
              {m.sku}
            </Text>
          </Stack>
        </HStack>
      ),
    },
    { id: "unit", header: t("inventory.products.unit"), render: (m) => m.unit },
    { id: "unitPrice", header: t("inventory.products.unitPrice"), render: (m) => formatMoney(m.unitPrice) },
    { id: "ready", header: t("inventory.products.readyStock"), alignEnd: true, render: (m) => stockCell(m.readyStock, m) },
    {
      id: "onOrder",
      header: t("inventory.products.onOrder"),
      alignEnd: true,
      render: (m) => (m.onOrderStock > 0n ? stockCell(m.onOrderStock, m) : "—"),
    },
    { id: "lastStocktake", header: t("inventory.products.lastStocktake"), render: (m) => m.lastStocktakeDate || "—" },
    {
      id: "lastRestockPrice",
      header: t("inventory.products.lastRestockPrice"),
      alignEnd: true,
      render: (m) => (m.lastRestockArrivedAt > 0n ? formatMoney(m.lastRestockPrice) : "—"),
    },
    {
      id: "lastRestockQty",
      header: t("inventory.products.lastRestockQty"),
      alignEnd: true,
      render: (m) => (m.lastRestockArrivedAt > 0n ? m.lastRestockQty.toString() : "—"),
    },
    {
      id: "lastRestockDiscount",
      header: t("inventory.products.lastRestockDiscount"),
      alignEnd: true,
      render: (m) =>
        m.lastRestockArrivedAt > 0n
          ? formatDiscount(m.lastRestockDiscountType, m.lastRestockDiscountValue)
          : "—",
    },
    {
      id: "lastRestockCreated",
      header: t("inventory.products.lastRestockCreated"),
      render: (m) => formatUnixOrDash(m.lastRestockCreatedAt),
    },
    {
      id: "lastRestockArrived",
      header: t("inventory.products.lastRestockArrived"),
      render: (m) => formatUnixOrDash(m.lastRestockArrivedAt),
    },
    {
      id: "lastSupplier",
      header: t("inventory.products.lastSupplier"),
      render: (m) =>
        m.lastRestockSupplierId ? supplierRefs.get(m.lastRestockSupplierId)?.name ?? "—" : "—",
    },
  ];
  const cols = allCols.filter((c) => c.always || visibleSet.has(c.id));

  const columnGroups: GroupSpec[] = [
    {
      id: "standard",
      label: t("inventory.products.colGroupStandard"),
      fields: [
        // No `image` / `sku` entries: both are folded into the always-on
        // identity column, so there is nothing to toggle.
        { id: "unit", label: t("inventory.products.unit") },
        { id: "unitPrice", label: t("inventory.products.unitPrice") },
        { id: "ready", label: t("inventory.products.readyStock") },
        { id: "onOrder", label: t("inventory.products.onOrder") },
        { id: "lastStocktake", label: t("inventory.products.lastStocktake") },
      ],
    },
    {
      id: "restock",
      label: t("inventory.products.colGroupRestock"),
      fields: [
        { id: "lastRestockPrice", label: t("inventory.products.lastRestockPrice") },
        { id: "lastRestockQty", label: t("inventory.products.lastRestockQty") },
        { id: "lastRestockDiscount", label: t("inventory.products.lastRestockDiscount") },
        { id: "lastRestockCreated", label: t("inventory.products.lastRestockCreated") },
        { id: "lastRestockArrived", label: t("inventory.products.lastRestockArrived") },
        { id: "lastSupplier", label: t("inventory.products.lastSupplier") },
      ],
    },
  ];

  const onExport = async () => {
    const rows = await fetchProductsForExport({ query, opnameBefore, onlyArchived });
    downloadCsv(
      `products-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((m) => ({
        sku: m.sku,
        name: m.name,
        unit: m.unit,
        unitPrice: Number(m.unitPrice),
        ready: Number(m.readyStock),
        onOrder: Number(m.onOrderStock),
        lastOpname: m.lastStocktakeDate || "",
        lastRestockPrice: m.lastRestockArrivedAt > 0n ? Number(m.lastRestockPrice) : "",
        lastRestockQty: m.lastRestockArrivedAt > 0n ? Number(m.lastRestockQty) : "",
        lastRestockDiscount:
          m.lastRestockArrivedAt > 0n
            ? formatDiscount(m.lastRestockDiscountType, m.lastRestockDiscountValue)
            : "",
        lastRestockArrived: formatUnixOrDash(m.lastRestockArrivedAt),
        lastSupplier: m.lastRestockSupplierId
          ? supplierRefs.get(m.lastRestockSupplierId)?.name ?? ""
          : "",
      })),
      [
        { key: "sku", header: t("inventory.products.sku"), text: true },
        { key: "name", header: t("inventory.products.name") },
        { key: "unit", header: t("inventory.products.unit") },
        { key: "unitPrice", header: t("inventory.products.unitPrice") },
        { key: "ready", header: t("inventory.products.readyStock") },
        { key: "onOrder", header: t("inventory.products.onOrder") },
        { key: "lastOpname", header: t("inventory.products.lastStocktake") },
        { key: "lastRestockPrice", header: t("inventory.products.lastRestockPrice") },
        { key: "lastRestockQty", header: t("inventory.products.lastRestockQty") },
        { key: "lastRestockDiscount", header: t("inventory.products.lastRestockDiscount") },
        { key: "lastRestockArrived", header: t("inventory.products.lastRestockArrived") },
        { key: "lastSupplier", header: t("inventory.products.lastSupplier") },
      ],
    );
  };

  return (
    <Box>
      <PageHeader title={catalogLabel} description={t("inventory.products.description")} />
      <Stack gap={4}>
        <Tabs.Root
          value={tab}
          onValueChange={(d) => setTab(d.value as "active" | "archived")}
          variant="line"
        >
          <Tabs.List>
            <Tabs.Trigger value="active">{t("inventory.products.tabActive")}</Tabs.Trigger>
            <Tabs.Trigger value="archived">{t("inventory.products.tabArchived")}</Tabs.Trigger>
          </Tabs.List>
        </Tabs.Root>

        <HStack justify="space-between" wrap="wrap" gap={2}>
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
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
            <ExportButton onExport={onExport} />
            <Button size="sm" variant="outline" onClick={() => setImportOpen(true)}>
              <Upload size={16} />
              {t("inventory.products.importTitle")}
            </Button>
            <Button size="sm" colorPalette="blue" onClick={() => setCreateOpen(true)}>
              <Plus size={16} />
              {t("inventory.products.addTitle")}
            </Button>
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
                    <Table.ColumnHeader key={c.id} textAlign={c.alignEnd ? "end" : undefined}>
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
                      <Table.Cell key={c.id} textAlign={c.alignEnd ? "end" : undefined}>
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

        <CreateProductDialog open={createOpen} onClose={() => setCreateOpen(false)} />
        <ImportProductsDialog open={importOpen} onClose={() => setImportOpen(false)} />
      </Stack>
    </Box>
  );
}
