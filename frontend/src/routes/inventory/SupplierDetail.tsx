import { useEffect, useMemo, useState } from "react";
import { Badge, Box, Button, Heading, HStack, Input, SimpleGrid, Spinner, Stack, Table, Tabs, Text } from "@chakra-ui/react";
import { Archive, ArchiveRestore, Pencil, Plus, Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import ConfirmDialog from "../../components/ConfirmDialog";
import PageHeader from "../../components/PageHeader";
import Pagination from "../../components/Pagination";
import { PriceAgreement } from "../../gen/inventory_iface/v1/price_agreement_pb";
import { formatDiscount, formatMoney, formatUnixOrDash } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useProductRefs } from "../../queries/refs";
import { useArchivePriceAgreementMutation, usePriceAgreementsQuery } from "../../queries/priceAgreements";
import {
  useArchiveSupplierMutation,
  useSupplierQuery,
  useSupplierRestocksQuery,
  useUnarchiveSupplierMutation,
} from "../../queries/suppliers";
import PriceAgreementDrawer from "./PriceAgreementDrawer";
import { EditSupplierDrawer } from "./supplierDrawers";

export default function SupplierDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const supQ = useSupplierQuery(id);
  const archive = useArchiveSupplierMutation();
  const unarchive = useUnarchiveSupplierMutation();
  const [editing, setEditing] = useState(false);
  const [pendingArchive, setPendingArchive] = useState(false);
  const [pendingUnarchive, setPendingUnarchive] = useState(false);
  const [searchInput, setSearchInput] = useState("");
  const [query, setQuery] = useState("");
  // Debounce the search box (250ms) into the query that drives the request.
  useEffect(() => {
    const h = setTimeout(() => setQuery(searchInput.trim()), 250);
    return () => clearTimeout(h);
  }, [searchInput]);
  const { page, setPage, pageSize, setPageSize } = usePageState(`${id}|${query}`);
  const restocksQ = useSupplierRestocksQuery(id, { query, page, pageSize, enabled: !!id });
  const productRefs = useProductRefs(
    useMemo(() => restocksQ.rows.map((r) => r.productId), [restocksQ.rows]),
  );

  if (supQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const sup = supQ.data;
  if (!sup) {
    return (
      <Box p={8}>
        <Text color="fg.muted">{t("common.noResults")}</Text>
      </Box>
    );
  }

  // Archive/unarchive in place — query invalidation flips the badge + button.
  const onArchive = async () => {
    try {
      await archive.mutateAsync({ id: sup.id });
      toast.success(t("common.archive") + " ✓");
    } catch {
      /* toast handled globally */
    } finally {
      setPendingArchive(false);
    }
  };
  const onUnarchive = async () => {
    try {
      await unarchive.mutateAsync({ id: sup.id });
      toast.success(t("common.unarchive") + " ✓");
    } catch {
      /* collision (supplier.name_taken) auto-toasted globally */
    } finally {
      setPendingUnarchive(false);
    }
  };

  return (
    <Box>
      <BackButton to="/inventory/suppliers" />
      <PageHeader
        breadcrumbs={[{ label: t("inventory.suppliers.title"), to: "/inventory/suppliers" }, { label: sup.name }]}
        title={sup.name}
        actions={
          <HStack>
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil size={14} />
              {t("common.edit")}
            </Button>
            {sup.active ? (
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
            {t("inventory.suppliers.infoSection")}
          </Heading>
          <SimpleGrid columns={{ base: 2, md: 4 }} gap={3}>
            <Field label={t("inventory.suppliers.code")} value={sup.code} mono />
            <Field label={t("inventory.suppliers.email")} value={sup.contactEmail || "—"} />
            <Field label={t("inventory.suppliers.phone")} value={sup.phone || "—"} />
            <Field label={t("inventory.suppliers.address")} value={sup.address || "—"} />
            <Field label={t("inventory.suppliers.bankName")} value={sup.bankName || "—"} />
            <Field label={t("inventory.suppliers.bankAccountNumber")} value={sup.bankAccountNumber || "—"} mono />
            <Field label={t("inventory.suppliers.bankAccountHolder")} value={sup.bankAccountHolder || "—"} />
            <Box>
              <Text fontSize="xs" color="fg.muted" mb={1}>
                {t("common.active")}
              </Text>
              <Badge colorPalette={sup.active ? "green" : "gray"}>
                {sup.active ? t("common.active") : t("common.inactive")}
              </Badge>
            </Box>
          </SimpleGrid>
        </Box>

        <Tabs.Root defaultValue="restocks" variant="line">
          <Tabs.List>
            <Tabs.Trigger value="restocks">{t("inventory.suppliers.restockSection")}</Tabs.Trigger>
            <Tabs.Trigger value="agreements">{t("inventory.priceAgreements.supplierSection")}</Tabs.Trigger>
          </Tabs.List>

          <Tabs.Content value="restocks">
            <HStack justify="flex-end" mb={3} wrap="wrap" gap={2}>
              <Box position="relative">
                <Box position="absolute" left={2} top="50%" transform="translateY(-50%)" color="fg.muted">
                  <Search size={14} />
                </Box>
                <Input
                  size="sm"
                  pl={7}
                  width="240px"
                  placeholder={t("inventory.suppliers.restockSearchPlaceholder")}
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                />
              </Box>
            </HStack>
            <Box overflowX="auto">
              <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
                <Table.Header bg="bg.muted">
                  <Table.Row>
                    <Table.ColumnHeader>{t("inventory.suppliers.restockProduct")}</Table.ColumnHeader>
                    <Table.ColumnHeader textAlign="end">{t("inventory.products.lastRestockPrice")}</Table.ColumnHeader>
                    <Table.ColumnHeader textAlign="end">{t("inventory.products.lastRestockQty")}</Table.ColumnHeader>
                    <Table.ColumnHeader textAlign="end">{t("inventory.products.lastRestockDiscount")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.products.lastRestockCreated")}</Table.ColumnHeader>
                    <Table.ColumnHeader>{t("inventory.products.lastRestockArrived")}</Table.ColumnHeader>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {restocksQ.rows.map((r) => (
                    <Table.Row key={r.productId}>
                      <Table.Cell>{productRefs.get(r.productId)?.name ?? "—"}</Table.Cell>
                      <Table.Cell textAlign="end">{formatMoney(r.lastPrice)}</Table.Cell>
                      <Table.Cell textAlign="end">{r.lastQty.toString()}</Table.Cell>
                      <Table.Cell textAlign="end">{formatDiscount(r.lastDiscountType, r.lastDiscountValue)}</Table.Cell>
                      <Table.Cell>{formatUnixOrDash(r.lastCreatedAt)}</Table.Cell>
                      <Table.Cell>{formatUnixOrDash(r.lastArrivedAt)}</Table.Cell>
                    </Table.Row>
                  ))}
                  {restocksQ.rows.length === 0 && (
                    <Table.Row>
                      <Table.Cell colSpan={6}>
                        <Text color="fg.muted" textAlign="center" py={4}>
                          {t("inventory.suppliers.restockEmpty")}
                        </Text>
                      </Table.Cell>
                    </Table.Row>
                  )}
                </Table.Body>
              </Table.Root>
            </Box>
            <Pagination
              page={page}
              pageSize={pageSize}
              total={restocksQ.total}
              onPageChange={setPage}
              onPageSizeChange={setPageSize}
            />
          </Tabs.Content>

          <Tabs.Content value="agreements">
            <PriceAgreementsSection supplierId={id} />
          </Tabs.Content>
        </Tabs.Root>
      </Stack>

      <EditSupplierDrawer supplier={editing ? sup : null} onClose={() => setEditing(false)} />
      <ConfirmDialog
        open={pendingArchive}
        title={t("common.archive")}
        body={t("inventory.suppliers.confirmArchive")}
        confirmLabel={t("common.archive")}
        loading={archive.isPending}
        onConfirm={onArchive}
        onCancel={() => setPendingArchive(false)}
      />
      <ConfirmDialog
        open={pendingUnarchive}
        title={t("common.unarchive")}
        body={t("inventory.suppliers.confirmUnarchive")}
        confirmLabel={t("common.unarchive")}
        confirmColorPalette="green"
        loading={unarchive.isPending}
        onConfirm={onUnarchive}
        onCancel={() => setPendingUnarchive(false)}
      />
    </Box>
  );
}

// That supplier's price agreements (reuses the shared drawer with the supplier
// locked). Its own pagination so it doesn't fight the restock section's.
function PriceAgreementsSection({ supplierId }: { supplierId: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { page, setPage, pageSize, setPageSize } = usePageState(`pa|${supplierId}`);
  const q = usePriceAgreementsQuery({ supplierId, page, pageSize });
  const archive = useArchivePriceAgreementMutation();
  const productRefs = useProductRefs(useMemo(() => q.rows.map((r) => r.productId), [q.rows]));
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<PriceAgreement | null>(null);
  const [pendingArchive, setPendingArchive] = useState<PriceAgreement | null>(null);

  return (
    <Box>
      <HStack justify="flex-end" mb={3} wrap="wrap" gap={2}>
        <Button
          size="sm"
          colorPalette="blue"
          onClick={() =>
            navigate(`/inventory/price-agreements/new?supplier=${supplierId}&returnTo=supplier`)
          }
        >
          <Plus size={16} />
          {t("inventory.priceAgreements.addTitle")}
        </Button>
      </HStack>
      <Box overflowX="auto">
        <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>{t("inventory.priceAgreements.product")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.priceAgreements.unit")}</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">{t("inventory.priceAgreements.price")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("inventory.priceAgreements.valid")}</Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {q.rows.map((a) => (
              <Table.Row key={a.id}>
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
                <Table.Cell>
                  <HStack gap={1}>
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => {
                        setEditing(a);
                        setDrawerOpen(true);
                      }}
                    >
                      <Pencil size={14} />
                    </Button>
                    <Button size="xs" variant="ghost" colorPalette="red" onClick={() => setPendingArchive(a)}>
                      <Archive size={14} />
                    </Button>
                  </HStack>
                </Table.Cell>
              </Table.Row>
            ))}
            {q.rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={5}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("inventory.priceAgreements.empty")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      </Box>
      <Pagination page={page} pageSize={pageSize} total={q.total} onPageChange={setPage} onPageSizeChange={setPageSize} />

      <PriceAgreementDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        editing={editing}
      />
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
