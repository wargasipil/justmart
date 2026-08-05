import { useState } from "react";
import {
  Badge,
  Box,
  Button,
  HStack,
  SimpleGrid,
  Spinner,
  Stack,
  Tabs,
  Text,
} from "@chakra-ui/react";
import { Archive, ArchiveRestore, Pencil } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import { useCrumbLabel } from "../../lib/breadcrumbs";
import BackButton from "../../components/BackButton";
import ProductImage from "../../components/ProductImage";
import ProductImagePicker from "./ProductImagePicker";
import ConfirmDialog from "../../components/ConfirmDialog";
import PageHeader from "../../components/PageHeader";
import { useAuth } from "../../lib/auth";
import { formatMoney } from "../../lib/format";
import { canSeeCost } from "../../lib/roles";
import { toast } from "../../lib/toaster";
import {
  useArchiveProductMutation,
  useProductQuery,
  useUnarchiveProductMutation,
} from "../../queries/products";
import { EditProductDialog } from "./productDrawers";
import { Card, Field, Tile, UnitsCard } from "./productDetailCards";
import ProductBatchesTab from "./ProductBatchesTab";
import DiscountTab from "./ProductDiscountTab";
import GrosirPanel from "./ProductGrosirPanel";
import ProductMovementsTab from "./ProductMovementsTab";
import ProductPriceHistoryTab from "./ProductPriceHistoryTab";
import ProductRestockTab from "./ProductRestockTab";

function fmtVariance(v: bigint): string {
  if (v === 0n) return "±0";
  return v > 0n ? `+${v.toString()}` : v.toString();
}

export default function ProductDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const { user } = useAuth();
  // The till reads this page: identity, sell price, stock and expiry only. Every
  // surface below that shows cost — or is backed by a manager-only RPC — is
  // dropped rather than rendered empty.
  const showCost = canSeeCost(user?.role);
  const [editing, setEditing] = useState(false);
  const [pendingArchive, setPendingArchive] = useState(false);
  const [pendingUnarchive, setPendingUnarchive] = useState(false);

  const medQ = useProductQuery(id);
  useCrumbLabel(medQ.data?.name);
  const archive = useArchiveProductMutation();
  const unarchive = useUnarchiveProductMutation();
  // Each tab owns its own query and pager (keyed by product id, so navigating to
  // a different product resets it rather than landing on page 3 of a shorter
  // list) — the page itself only fetches the product.

  if (medQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const med = medQ.data;
  if (!med) {
    return (
      <Box p={8}>
        <Text color="fg.muted">{t("common.noResults")}</Text>
      </Box>
    );
  }

  const onArchive = async () => {
    try {
      await archive.mutateAsync({ id: med.id });
      toast.success(t("common.archive") + " ✓");
      setPendingArchive(false);
      navigate("/products");
    } catch {
      /* toast handled globally */
    }
  };

  const onUnarchive = async () => {
    try {
      await unarchive.mutateAsync({ id: med.id });
      toast.success(t("inventory.products.unarchive") + " ✓");
      setPendingUnarchive(false);
      navigate("/products");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <BackButton to="/products" />
      <PageHeader
        title={med.name}
        description={t("inventory.products.detailDescription")}
        titleBadge={
          <Badge colorPalette={med.active ? "green" : "gray"}>
            {med.active ? t("common.active") : t("common.inactive")}
          </Badge>
        }
        actions={
          !showCost ? undefined : (
            <HStack>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setEditing(true)}
              >
                <Pencil size={14} />
                {t("common.edit")}
              </Button>
              {med.active ? (
                <Button
                  size="sm"
                  variant="outline"
                  colorPalette="red"
                  onClick={() => setPendingArchive(true)}
                >
                  <Archive size={14} />
                  {t("common.archive")}
                </Button>
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  colorPalette="green"
                  onClick={() => setPendingUnarchive(true)}
                >
                  <ArchiveRestore size={14} />
                  {t("inventory.products.unarchive")}
                </Button>
              )}
            </HStack>
          )
        }
      />

      <Stack gap={6}>
        {/* Info and unit pricing sit side by side: they answer two different
            questions ("what/how much do I hold" vs "what do I sell it for"),
            and pairing them keeps both above the fold. Stacks on narrow. */}
        <SimpleGrid columns={{ base: 1, lg: 2 }} gap={6} alignItems="start">
          {/* Left column = what this product IS: the photo identifies it at a
              glance, then the facts. Card vocabulary shared with the tab strip
              below: a bg.subtle body under a bg.muted header bar. */}
          <Stack gap={6}>
            <Card title={t("inventory.products.imageSection")}>
              <Box p={4}>
                {showCost ? (
                  <ProductImagePicker
                    productId={med.id}
                    name={med.name}
                    version={Number(med.imageUpdatedAt)}
                  />
                ) : (
                  // Same picture, no Change/Remove: the upload + delete RPCs are
                  // manager-only. `full` reads the ORIGINAL — the one deliberate
                  // full-size surface, matching the picker.
                  <Stack align="center">
                    <ProductImage
                      productId={med.id}
                      name={med.name}
                      version={Number(med.imageUpdatedAt)}
                      size={200}
                      full
                    />
                  </Stack>
                )}
              </Box>
            </Card>
            <Card title={t("inventory.products.infoSection")}>
              <Box p={4}>
                <SimpleGrid columns={{ base: 2, xl: 3 }} gap={3}>
                  <Field
                    label={t("inventory.products.sku")}
                    value={med.sku}
                    mono
                  />
                  <Field
                    label={t("inventory.products.unit")}
                    value={med.unit}
                  />
                  <Field
                    label={t("inventory.products.unitPrice")}
                    value={formatMoney(med.unitPrice)}
                  />
                  {showCost && (
                    <>
                      <Field
                        label={t("inventory.products.lastCost")}
                        value={
                          med.referenceCost > 0n
                            ? formatMoney(med.referenceCost)
                            : "—"
                        }
                      />
                      <Field
                        label={t("inventory.products.lastRestock")}
                        value={
                          med.lastRestockDate
                            ? med.lastRestockDate +
                              (med.lastRestockSupplier
                                ? ` · ${med.lastRestockSupplier}`
                                : "")
                            : "—"
                        }
                      />
                    </>
                  )}
                  <Field
                    label={t("inventory.products.lastStocktake")}
                    value={
                      med.lastStocktakeDate
                        ? `${med.lastStocktakeDate} · ${fmtVariance(med.lastStocktakeVariance)}`
                        : "—"
                    }
                  />
                  {/* Each stock tile carries its own valuation, so "how much do I
                    hold / have coming" reads as one figure pair instead of a
                    count here and a lone total elsewhere. */}
                  {/* The counts stay for the till; only the valuation under
                      them drops away. */}
                  <Tile
                    label={t("inventory.products.readyStock")}
                    value={med.readyStock.toString()}
                    sub={showCost ? formatMoney(med.stockValuation) : undefined}
                    palette="blue"
                  />
                  <Tile
                    label={t("inventory.products.onOrder")}
                    value={
                      med.onOrderStock > 0n ? med.onOrderStock.toString() : "—"
                    }
                    sub={
                      showCost && med.onOrderStock > 0n
                        ? formatMoney(med.onOrderValuation)
                        : undefined
                    }
                    palette="orange"
                  />
                </SimpleGrid>
              </Box>
            </Card>
          </Stack>

          {/* Right column = the two pricing cards, stacked. Grosir follows
              Satuan because a tier prices ONE unit: the ladder is only readable
              directly under the unit list it keys off. */}
          <Stack gap={6}>
            <UnitsCard product={med} showCost={showCost} />
            {/* Grosir is a manager surface: it prices against cost (margin per
                rung) and its list/create/delete RPCs are manager-only. The till
                sees the ladder where it matters — POS, at the moment of sale. */}
            {showCost && (
              <Card title={t("priceTiers.section")}>
                <Box p={4}>
                  <GrosirPanel product={med} />
                </Box>
              </Card>
            )}
          </Stack>
        </SimpleGrid>

        {/* Batches / Price history / Movements as tabs. The card wraps the whole
            strip so the triggers and the active panel read as one surface — the
            panel tables therefore drop their own bg/border (they'd be a card in
            a card) and each scrolls in its own container, since the card clips. */}
        <Box
          bg="bg.subtle"
          borderWidth="1px"
          borderRadius="lg"
          overflow="hidden"
        >
          <Tabs.Root defaultValue="batches" variant="line">
            <Tabs.List bg="bg.muted" px={2}>
              <Tabs.Trigger value="batches">
                {t("inventory.products.batchesSection")}
              </Tabs.Trigger>
              {/* The other four tabs are all manager-only reads (price history,
                  restock log, the stock ledger, discount rules), so the till
                  gets the batches tab alone — the one that answers "what do we
                  actually have on the shelf, and when does it expire". */}
              {showCost && (
                <>
                  <Tabs.Trigger value="prices">
                    {t("inventory.products.soldPriceHistory")}
                  </Tabs.Trigger>
                  <Tabs.Trigger value="restocks">
                    {t("inventory.products.restockPriceHistory")}
                  </Tabs.Trigger>
                  <Tabs.Trigger value="movements">
                    {t("inventory.products.movementsSection")}
                  </Tabs.Trigger>
                  <Tabs.Trigger value="discount">
                    {t("productDiscounts.section")}
                  </Tabs.Trigger>
                </>
              )}
            </Tabs.List>

            <Tabs.Content value="batches" p={4}>
              <ProductBatchesTab productId={id} showCost={showCost} />
            </Tabs.Content>

            {/* Rendered, not just untriggered: Chakra keeps inactive panels
                mounted, so leaving these in would fire their manager-only
                queries for a cashier the moment the page loads. */}
            {showCost && (
              <>
                <Tabs.Content value="prices" p={4}>
                  <ProductPriceHistoryTab productId={id} />
                </Tabs.Content>
                <Tabs.Content value="restocks" p={4}>
                  <ProductRestockTab productId={id} />
                </Tabs.Content>
                <Tabs.Content value="movements" p={4}>
                  <ProductMovementsTab productId={id} />
                </Tabs.Content>
                <Tabs.Content value="discount" p={4}>
                  <DiscountTab productId={id} />
                </Tabs.Content>
              </>
            )}
          </Tabs.Root>
        </Box>
      </Stack>

      <EditProductDialog
        product={editing ? med : null}
        onClose={() => setEditing(false)}
      />

      <ConfirmDialog
        open={pendingArchive}
        title={t("common.archive")}
        body={t("inventory.products.confirmArchive")}
        confirmLabel={t("common.archive")}
        loading={archive.isPending}
        onConfirm={onArchive}
        onCancel={() => setPendingArchive(false)}
      />
      <ConfirmDialog
        open={pendingUnarchive}
        title={t("inventory.products.unarchive")}
        body={t("inventory.products.confirmUnarchive")}
        confirmLabel={t("inventory.products.unarchive")}
        confirmColorPalette="green"
        loading={unarchive.isPending}
        onConfirm={onUnarchive}
        onCancel={() => setPendingUnarchive(false)}
      />
    </Box>
  );
}
