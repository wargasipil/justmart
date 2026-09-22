import { useEffect, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Heading,
  HStack,
  Input,
  SimpleGrid,
  Spinner,
  Stack,
  Switch,
  Table,
  Text,
} from "@chakra-ui/react";
import { Archive, ArchiveRestore, Pencil, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import ConfirmDialog from "../../components/ConfirmDialog";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { useCrumbLabel } from "../../lib/breadcrumbs";
import { formatCount, formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import {
  useArchiveManufacturerMutation,
  useManufacturerProductsQuery,
  useManufacturerQuery,
  useUnarchiveManufacturerMutation,
} from "../../queries/manufacturers";
import { EditManufacturerDrawer } from "./manufacturerDrawers";

// One pabrik: its contact details, and the catalog it makes.
//
// The product list is the reason this page exists rather than being an edit
// drawer — it is the one thing the row cannot show. Same shape as
// SupplierDetail's restock section (the maker's counterpart to the seller's).
export default function ManufacturerDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const mfrQ = useManufacturerQuery(id);
  // Fills the TopBar trail's leaf. Called before any early return — it's a
  // hook — and `undefined` while loading OMITS the crumb rather than flashing
  // a raw UUID.
  useCrumbLabel(mfrQ.data?.name);

  const archive = useArchiveManufacturerMutation();
  const unarchive = useUnarchiveManufacturerMutation();
  const [editing, setEditing] = useState(false);
  const [pendingArchive, setPendingArchive] = useState(false);
  const [pendingUnarchive, setPendingUnarchive] = useState(false);
  const [includeArchived, setIncludeArchived] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");

  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(`${id}|${query}|${includeArchived}`);
  const productsQ = useManufacturerProductsQuery(id, {
    query,
    includeArchived,
    page,
    pageSize,
    enabled: !!id,
  });

  if (mfrQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const mfr = mfrQ.data;
  if (!mfr) {
    return (
      <Box p={8}>
        <BackButton to="/inventory/manufacturers" />
        <Text color="fg.muted">{t("common.noResults")}</Text>
      </Box>
    );
  }

  // Archive/unarchive in place — query invalidation flips the badge + button.
  const onArchive = async () => {
    try {
      await archive.mutateAsync({ id: mfr.id });
      toast.success(t("common.archive") + " ✓");
    } catch {
      /* toast handled globally */
    } finally {
      setPendingArchive(false);
    }
  };
  const onUnarchive = async () => {
    try {
      await unarchive.mutateAsync({ id: mfr.id });
      toast.success(t("common.unarchive") + " ✓");
    } catch {
      /* collision (manufacturer.name_taken) auto-toasted globally */
    } finally {
      setPendingUnarchive(false);
    }
  };

  return (
    <Box>
      <BackButton to="/inventory/manufacturers" />
      <PageHeader
        title={mfr.name}
        description={t("inventory.manufacturers.detailDescription")}
        actions={
          <HStack>
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil size={14} />
              {t("common.edit")}
            </Button>
            {mfr.active ? (
              <Button size="sm" variant="outline" colorPalette="red" onClick={() => setPendingArchive(true)}>
                <Archive size={14} />
                {t("common.archive")}
              </Button>
            ) : (
              <Button size="sm" variant="outline" colorPalette="green" onClick={() => setPendingUnarchive(true)}>
                <ArchiveRestore size={14} />
                {t("common.unarchive")}
              </Button>
            )}
          </HStack>
        }
      />

      <Stack gap={6}>
        <Box>
          <Heading size="sm" mb={3}>
            {t("inventory.manufacturers.infoSection")}
          </Heading>
          <SimpleGrid columns={{ base: 2, md: 4 }} gap={3}>
            <Field label={t("inventory.manufacturers.code")} value={mfr.code} mono />
            <Field label={t("inventory.manufacturers.phone")} value={mfr.phone || "—"} />
            <Field label={t("inventory.manufacturers.email")} value={mfr.contactEmail || "—"} />
            <Field label={t("inventory.manufacturers.address")} value={mfr.address || "—"} />
            <Field label={t("inventory.manufacturers.note")} value={mfr.note || "—"} />
            <Box>
              <Text fontSize="xs" color="fg.muted" mb={1}>
                {t("common.active")}
              </Text>
              <Badge colorPalette={mfr.active ? "green" : "gray"}>
                {mfr.active ? t("common.active") : t("common.inactive")}
              </Badge>
            </Box>
          </SimpleGrid>
        </Box>

        <Box>
          <HStack justify="space-between" mb={3} wrap="wrap" gap={2}>
            <Heading size="sm">{t("inventory.manufacturers.productsSection")}</Heading>
            <HStack gap={3} wrap="wrap">
              <Switch.Root
                checked={includeArchived}
                onCheckedChange={(d) => setIncludeArchived(d.checked)}
              >
                <Switch.HiddenInput />
                <Switch.Control />
                <Switch.Label>{t("common.showArchived")}</Switch.Label>
              </Switch.Root>
              <Box position="relative" width={{ base: "full", sm: "240px" }}>
                <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
                  <Search size={14} />
                </Box>
                <Input
                  size="sm"
                  pl={7}
                  placeholder={t("inventory.manufacturers.productsSearchPlaceholder")}
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                />
              </Box>
            </HStack>
          </HStack>

          <TableScroll maxH={TABLE_MAX_H_NESTED}>
            <Table.Root size="sm" stickyHeader>
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("inventory.manufacturers.productName")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.products.sku")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">
                    {t("inventory.manufacturers.productPrice")}
                  </Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">
                    {t("inventory.manufacturers.productReady")}
                  </Table.ColumnHeader>
                  <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {productsQ.rows.map((p) => (
                  <Table.Row key={p.productId} opacity={p.active ? 1 : 0.6}>
                    <Table.Cell>{p.name}</Table.Cell>
                    <Table.Cell fontFamily="mono">{p.sku}</Table.Cell>
                    <Table.Cell textAlign="end">{formatMoney(p.unitPrice)}</Table.Cell>
                    <Table.Cell textAlign="end">
                      {formatCount(p.readyStock)}
                      {p.baseUnit ? ` ${p.baseUnit}` : ""}
                    </Table.Cell>
                    <Table.Cell>{p.active ? t("common.yes") : t("common.no")}</Table.Cell>
                  </Table.Row>
                ))}
                {productsQ.rows.length === 0 && (
                  <Table.Row>
                    <Table.Cell colSpan={5}>
                      <Text color="fg.muted" textAlign="center" py={4}>
                        {query ? t("common.noResults") : t("inventory.manufacturers.productsEmpty")}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                )}
              </Table.Body>
            </Table.Root>
          </TableScroll>

          <Pagination
            page={page}
            pageSize={pageSize}
            total={productsQ.total}
            loading={productsQ.isPlaceholderData}
            onPageChange={setPage}
            onPageSizeChange={setPageSize}
          />
        </Box>
      </Stack>

      <EditManufacturerDrawer manufacturer={editing ? mfr : null} onClose={() => setEditing(false)} />

      {/* Both rendered unconditionally and driven by `open` — unmounting an
          open Chakra dialog leaves the body pointer-events lock in place. */}
      <ConfirmDialog
        open={pendingArchive}
        title={t("common.archive")}
        body={t("inventory.manufacturers.confirmArchive")}
        confirmLabel={t("common.archive")}
        loading={archive.isPending}
        onConfirm={onArchive}
        onCancel={() => setPendingArchive(false)}
      />
      <ConfirmDialog
        open={pendingUnarchive}
        title={t("common.unarchive")}
        body={t("inventory.manufacturers.confirmUnarchive")}
        confirmLabel={t("common.unarchive")}
        confirmColorPalette="blue"
        loading={unarchive.isPending}
        onConfirm={onUnarchive}
        onCancel={() => setPendingUnarchive(false)}
      />
    </Box>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily={mono ? "mono" : undefined}>{value}</Text>
    </Box>
  );
}
