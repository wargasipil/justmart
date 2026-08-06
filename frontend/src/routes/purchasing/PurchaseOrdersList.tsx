import {
  Badge,
  Box,
  Button,
  HStack,
  Input,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { Plus, Search } from "lucide-react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import DateRangeFilter from "../../components/DateRangeFilter";
import ExportButton from "../../components/ExportButton";
import Pagination from "../../components/Pagination";
import SupplierSelect, { supplierLabel } from "../../components/SupplierSelect";
import TableScroll from "../../components/TableScroll";
import {
  POStatus,
  type PurchaseOrder,
} from "../../gen/purchasing_iface/v1/order_pb";
import { downloadCsv } from "../../lib/csv";
import { formatMoney, formatDate } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { fmtUnitQty } from "../../lib/purchaseLine";
import { fetchPurchaseOrdersForExport, usePurchaseOrdersQuery } from "../../queries/purchasing";
import { resolveSupplierMap, useSupplierRefs } from "../../queries/refs";
import { restockPageKey, useRestockFilters } from "./restockFilters";

const STATUS_BADGE_PALETTE: Record<POStatus, string> = {
  [POStatus.PO_STATUS_UNSPECIFIED]: "gray",
  [POStatus.PO_STATUS_DRAFT]: "gray",
  [POStatus.PO_STATUS_SENT]: "blue",
  [POStatus.PO_STATUS_PARTIALLY_RECEIVED]: "orange",
  [POStatus.PO_STATUS_RECEIVED]: "green",
  [POStatus.PO_STATUS_CLOSED]: "green",
  [POStatus.PO_STATUS_VOIDED]: "red",
};

export default function PurchaseOrdersList() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  // Filter state lives in the Purchasing shell: the stat row above the tab
  // strip and this toolbar below it are driven by the same values, and both
  // requests carry the same `request` object so they can't disagree.
  const {
    request,
    searchInput,
    setSearchInput,
    supplierId,
    setSupplierId,
    onlyOutstanding,
    setOnlyOutstanding,
    dateField,
    setDateField,
    range,
    setRange,
  } = useRestockFilters();

  const dateFields = [
    { value: "created", label: t("purchasing.dateCreated") },
    { value: "received", label: t("purchasing.dateReceived") },
  ];
  const { page, setPage, pageSize, setPageSize } = usePageState(restockPageKey(request));
  const posQ = usePurchaseOrdersQuery({
    ...request,
    limit: pageSize,
    offset: page * pageSize,
  });

  // Resolve supplier names for the current page's rows (resolve-by-IDs; not a
  // full supplier-list preload). <SupplierSelect> resolves its own trigger label.
  const supplierRefs = useSupplierRefs(
    useMemo(() => posQ.rows.map((po) => po.supplierId), [posQ.rows]),
  );

  const onExport = async () => {
    const rows = await fetchPurchaseOrdersForExport(request);
    const sup = await resolveSupplierMap(rows.map((po) => po.supplierId));
    downloadCsv(
      `restock-${new Date().toISOString().slice(0, 10)}.csv`,
      rows.map((po) => {
        return {
          poNo: po.poNo || po.id.slice(0, 8),
          supplier: supplierLabel(sup.get(po.supplierId)) ?? "—",
          items: po.items
            .map(
              (it) =>
                `${it.productName || it.productId.slice(0, 8)} ×${fmtUnitQty(it.orderedQty, it.unitName, it.unitFactor)}`,
            )
            .join("; "),
          status: t(`purchasing.states.${statusKey(po.status)}`),
          created: formatDate(new Date(Number(po.createdAt) * 1000)),
          received: po.receivedAt > 0n ? formatDate(new Date(Number(po.receivedAt) * 1000)) : "",
          invoiceNo: po.invoiceNo || "",
          total: Number(po.orderedTotal),
          outstanding: Number(po.outstanding),
        };
      }),
      [
        { key: "poNo", header: t("purchasing.poNo") },
        { key: "supplier", header: t("purchasing.supplier") },
        { key: "items", header: t("purchasing.item") },
        { key: "status", header: t("purchasing.status") },
        { key: "created", header: t("purchasing.created") },
        { key: "received", header: t("purchasing.received") },
        { key: "invoiceNo", header: t("purchasing.invoiceNo"), text: true },
        { key: "total", header: t("purchasing.totalOrdered") },
        { key: "outstanding", header: t("purchasing.outstanding") },
      ],
    );
  };

  return (
    // minW=0 so the 9-column table scrolls inside <TableScroll> rather than
    // widening this column and taking the page with it (see Purchasing.tsx).
    <Stack gap={4} minW={0}>
      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={2} wrap="wrap">
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="260px"
              placeholder={t("purchasing.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <SupplierSelect
            size="sm"
            width="220px"
            value={supplierId}
            onChange={setSupplierId}
            placeholder={t("purchasing.supplier")}
          />
          <DateRangeFilter
            value={range}
            onChange={setRange}
            fields={dateFields}
            field={dateField}
            onFieldChange={setDateField}
          />
          <Switch.Root checked={onlyOutstanding} onCheckedChange={(d) => setOnlyOutstanding(d.checked)}>
            <Switch.HiddenInput />
            <Switch.Control />
            <Switch.Label>{t("purchasing.outstandingToggle")}</Switch.Label>
          </Switch.Root>
        </HStack>
        <HStack gap={2}>
          <ExportButton onExport={onExport} />
          <Button colorPalette="blue" onClick={() => navigate("/purchasing/new")}>
            <Plus size={16} />
            {t("purchasing.newPo")}
          </Button>
        </HStack>
      </HStack>

      {posQ.isLoading ? (
        <Box p={6} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        // maxH="none": the restock table is not height-capped — it grows with
        // its rows and the page scrolls vertically. Horizontal overflow is
        // still the scroll area's own business (Chakra's max-width: 100%).
        <TableScroll maxH="none">
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("purchasing.poNo")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.supplier")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.item")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.status")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.created")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.received")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.invoiceNo")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.totalOrdered")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("purchasing.outstanding")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {posQ.rows.map((po: PurchaseOrder) => (
                <Table.Row
                  key={po.id}
                  onClick={() => navigate(`/purchasing/${po.id}`)}
                  cursor="pointer"
                  _hover={{ bg: "bg.muted" }}
                >
                  <Table.Cell fontFamily="mono">{po.poNo || po.id.slice(0, 8)}</Table.Cell>
                  <Table.Cell>{supplierLabel(supplierRefs.get(po.supplierId)) ?? "—"}</Table.Cell>
                  <Table.Cell>
                    <ItemsList po={po} moreLabel={t("purchasing.itemsMore")} />
                  </Table.Cell>
                  <Table.Cell>
                    <Badge colorPalette={STATUS_BADGE_PALETTE[po.status]}>
                      {t(`purchasing.states.${statusKey(po.status)}`)}
                    </Badge>
                  </Table.Cell>
                  <Table.Cell>{formatDate(new Date(Number(po.createdAt) * 1000))}</Table.Cell>
                  <Table.Cell>
                    {po.receivedAt > 0n ? formatDate(new Date(Number(po.receivedAt) * 1000)) : "—"}
                  </Table.Cell>
                  <Table.Cell fontFamily="mono">{po.invoiceNo || "—"}</Table.Cell>
                  <Table.Cell fontFamily="mono">{formatMoney(Number(po.orderedTotal))}</Table.Cell>
                  <Table.Cell fontFamily="mono">{formatMoney(Number(po.outstanding))}</Table.Cell>
                </Table.Row>
              ))}
              {posQ.rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={9}>
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
        total={posQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />
    </Stack>
  );
}

function ItemsList({ po, moreLabel }: { po: PurchaseOrder; moreLabel: string }) {
  if (po.items.length === 0) return <Text color="fg.muted">—</Text>;
  const shown = po.items.slice(0, 3);
  const extra = po.items.length - shown.length;
  return (
    <Stack gap={0}>
      {shown.map((it) => (
        <Text key={it.id} fontSize="xs">
          <Text as="span" fontWeight="semibold">
            {it.productName || it.productId.slice(0, 8)}
            {it.productSku ? ` (${it.productSku})` : ""}
          </Text>
          {" ×" + fmtUnitQty(it.orderedQty, it.unitName, it.unitFactor)}
        </Text>
      ))}
      {extra > 0 && (
        <Text fontSize="xs" color="fg.muted">
          {moreLabel.replace("{{count}}", String(extra))}
        </Text>
      )}
    </Stack>
  );
}

function statusKey(s: POStatus): string {
  switch (s) {
    case POStatus.PO_STATUS_DRAFT:
      return "draft";
    case POStatus.PO_STATUS_SENT:
      return "sent";
    case POStatus.PO_STATUS_PARTIALLY_RECEIVED:
      return "partiallyReceived";
    case POStatus.PO_STATUS_RECEIVED:
      return "received";
    case POStatus.PO_STATUS_CLOSED:
      return "closed";
    case POStatus.PO_STATUS_VOIDED:
      return "voided";
    default:
      return "draft";
  }
}
