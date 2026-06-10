import { useEffect, useState } from "react";
import {
  Box,
  Button,
  HStack,
  IconButton,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { ArrowRight, Plus, Search, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import DateRangeFilter, { resolveRange, type DateRange } from "../../components/DateRangeFilter";
import EntityDrawer from "../../components/EntityDrawer";
import EnumSelect from "../../components/EnumSelect";
import Pagination from "../../components/Pagination";
import ExportButton from "../../components/ExportButton";
import NumberInput from "../../components/NumberInput";
import BatchSelect from "../../components/BatchSelect";
import WarehouseSelect from "../../components/WarehouseSelect";
import { WAREHOUSE_KEY } from "../../lib/transport";
import type { Batch } from "../../gen/inventory_iface/v1/batch_pb";
import type { ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import { downloadCsv } from "../../lib/csv";
import { formatUnix } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useAllWarehousesQuery } from "../../queries/warehouses";
import { fetchTransfersForExport, useCreateTransferMutation, useTransfersQuery } from "../../queries/transfers";

type LineDraft = {
  batchId: string;
  productName: string;
  batchNumber: string;
  expiry: string;
  available: number; // base units in the source warehouse
  units: ProductUnit[]; // the product's active units (base first)
  unitId: string; // selected entry unit (qty is in this unit)
  qty: string; // entered in the selected unit
};

export default function Transfers() {
  const { t } = useTranslation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  const [dateOn, setDateOn] = useState(false);
  const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${dateOn ? range.fromUnix : 0}|${dateOn ? range.toUnix : 0}`,
  );
  const transfersQ = useTransfersQuery({
    query,
    fromUnix: dateOn ? range.fromUnix : 0,
    toUnix: dateOn ? range.toUnix : 0,
    page,
    pageSize,
  });

  const onExport = async () => {
    const rows = await fetchTransfersForExport({
      query,
      fromUnix: dateOn ? range.fromUnix : 0,
      toUnix: dateOn ? range.toUnix : 0,
    });
    downloadCsv(
      `transfers-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((tr) => ({
        transferNo: tr.transferNo,
        from: tr.fromWarehouseName,
        to: tr.toWarehouseName,
        lines: tr.lines.length,
        note: tr.note,
        date: formatUnix(tr.createdAt),
      })),
      [
        { key: "transferNo", header: t("transfers.transferNo") },
        { key: "from", header: t("transfers.from") },
        { key: "to", header: t("transfers.to") },
        { key: "lines", header: t("transfers.lines") },
        { key: "note", header: t("transfers.note") },
        { key: "date", header: t("transfers.when") },
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
              width="240px"
              placeholder={t("transfers.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <EnumSelect
            size="sm"
            width="150px"
            value={dateOn ? "on" : "off"}
            onChange={(v) => setDateOn(v === "on")}
            items={[
              { value: "off", label: t("common.anyDate") },
              { value: "on", label: t("common.dateRange") },
            ]}
            itemToString={(o) => o.label}
            itemToValue={(o) => o.value}
          />
          {dateOn && <DateRangeFilter value={range} onChange={setRange} />}
        </HStack>
        <HStack gap={2}>
          <ExportButton onExport={onExport} />
          <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
            <Plus size={16} />
            {t("transfers.newTransfer")}
          </Button>
        </HStack>
      </HStack>

      {transfersQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("transfers.transferNo")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.route")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.lines")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.note")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("transfers.when")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {transfersQ.rows.map((tr) => (
              <Table.Row key={tr.id}>
                <Table.Cell fontFamily="mono">{tr.transferNo}</Table.Cell>
                <Table.Cell>
                  <HStack gap={1}>
                    <Text>{tr.fromWarehouseName}</Text>
                    <ArrowRight size={14} />
                    <Text>{tr.toWarehouseName}</Text>
                  </HStack>
                </Table.Cell>
                <Table.Cell>{tr.lines.length}</Table.Cell>
                <Table.Cell>{tr.note}</Table.Cell>
                <Table.Cell>{formatUnix(tr.createdAt)}</Table.Cell>
              </Table.Row>
            ))}
            {transfersQ.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={5}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("transfers.empty")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      )}

      <Pagination
        page={page}
        pageSize={pageSize}
        total={transfersQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Stack>
  );
}

function CreateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  // Selector mode: needs every warehouse for the From/To pickers, not a page.
  const warehousesQ = useAllWarehousesQuery();
  const create = useCreateTransferMutation();
  // Default the source to the active warehouse so the common "move out of here"
  // case needs no extra click; batch availability is scoped to it.
  const [from, setFrom] = useState(() => localStorage.getItem(WAREHOUSE_KEY) ?? "");
  const [to, setTo] = useState("");
  const [note, setNote] = useState("");
  const [lines, setLines] = useState<LineDraft[]>([]);

  const reset = () => {
    setFrom(localStorage.getItem(WAREHOUSE_KEY) ?? "");
    setTo("");
    setNote("");
    setLines([]);
  };

  // Availability is per-source-warehouse, so changing the source invalidates
  // every picked line — clear them. Also break a From == To collision.
  const onChangeFrom = (id: string) => {
    setFrom(id);
    setLines([]);
    if (id && id === to) setTo("");
  };

  // Append a picked batch as a line, capturing its product name + availability
  // straight from the picker (no second lookup). Dedupe by batch id.
  const appendLine = (b: Batch) => {
    setLines((ls) => {
      if (ls.some((l) => l.batchId === b.id)) return ls;
      const units = (b.units ?? []).filter((u) => u.active);
      const base = units.find((u) => u.isBase);
      return [
        ...ls,
        {
          batchId: b.id,
          productName: b.productName || b.batchNumber || b.id.slice(0, 8),
          batchNumber: b.batchNumber,
          expiry: b.expiryDate,
          available: Number(b.currentQuantity),
          units,
          unitId: base?.id ?? units[0]?.id ?? "",
          qty: "",
        },
      ];
    });
  };

  // Qty is entered in the line's selected unit; convert to base via the unit's
  // factor. NumberInput clamps the unit-qty so base never exceeds availability.
  const factorOf = (l: LineDraft) =>
    Number(l.units.find((u) => u.id === l.unitId)?.factor ?? 1) || 1;
  const baseQtyOf = (l: LineDraft) => Number(l.qty || 0) * factorOf(l);
  const validLines = lines.filter((l) => baseQtyOf(l) > 0 && baseQtyOf(l) <= l.available);
  const canSubmit = from !== "" && to !== "" && from !== to && validLines.length > 0;

  const submit = async () => {
    try {
      await create.mutateAsync({
        fromWarehouseId: from,
        toWarehouseId: to,
        note,
        lines: validLines.map((l) => ({ batchId: l.batchId, qty: baseQtyOf(l) })),
      });
      toast.success(t("transfers.created"));
      reset();
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  const warehouseItems = warehousesQ.rows;

  return (
    <EntityDrawer
      open={open}
      onClose={onClose}
      size="lg"
      title={t("transfers.newTransfer")}
      footer={
        <HStack justify="space-between" w="100%">
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button colorPalette="blue" onClick={submit} loading={create.isPending} disabled={!canSubmit}>
            {t("transfers.submit")}
          </Button>
        </HStack>
      }
    >
      <Stack gap={4}>
        <HStack gap={3} align="flex-end">
          <Stack gap={1} flex="1">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("transfers.from")} *
            </Text>
            <WarehouseSelect
              value={from}
              onChange={onChangeFrom}
              warehouses={warehouseItems}
              placeholder={t("transfers.selectWarehouse")}
            />
          </Stack>
          <Box color="fg.muted" pb={2}>
            <ArrowRight size={16} />
          </Box>
          <Stack gap={1} flex="1">
            <Text fontSize="sm" fontWeight="medium" color="fg.muted">
              {t("transfers.to")} *
            </Text>
            <WarehouseSelect
              value={to}
              onChange={setTo}
              warehouses={warehouseItems}
              excludeId={from}
              placeholder={t("transfers.selectWarehouse")}
            />
          </Stack>
        </HStack>

        <Stack gap={2}>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted">
            {t("transfers.items")} *
          </Text>

          {lines.length > 0 && (
            <Table.Root size="sm" borderWidth="1px" borderRadius="md">
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("transfers.product")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="right">{t("transfers.available")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("transfers.qty")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("transfers.unit")}</Table.ColumnHeader>
                  <Table.ColumnHeader />
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {lines.map((l, idx) => {
                  const factor = factorOf(l);
                  const baseName = l.units.find((u) => u.isBase)?.name ?? "";
                  const maxUnitQty = Math.floor(l.available / factor);
                  const baseQty = baseQtyOf(l);
                  return (
                    <Table.Row key={l.batchId}>
                      <Table.Cell>
                        <Stack gap={0} minW={0}>
                          <Text fontSize="sm" fontWeight="medium" truncate>
                            {l.productName}
                          </Text>
                          <Text fontSize="xs" color="fg.muted" truncate>
                            {l.batchNumber || l.batchId.slice(0, 8)}
                            {l.expiry ? ` · ${t("transfers.expShort")} ${l.expiry}` : ""}
                          </Text>
                        </Stack>
                      </Table.Cell>
                      <Table.Cell textAlign="right" color="fg.muted" whiteSpace="nowrap">
                        {l.available}
                        {baseName ? ` ${baseName}` : ""}
                      </Table.Cell>
                      <Table.Cell>
                        <Stack gap={0}>
                          <NumberInput
                            width="80px"
                            value={l.qty}
                            max={maxUnitQty}
                            placeholder={t("transfers.qty")}
                            onChange={(raw) =>
                              setLines((ls) => ls.map((x, i) => (i === idx ? { ...x, qty: raw } : x)))
                            }
                          />
                          {factor > 1 && baseQty > 0 && (
                            <Text fontSize="xs" color="fg.muted" whiteSpace="nowrap">
                              = {baseQty} {baseName}
                            </Text>
                          )}
                        </Stack>
                      </Table.Cell>
                      <Table.Cell>
                        {l.units.length > 1 ? (
                          <EnumSelect
                            size="sm"
                            width="110px"
                            value={l.unitId}
                            onChange={(v) =>
                              setLines((ls) =>
                                ls.map((x, i) => (i === idx ? { ...x, unitId: v, qty: "" } : x)),
                              )
                            }
                            items={l.units}
                            itemToString={(u) => u.name}
                            itemToValue={(u) => u.id}
                          />
                        ) : (
                          <Text fontSize="sm" color="fg.muted">
                            {baseName || "—"}
                          </Text>
                        )}
                      </Table.Cell>
                      <Table.Cell>
                        <IconButton
                          aria-label={t("transfers.removeLine")}
                          size="sm"
                          variant="ghost"
                          onClick={() => setLines((ls) => ls.filter((_, i) => i !== idx))}
                        >
                          <Trash2 size={14} />
                        </IconButton>
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
              </Table.Body>
            </Table.Root>
          )}

          <BatchSelect
            warehouseId={from}
            value=""
            onSelectItem={appendLine}
            onlyInStock
            excludeIds={lines.map((l) => l.batchId)}
            disabled={!from}
            placeholder={t("transfers.addBatch")}
          />
          {!from && (
            <Text fontSize="xs" color="fg.muted">
              {t("transfers.selectFromFirst")}
            </Text>
          )}
        </Stack>

        <Stack gap={1}>
          <Text fontSize="sm" fontWeight="medium" color="fg.muted">
            {t("transfers.note")}
          </Text>
          <Input value={note} onChange={(e) => setNote(e.target.value)} />
        </Stack>
      </Stack>
    </EntityDrawer>
  );
}
