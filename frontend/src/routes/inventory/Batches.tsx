import { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  Link as ChakraLink,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Link as RouterLink } from "react-router-dom";
import { Plus, Search, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";

import DateRangeFilter from "../../components/DateRangeFilter";
import EnumSelect from "../../components/EnumSelect";
import ImportStockDialog from "./ImportStockDialog";
import Pagination from "../../components/Pagination";
import ManufacturerSelect, { manufacturerLabel } from "../../components/ManufacturerSelect";
import SupplierSelect, { supplierLabel } from "../../components/SupplierSelect";
import TableScroll from "../../components/TableScroll";
import { ExpirySource, type Batch } from "../../gen/inventory_iface/v1/batch_pb";
import { resolveRange, type DateRange } from "../../lib/dateRange";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { useBatchesQuery } from "../../queries/batches";
import { useManufacturerRefs, useProductRefs, useSupplierRefs } from "../../queries/refs";
import BatchExpiryCell from "./BatchExpiryCell";
import BatchExpiryDialog from "./BatchExpiryDialog";
import { CreateBatchDrawer } from "./batchDrawers";

export default function Batches() {
  const { t } = useTranslation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  // "" = Any date (the picker's own off state) — send no bounds at all then.
  const [dateField, setDateField] = useState("");
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const dateFields = [
    { value: "received", label: t("inventory.batches.byReceived") },
    { value: "expiry", label: t("inventory.batches.byExpiry") },
  ];
  const fromUnix = dateField ? range.fromUnix : 0;
  const toUnix = dateField ? range.toUnix : 0;
  const [supplierId, setSupplierId] = useState("");
  const [manufacturerId, setManufacturerId] = useState("");
  // Where each lot's expiry came from. "Product default" is the shelf-check
  // worklist: dates that are an estimate because nobody typed over them.
  const [expirySource, setExpirySource] = useState<ExpirySource>(ExpirySource.UNSPECIFIED);
  const expirySourceOptions = [
    { value: String(ExpirySource.UNSPECIFIED), label: t("inventory.batches.expirySourceAll") },
    { value: String(ExpirySource.DEFAULT), label: t("inventory.batches.expirySourceDefault") },
    { value: String(ExpirySource.ENTERED), label: t("inventory.batches.expirySourceEntered") },
    { value: String(ExpirySource.NONE), label: t("inventory.batches.expirySourceNone") },
  ];
  const [editing, setEditing] = useState<Batch | null>(null);
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${dateField}|${fromUnix}|${toUnix}|${supplierId}|${manufacturerId}|${expirySource}`,
  );
  const batchesQ = useBatchesQuery({
    // Scope rows to the active warehouse: only lots with stock here (backend
    // HAVING SUM(qty in warehouse) > 0). Qty is already active-warehouse.
    onlyInStock: true,
    query,
    supplierId,
    manufacturerId,
    dateField,
    fromUnix,
    toUnix,
    expirySource,
    page,
    pageSize,
  });
  // Resolve the page's product names (resolve-by-IDs; the CreateDrawer selects
  // use server-side search via loadOptions).
  const medRefs = useProductRefs(
    useMemo(() => batchesQ.rows.map((b) => b.productId), [batchesQ.rows]),
  );
  // Table cells only — the selects resolve their own trigger labels.
  const manufacturerRefs = useManufacturerRefs(
    useMemo(() => batchesQ.rows.map((b) => b.manufacturerId).filter(Boolean), [batchesQ.rows]),
  );
  const supplierRefs = useSupplierRefs(
    useMemo(() => batchesQ.rows.map((b) => b.supplierId).filter(Boolean), [batchesQ.rows]),
  );

  return (
    <Stack gap={4}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={2} wrap="wrap">
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="240px"
              placeholder={t("inventory.batches.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <DateRangeFilter
            value={range}
            onChange={setRange}
            fields={dateFields}
            field={dateField}
            onFieldChange={setDateField}
          />
          <Box width="200px">
            <SupplierSelect
              size="sm"
              value={supplierId}
              onChange={setSupplierId}
              placeholder={t("inventory.batches.supplier")}
            />
          </Box>
          {/*
            Who MADE the stock, beside who sold it. The lot is the only row
            that records a maker, so this is the filter a recall actually
            needs: pabrik X recalls their batch, and this is where you find
            every lot of theirs still on the shelf.
          */}
          <Box width="200px">
            <ManufacturerSelect
              size="sm"
              value={manufacturerId}
              onChange={setManufacturerId}
              placeholder={t("inventory.products.manufacturer")}
            />
          </Box>
          <Box width="220px">
            <EnumSelect
              size="sm"
              value={String(expirySource)}
              onChange={(v) => setExpirySource(Number(v) as ExpirySource)}
              items={expirySourceOptions}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
              ariaLabel={t("inventory.batches.expirySourceFilter")}
            />
          </Box>
        </HStack>
        <HStack gap={2}>
          <Button size="sm" variant="outline" onClick={() => setImportOpen(true)}>
            <Upload size={16} />
            {t("inventory.batches.importTitle")}
          </Button>
          <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
            <Plus size={16} />
            {t("inventory.batches.addTitle")}
          </Button>
        </HStack>
      </HStack>

      {batchesQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <TableScroll>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.batches.product")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.batchNumber")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.supplier")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.products.manufacturer")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.expiry")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.cost")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.qty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.po")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {batchesQ.rows.map((b) => (
                <Table.Row key={b.id}>
                  <Table.Cell>{medRefs.get(b.productId)?.name ?? "—"}</Table.Cell>
                  <Table.Cell>{b.batchNumber || "—"}</Table.Cell>
                  <Table.Cell>{supplierLabel(supplierRefs.get(b.supplierId)) ?? "—"}</Table.Cell>
                  {/*
                    Who MADE the lot. Blank on everything received before the
                    column existed and on the manual CreateBatch path -- the
                    dash is honest there, the alternative would be the
                    product's current maker invented as this lot's history.
                  */}
                  <Table.Cell>
                    {manufacturerLabel(manufacturerRefs.get(b.manufacturerId)) ?? "—"}
                  </Table.Cell>
                  <Table.Cell>
                    <BatchExpiryCell batch={b} onEdit={() => setEditing(b)} />
                  </Table.Cell>
                  <Table.Cell>{formatMoney(b.costPrice)}</Table.Cell>
                  <Table.Cell>{String(b.currentQuantity)}</Table.Cell>
                  <Table.Cell fontFamily="mono">
                    {b.purchaseOrderId ? (
                      <ChakraLink asChild colorPalette="blue">
                        <RouterLink to={`/purchasing/${b.purchaseOrderId}`}>
                          {b.poNo || b.purchaseOrderId.slice(0, 8)}
                        </RouterLink>
                      </ChakraLink>
                    ) : (
                      <Text color="fg.muted">—</Text>
                    )}
                  </Table.Cell>
                </Table.Row>
              ))}
              {batchesQ.rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={7}>
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
        total={batchesQ.total} loading={batchesQ.isPlaceholderData}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateBatchDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
      <ImportStockDialog open={importOpen} onClose={() => setImportOpen(false)} />
      <BatchExpiryDialog
        batch={editing}
        productName={editing ? medRefs.get(editing.productId)?.name : undefined}
        onClose={() => setEditing(null)}
      />
    </Stack>
  );
}
