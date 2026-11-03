import { useEffect, useState } from "react";
import {
  Box,
  Button,
  HStack,
  Input,
  SimpleGrid,
  Spinner,
  Stack,
  StackSeparator,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { Archive, ArchiveRestore, Pencil, Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ConfirmDialog from "../../components/ConfirmDialog";
import ManufacturerListItemMobile, {
  ManufacturerListItemMobileSkeleton,
} from "../../components/manufacturer/ManufacturerListItemMobile";
import Pagination from "../../components/Pagination";
import SummaryTile from "../../components/SummaryTile";
import TableScroll from "../../components/TableScroll";
import type { Manufacturer } from "../../gen/inventory_iface/v1/manufacturer_pb";
import {
  useArchiveManufacturerMutation,
  useManufacturersQuery,
  useManufacturersSummaryQuery,
  useUnarchiveManufacturerMutation,
} from "../../queries/manufacturers";
import { formatCount } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { CreateManufacturerDrawer, EditManufacturerDrawer } from "./manufacturerDrawers";

// Pabrik — who MADE the goods, as opposed to the pemasok who sold them here.
// A flat admin list: search + archived toggle + create/edit/archive. There is
// a detail page behind every row (what this pabrik makes), which is also where
// Edit and Archive live once the screen is too narrow for an actions column.
export default function Manufacturers() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [includeInactive, setIncludeInactive] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Manufacturer | null>(null);
  // The row awaiting an archive/unarchive confirmation (ConfirmDialog's
  // `pending` pattern — the app never uses window.confirm).
  const [pending, setPending] = useState<Manufacturer | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(`${query}|${includeInactive}`);
  const listQ = useManufacturersQuery({ includeInactive, query, page, pageSize });
  // Count over EVERY matching manufacturer (server-side aggregate), not a count
  // of the page — same filters as the list, minus paging, so the tile keeps
  // describing the table under it as the user pages.
  const summaryQ = useManufacturersSummaryQuery({ includeInactive, query });

  const archive = useArchiveManufacturerMutation();
  const unarchive = useUnarchiveManufacturerMutation();
  const pendingArchives = pending?.active ?? true;
  const confirmBusy = archive.isPending || unarchive.isPending;

  const runPending = () => {
    if (!pending) return;
    const mutation = pending.active ? archive : unarchive;
    mutation.mutate({ id: pending.id }, { onSettled: () => setPending(null) });
  };

  return (
    <Stack gap={4}>
      {/* Summarizes the whole page, so it sits above the toolbar rather than
          inside it. One tile today; the grid is the 4-up row the other list
          pages use, so a second figure drops in without a re-layout. */}
      <SimpleGrid columns={{ base: 1, sm: 2, lg: 4 }} gap={3}>
        <SummaryTile
          label={t("inventory.manufacturers.summary.total")}
          value={formatCount(summaryQ.data?.totalManufacturers ?? 0n)}
        />
      </SimpleGrid>

      <HStack justify="space-between" wrap="wrap" gap={2}>
        {/* Wraps and goes full-width on a phone: a fixed-width search box beside
            the switch is what pushes a 390px viewport sideways. */}
        <HStack gap={3} wrap="wrap" flex="1 1 auto">
          <Box position="relative" width={{ base: "full", sm: "280px" }}>
            <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={14} />
            </Box>
            <Input
              size="sm"
              pl={7}
              placeholder={t("inventory.manufacturers.searchPlaceholder")}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </Box>
          <Switch.Root checked={includeInactive} onCheckedChange={(d) => setIncludeInactive(d.checked)}>
            <Switch.HiddenInput />
            <Switch.Control />
            <Switch.Label>{t("common.showArchived")}</Switch.Label>
          </Switch.Root>
        </HStack>
        <Button size="sm" colorPalette="blue" onClick={() => setCreateOpen(true)}>
          <Plus size={16} />
          {t("inventory.manufacturers.addTitle")}
        </Button>
      </HStack>

      {listQ.isLoading ? (
        <>
          {/* A phone gets the shape of the rows it is about to show rather than
              a spinner alone in the middle of the screen. */}
          <Box hideBelow="md" p={8} textAlign="center">
            <Spinner />
          </Box>
          <Stack hideFrom="md" gap={0} separator={<StackSeparator />}>
            {Array.from({ length: 6 }, (_, i) => (
              <ManufacturerListItemMobileSkeleton key={i} chevron />
            ))}
          </Stack>
        </>
      ) : (
        <>
          {/* Two layouts, switched purely by CSS breakpoint like AppShell: md+
              gets the column table, a phone gets one tappable row per pabrik
              (eight columns don't fit at 390px). The rows come AFTER the table
              in the DOM so a desktop `getByText(...).first()` still lands on
              the visible table. */}
          <Box hideBelow="md">
            <TableScroll>
              <Table.Root size="sm" stickyHeader>
                <Table.Header bg="bg.muted">
                  <Table.Row>
                    <Table.ColumnHeader>{t("inventory.manufacturers.code")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.manufacturers.name")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.manufacturers.phone")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.manufacturers.email")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.manufacturers.address")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.manufacturers.note")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {listQ.rows.map((m) => (
                    <Row key={m.id} manufacturer={m} onEdit={setEditing} onToggleArchive={setPending} />
                  ))}
                  {listQ.rows.length === 0 && (
                    <Table.Row>
                      <Table.Cell colSpan={8}>
                        <Text color="fg.muted" textAlign="center" py={4}>
                          {query || includeInactive
                            ? t("common.noResults")
                            : t("inventory.manufacturers.empty")}
                        </Text>
                      </Table.Cell>
                    </Table.Row>
                  )}
                </Table.Body>
              </Table.Root>
            </TableScroll>
          </Box>
          <Stack hideFrom="md" gap={0} separator={<StackSeparator />}>
            {listQ.rows.map((m) => (
              <ManufacturerListItemMobile
                key={m.id}
                manufacturer={m}
                onClick={() => navigate(`/inventory/manufacturers/${m.id}`)}
              />
            ))}
            {listQ.rows.length === 0 && (
              <Text color="fg.muted" textAlign="center" py={4}>
                {query || includeInactive
                  ? t("common.noResults")
                  : t("inventory.manufacturers.empty")}
              </Text>
            )}
          </Stack>
        </>
      )}

      <Pagination
        page={page}
        pageSize={pageSize}
        total={listQ.total}
        loading={listQ.isPlaceholderData}
        onPageChange={setPage}
        onPageSizeChange={setPageSize}
      />

      <CreateManufacturerDrawer open={createOpen} onClose={() => setCreateOpen(false)} />
      <EditManufacturerDrawer manufacturer={editing} onClose={() => setEditing(null)} />

      {/* Rendered unconditionally and driven by `open` — unmounting an open
          Chakra dialog leaves the body pointer-events lock in place. */}
      <ConfirmDialog
        open={pending != null}
        title={
          pendingArchives ? t("common.archive") : t("common.unarchive")
        }
        body={
          pendingArchives
            ? t("inventory.manufacturers.confirmArchive")
            : t("inventory.manufacturers.confirmUnarchive")
        }
        confirmLabel={pendingArchives ? t("common.archive") : t("common.unarchive")}
        confirmColorPalette={pendingArchives ? "red" : "blue"}
        loading={confirmBusy}
        onConfirm={runPending}
        onCancel={() => setPending(null)}
      />
    </Stack>
  );
}

function Row({
  manufacturer,
  onEdit,
  onToggleArchive,
}: {
  manufacturer: Manufacturer;
  onEdit: (m: Manufacturer) => void;
  onToggleArchive: (m: Manufacturer) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <Table.Row
      opacity={manufacturer.active ? 1 : 0.6}
      cursor="pointer"
      _hover={{ bg: "bg.muted" }}
      onClick={() => navigate(`/inventory/manufacturers/${manufacturer.id}`)}
    >
      <Table.Cell fontFamily="mono">{manufacturer.code}</Table.Cell>
      <Table.Cell>{manufacturer.name}</Table.Cell>
      <Table.Cell>{manufacturer.phone || "—"}</Table.Cell>
      <Table.Cell>{manufacturer.contactEmail || "—"}</Table.Cell>
      <Table.Cell>{manufacturer.address || "—"}</Table.Cell>
      <Table.Cell color="fg.muted">{manufacturer.note || "—"}</Table.Cell>
      <Table.Cell>{manufacturer.active ? t("common.yes") : t("common.no")}</Table.Cell>
      <Table.Cell>
        <HStack gap={1}>
          <Button
            size="xs"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation();
              onEdit(manufacturer);
            }}
          >
            <Pencil size={14} />
            {t("common.edit")}
          </Button>
          <Button
            size="xs"
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation();
              onToggleArchive(manufacturer);
            }}
          >
            {manufacturer.active ? <Archive size={14} /> : <ArchiveRestore size={14} />}
            {manufacturer.active ? t("common.archive") : t("common.unarchive")}
          </Button>
        </HStack>
      </Table.Cell>
    </Table.Row>
  );
}
