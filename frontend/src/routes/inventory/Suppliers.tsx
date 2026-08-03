import { useEffect, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  SimpleGrid,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { Archive, ArchiveRestore, Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import Pagination from "../../components/Pagination";
import SummaryTile from "../../components/SummaryTile";
import TableScroll from "../../components/TableScroll";
import { Supplier } from "../../gen/inventory_iface/v1/supplier_pb";
import {
  useArchiveSupplierMutation,
  useSuppliersQuery,
  useSuppliersSummaryQuery,
  useUnarchiveSupplierMutation,
} from "../../queries/suppliers";
import { formatCount } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { CreateSupplierDrawer } from "./supplierDrawers";

export default function Suppliers() {
  const { t } = useTranslation();
  const [includeInactive, setIncludeInactive] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(`${query}|${includeInactive}`);
  const suppliersQ = useSuppliersQuery({ includeInactive, query, page, pageSize });
  // Count over EVERY matching supplier (server-side aggregate), not a count of
  // the page — same filters as the list, minus paging, so the tile keeps
  // describing the table under it as the user pages.
  const summaryQ = useSuppliersSummaryQuery({ includeInactive, query });

  return (
    <Stack gap={4}>
      {/* Summarizes the whole page, so it sits above the toolbar rather than
          inside it. One tile today; the grid is the 4-up row the other list
          pages use, so a second figure drops in without a re-layout. */}
      <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
        <SummaryTile
          label={t("inventory.suppliers.summary.total")}
          value={formatCount(summaryQ.data?.totalSuppliers ?? 0n)}
        />
      </SimpleGrid>

      <HStack justify="space-between" wrap="wrap" gap={2}>
        <HStack gap={3}>
          <Box position="relative">
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              width="280px"
              placeholder={t("inventory.suppliers.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <Switch.Root
            checked={includeInactive}
            onCheckedChange={(d) => setIncludeInactive(d.checked)}
          >
            <Switch.HiddenInput />
            <Switch.Control />
            <Switch.Label>{t("common.showArchived")}</Switch.Label>
          </Switch.Root>
        </HStack>
        <Button size="sm" colorPalette="blue" onClick={() => setDrawerOpen(true)}>
          <Plus size={16} />
          {t("inventory.suppliers.addTitle")}
        </Button>
      </HStack>

      {suppliersQ.isLoading ? (
        <Box p={8} textAlign="center">
          <Spinner />
        </Box>
      ) : (
        <TableScroll>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.suppliers.code")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.suppliers.name")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.suppliers.email")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.suppliers.phone")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.suppliers.address")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.suppliers.bankInfo")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {suppliersQ.rows.map((s) => (
                <Row key={s.id} supplier={s} />
              ))}
              {suppliersQ.rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={8}>
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
        total={suppliersQ.total}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateSupplierDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </Stack>
  );
}

function Row({ supplier }: { supplier: Supplier }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const archive = useArchiveSupplierMutation();
  const unarchive = useUnarchiveSupplierMutation();
  return (
    <Table.Row
      cursor="pointer"
      _hover={{ bg: "bg.muted" }}
      onClick={() => navigate(`/inventory/suppliers/${supplier.id}`)}
    >
      <Table.Cell fontFamily="mono">{supplier.code}</Table.Cell>
      <Table.Cell>{supplier.name}</Table.Cell>
      <Table.Cell>{supplier.contactEmail}</Table.Cell>
      <Table.Cell>{supplier.phone}</Table.Cell>
      <Table.Cell>{supplier.address}</Table.Cell>
      <Table.Cell>
        <RekeningCell supplier={supplier} />
      </Table.Cell>
      <Table.Cell>{supplier.active ? t("common.yes") : t("common.no")}</Table.Cell>
      <Table.Cell>
        {supplier.active ? (
          <Button
            size="xs"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation();
              archive.mutate({ id: supplier.id });
            }}
          >
            <Archive size={14} />
            {t("common.archive")}
          </Button>
        ) : (
          <Button
            size="xs"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation();
              unarchive.mutate({ id: supplier.id });
            }}
          >
            <ArchiveRestore size={14} />
            {t("common.unarchive")}
          </Button>
        )}
      </Table.Cell>
    </Table.Row>
  );
}

// Combined "Rekening" cell: bank · account number on the first line, account
// holder muted below. Degrades gracefully — joins only the present values, and
// shows a single "—" when no bank info exists at all.
function RekeningCell({ supplier }: { supplier: Supplier }) {
  const top = [supplier.bankName, supplier.bankAccountNumber].filter(Boolean).join(" · ");
  if (!top && !supplier.bankAccountHolder) {
    return <Text color="fg.muted">—</Text>;
  }
  return (
    <Stack gap={0}>
      {top && <Text>{top}</Text>}
      {supplier.bankAccountHolder && (
        <Text fontSize="xs" color="fg.muted">
          {supplier.bankAccountHolder}
        </Text>
      )}
    </Stack>
  );
}

