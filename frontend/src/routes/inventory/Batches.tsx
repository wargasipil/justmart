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
import { zodResolver } from "@hookform/resolvers/zod";
import { Plus, Search, Upload } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { z } from "zod";

import DateRangeFilter from "../../components/DateRangeFilter";
import EntityDrawer from "../../components/EntityDrawer";
import ExpiryBadge from "../../components/ExpiryBadge";
import ImportStockDialog from "./ImportStockDialog";
import FormField from "../../components/FormField";
import Pagination from "../../components/Pagination";
import SearchableSelect from "../../components/SearchableSelect";
import SupplierSelect, { supplierLabel } from "../../components/SupplierSelect";
import TableScroll from "../../components/TableScroll";
import { searchProducts } from "../../queries/products";
import { resolveRange, type DateRange } from "../../lib/dateRange";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useBatchesQuery, useCreateBatchMutation } from "../../queries/batches";
import { useProductRefs, useSupplierRefs } from "../../queries/refs";

const Schema = z.object({
  productId: z.string().min(1),
  supplierId: z.string(),
  batchNumber: z.string(),
  expiryDate: z.string().min(1),
  costPrice: z.coerce.bigint().min(0n),
  receivedAt: z.string(),
  initialQuantity: z.coerce.bigint().min(0n),
});
type FormValues = z.infer<typeof Schema>;

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
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${dateField}|${fromUnix}|${toUnix}|${supplierId}`,
  );
  const batchesQ = useBatchesQuery({
    // Scope rows to the active warehouse: only lots with stock here (backend
    // HAVING SUM(qty in warehouse) > 0). Qty is already active-warehouse.
    onlyInStock: true,
    query,
    supplierId,
    dateField,
    fromUnix,
    toUnix,
    page,
    pageSize,
  });
  // Resolve the page's product names (resolve-by-IDs; the CreateDrawer selects
  // use server-side search via loadOptions).
  const medRefs = useProductRefs(
    useMemo(() => batchesQ.rows.map((b) => b.productId), [batchesQ.rows]),
  );
  // Table cells only — <SupplierSelect> resolves its own trigger label.
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
                  <Table.Cell>
                    <HStack gap={2}>
                      <Text>{b.expiryDate}</Text>
                      <ExpiryBadge expiry={b.expiryDate} />
                    </HStack>
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
        total={batchesQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
      <ImportStockDialog open={importOpen} onClose={() => setImportOpen(false)} />
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateBatchMutation();
  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: {
      productId: "",
      supplierId: "",
      batchNumber: "",
      expiryDate: "",
      costPrice: 0n,
      receivedAt: "",
      initialQuantity: 0n,
    },
  });

  const submit = form.handleSubmit(async (values) => {
    try {
      await create.mutateAsync(values);
      toast.success(t("common.create") + " ✓");
      form.reset();
      onClose();
    } catch {
      /* toast handled globally */
    }
  });

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      title={t("inventory.batches.addTitle")}
      footer={
        <HStack justify="space-between">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending}>
            {t("inventory.batches.receive")}
          </Button>
        </HStack>
      }
    >
      <form onSubmit={submit}>
        <Stack gap={4}>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.batches.product")} *
            </Text>
            <SearchableSelect
              value={form.watch("productId")}
              onChange={(v) => form.setValue("productId", v)}
              loadOptions={searchProducts}
              itemToString={(m) => `${m.sku} · ${m.name}`}
              itemToValue={(m) => m.id}
              placeholder={t("inventory.batches.selectProduct")}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("inventory.batches.supplier")}
            </Text>
            <SupplierSelect
              value={form.watch("supplierId")}
              onChange={(v) => form.setValue("supplierId", v)}
              placeholder={t("inventory.batches.supplierNone")}
            />
          </Stack>
          <FormField
            control={form.control}
            name="batchNumber"
            label={t("inventory.batches.batchNumber")}
          />
          <FormField
            control={form.control}
            name="expiryDate"
            label={t("inventory.batches.expiry")}
            type="date"
            required
          />
          <FormField
            control={form.control}
            name="receivedAt"
            label={t("inventory.batches.received")}
            type="date"
          />
          <FormField
            control={form.control}
            name="costPrice"
            label={t("inventory.batches.costPerUnit")}
            money
          />
          <FormField
            control={form.control}
            name="initialQuantity"
            label={t("inventory.batches.initialQty")}
            number
            required
          />
        </Stack>
      </form>
    </EntityDrawer>
  );
}
