import { useEffect, useMemo, useState } from "react";
import { Box, Button, HStack, Input, Spinner, Stack, Switch, Table, Text } from "@chakra-ui/react";
import { Archive, ArchiveRestore, Pencil, Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import ConfirmDialog from "../../components/ConfirmDialog";
import EnumSelect from "../../components/EnumSelect";
import Pagination from "../../components/Pagination";
import PageHeader from "../../components/PageHeader";
import SearchableSelect from "../../components/SearchableSelect";
import SupplierSelect, { supplierLabel } from "../../components/SupplierSelect";
import TableScroll from "../../components/TableScroll";
import { PriceAgreement, PriceAgreementValidity } from "../../gen/inventory_iface/v1/price_agreement_pb";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import {
  usePriceAgreementsQuery,
  useArchivePriceAgreementMutation,
  useUnarchivePriceAgreementMutation,
} from "../../queries/priceAgreements";
import { useProductRefs, useSupplierRefs } from "../../queries/refs";
import { searchProducts } from "../../queries/products";
import PriceAgreementDrawer from "./PriceAgreementDrawer";

const VALIDITY_OPTIONS = [
  PriceAgreementValidity.UNSPECIFIED,
  PriceAgreementValidity.CURRENT,
  PriceAgreementValidity.EXPIRED,
  PriceAgreementValidity.UPCOMING,
] as const;

export default function PriceAgreements() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  const [supplierId, setSupplierId] = useState("");
  const [productId, setProductId] = useState("");
  const [includeInactive, setIncludeInactive] = useState(false);
  const [validity, setValidity] = useState<PriceAgreementValidity>(PriceAgreementValidity.UNSPECIFIED);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<PriceAgreement | null>(null);
  const [pendingArchive, setPendingArchive] = useState<PriceAgreement | null>(null);

  const validityLabel = (v: PriceAgreementValidity) => {
    switch (v) {
      case PriceAgreementValidity.CURRENT:
        return t("inventory.priceAgreements.validity.current");
      case PriceAgreementValidity.EXPIRED:
        return t("inventory.priceAgreements.validity.expired");
      case PriceAgreementValidity.UPCOMING:
        return t("inventory.priceAgreements.validity.upcoming");
      default:
        return t("inventory.priceAgreements.validity.all");
    }
  };

  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);

  const { page, setPage, pageSize, setPageSize } = usePageState(
    `${query}|${supplierId}|${productId}|${includeInactive}|${validity}`,
  );
  const q = usePriceAgreementsQuery({ query, supplierId, productId, includeInactive, validity, page, pageSize });
  const archive = useArchivePriceAgreementMutation();
  const unarchive = useUnarchivePriceAgreementMutation();

  // Table cells only — <SupplierSelect> resolves its own trigger label.
  const supplierRefs = useSupplierRefs(useMemo(() => q.rows.map((r) => r.supplierId), [q.rows]));
  const productRefs = useProductRefs(
    useMemo(() => {
      const ids = new Set<string>(q.rows.map((r) => r.productId));
      if (productId) ids.add(productId);
      return Array.from(ids);
    }, [q.rows, productId]),
  );

  const openEdit = (a: PriceAgreement) => {
    setEditing(a);
    setDrawerOpen(true);
  };

  return (
    <Box>
      <PageHeader
        title={t("inventory.priceAgreements.title")}
        description={t("inventory.priceAgreements.description")}
      />

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
                placeholder={t("inventory.priceAgreements.searchPlaceholder")}
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
              />
            </Box>
            <Box width="200px">
              <SupplierSelect
                size="sm"
                value={supplierId}
                onChange={setSupplierId}
                placeholder={t("inventory.priceAgreements.supplier")}
              />
            </Box>
            <Box width="200px">
              <SearchableSelect
                size="sm"
                value={productId}
                onChange={setProductId}
                loadOptions={searchProducts}
                itemToString={(m) => `${m.sku} · ${m.name}`}
                itemToValue={(m) => m.id}
                placeholder={t("inventory.priceAgreements.product")}
                selectedLabel={productId ? productRefs.get(productId)?.name : undefined}
              />
            </Box>
            <Box width="170px">
              <EnumSelect
                size="sm"
                value={String(validity)}
                onChange={(v) => setValidity(Number(v) as PriceAgreementValidity)}
                items={VALIDITY_OPTIONS}
                itemToString={validityLabel}
                itemToValue={(v) => String(v)}
                placeholder={t("inventory.priceAgreements.validity.label")}
              />
            </Box>
            <Switch.Root checked={includeInactive} onCheckedChange={(d) => setIncludeInactive(d.checked)}>
              <Switch.HiddenInput />
              <Switch.Control />
              <Switch.Label>{t("common.showArchived")}</Switch.Label>
            </Switch.Root>
          </HStack>
          <Button size="sm" colorPalette="blue" onClick={() => navigate("/inventory/price-agreements/new")}>
            <Plus size={16} />
            {t("inventory.priceAgreements.addTitle")}
          </Button>
        </HStack>

        {q.isLoading ? (
          <Box p={8} textAlign="center">
            <Spinner />
          </Box>
        ) : (
          <TableScroll>
            <Table.Root size="sm" stickyHeader>
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.supplier")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.product")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.unit")}</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">{t("inventory.priceAgreements.price")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.priceAgreements.valid")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("common.active")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {q.rows.map((a) => {
                  return (
                    <Table.Row key={a.id}>
                      <Table.Cell>{supplierLabel(supplierRefs.get(a.supplierId)) ?? "—"}</Table.Cell>
                      <Table.Cell>{productRefs.get(a.productId)?.name ?? "—"}</Table.Cell>
                      <Table.Cell>{a.unitName || "—"}</Table.Cell>
                      <Table.Cell textAlign="end" fontFamily="mono">
                        {formatMoney(Number(a.price))}
                        <Text as="span" fontSize="xs" color="fg.muted">
                          {" "}/ {a.unitName || "?"}
                        </Text>
                      </Table.Cell>
                      <Table.Cell color="fg.muted" fontSize="sm">
                        {a.validFrom || a.validUntil ? `${a.validFrom || "…"} → ${a.validUntil || "…"}` : "—"}
                      </Table.Cell>
                      <Table.Cell>{a.active ? t("common.yes") : t("common.no")}</Table.Cell>
                      <Table.Cell>
                        <HStack gap={1}>
                          <Button size="xs" variant="ghost" onClick={() => openEdit(a)}>
                            <Pencil size={14} />
                          </Button>
                          {a.active ? (
                            <Button size="xs" variant="ghost" colorPalette="red" onClick={() => setPendingArchive(a)}>
                              <Archive size={14} />
                            </Button>
                          ) : (
                            <Button
                              size="xs"
                              variant="ghost"
                              colorPalette="green"
                              loading={unarchive.isPending}
                              onClick={async () => {
                                try {
                                  await unarchive.mutateAsync({ id: a.id });
                                  toast.success(t("common.unarchive") + " ✓");
                                } catch {
                                  /* collision error auto-toasted by the global mutation handler */
                                }
                              }}
                            >
                              <ArchiveRestore size={14} />
                            </Button>
                          )}
                        </HStack>
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
                {q.rows.length === 0 && (
                  <Table.Row>
                    <Table.Cell colSpan={7}>
                      <Text color="fg.muted" textAlign="center" py={4}>
                        {t("inventory.priceAgreements.empty")}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                )}
              </Table.Body>
            </Table.Root>
          </TableScroll>
        )}

        <Pagination page={page} pageSize={pageSize} total={q.total} onPageChange={setPage} onPageSizeChange={setPageSize} />
      </Stack>

      <PriceAgreementDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} editing={editing} />
      <ConfirmDialog
        open={pendingArchive != null}
        title={t("inventory.priceAgreements.archiveTitle")}
        body={t("inventory.priceAgreements.archiveBody")}
        confirmLabel={t("common.archive")}
        loading={archive.isPending}
        onConfirm={async () => {
          if (!pendingArchive) return;
          try {
            await archive.mutateAsync({ id: pendingArchive.id });
          } finally {
            setPendingArchive(null);
          }
        }}
        onCancel={() => setPendingArchive(null)}
      />
    </Box>
  );
}
