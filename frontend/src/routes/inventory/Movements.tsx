import { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";

import BatchSelect from "../../components/BatchSelect";
import DateRangeFilter from "../../components/DateRangeFilter";
import ExportButton from "../../components/ExportButton";
import Pagination from "../../components/Pagination";
import TableScroll from "../../components/TableScroll";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { downloadCsv } from "../../lib/csv";
import { resolveRange, type DateRange } from "../../lib/dateRange";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { resolveBatchMap, useBatchRefs } from "../../queries/refs";
import { fetchMovementsForExport, useMovementsQuery } from "../../queries/stock";
import RecordDrawer from "./movementDrawers";

function typeKey(type: MovementType): string {
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

export default function Movements() {
  const { t } = useTranslation();
  const [filterBatch, setFilterBatch] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  // "" = Any date (the picker's own off state) — send no bounds at all then.
  // The ledger has a single filterable date (created_at) and ListMovements has
  // no date_field param, so the field value stays local: it only gates bounds.
  const [dateField, setDateField] = useState("");
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const dateFields = [{ value: "created", label: t("inventory.movements.when") }];
  const fromUnix = dateField ? range.fromUnix : 0;
  const toUnix = dateField ? range.toUnix : 0;
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${filterBatch}|${query}|${fromUnix}|${toUnix}`,
  );
  const movementsQ = useMovementsQuery({
    batchId: filterBatch || undefined,
    query,
    fromUnix,
    toUnix,
    page,
    pageSize,
  });
  // Resolve the page's batch refs (batch_number + product_name) + the active
  // filter batch (resolve-by-IDs; the batch filter still searches server-side).
  const batchRefs = useBatchRefs(
    useMemo(
      () => [filterBatch, ...movementsQ.rows.map((m) => m.batchId)],
      [filterBatch, movementsQ.rows],
    ),
  );

  const onExport = async () => {
    const rows = await fetchMovementsForExport({
      batchId: filterBatch || undefined,
      query,
      fromUnix,
      toUnix,
    });
    const refs = await resolveBatchMap(rows.map((m) => m.batchId));
    downloadCsv(
      `movements-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((m) => {
        const r = refs.get(m.batchId);
        return {
          date: formatUnix(m.createdAt),
          product: r?.productName ?? "—",
          batch: r?.batchNumber || "—",
          type: t(`inventory.movements.types.${typeKey(m.type)}`),
          qty: m.qty,
          reason: m.reason,
        };
      }),
      [
        { key: "date", header: t("inventory.movements.when") },
        { key: "product", header: t("inventory.batches.product") },
        { key: "batch", header: t("inventory.movements.batch"), text: true },
        { key: "type", header: t("inventory.movements.type") },
        { key: "qty", header: t("inventory.movements.qty") },
        { key: "reason", header: t("inventory.movements.reason") },
      ],
    );
  };

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
              width="220px"
              placeholder={t("inventory.movements.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <BatchSelect
            size="sm"
            width="260px"
            value={filterBatch}
            onChange={setFilterBatch}
            // A ledger must reach lots that are now empty — their movements are
            // exactly what you came here to read.
            onlyInStock={false}
            clearable
            selectedLabel={(() => {
              const r = batchRefs.get(filterBatch);
              if (!r) return undefined;
              const lot = r.batchNumber || r.id.slice(0, 8);
              return r.productName ? `${r.productName} · ${lot}` : lot;
            })()}
            placeholder={t("inventory.movements.filterAll")}
          />
          <DateRangeFilter
            value={range}
            onChange={setRange}
            fields={dateFields}
            field={dateField}
            onFieldChange={setDateField}
          />
        </HStack>
        <HStack gap={2}>
          <ExportButton onExport={onExport} />
          <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
            <Plus size={16} />
            {t("inventory.movements.record")}
          </Button>
        </HStack>
      </HStack>

      {movementsQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <TableScroll>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.movements.when")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.movements.batch")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.movements.type")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.movements.qty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.movements.reason")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {movementsQ.rows.map((m) => {
                const ref = batchRefs.get(m.batchId);
                return (
                  <Table.Row key={m.id}>
                    <Table.Cell>{formatUnix(m.createdAt)}</Table.Cell>
                    <Table.Cell>
                      {ref?.productName ?? "—"} · {ref?.batchNumber || "—"}
                    </Table.Cell>
                    <Table.Cell>{t(`inventory.movements.types.${typeKey(m.type)}`)}</Table.Cell>
                    <Table.Cell>{m.qty > 0 ? `+${m.qty}` : m.qty}</Table.Cell>
                    <Table.Cell>{m.reason}</Table.Cell>
                  </Table.Row>
                );
              })}
              {movementsQ.rows.length === 0 && (
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
      )}

      <Pagination
        page={page}
        pageSize={pageSize}
        total={movementsQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <RecordDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      />
    </Stack>
  );
}
