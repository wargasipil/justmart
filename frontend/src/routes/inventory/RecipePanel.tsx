import { Badge, Box, Button, HStack, IconButton, Stack, Table, Text } from "@chakra-ui/react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import Pagination from "../../components/Pagination";
import { ProductKind, type Product } from "../../gen/inventory_iface/v1/product_pb";
import type { ProductRecipeItem } from "../../gen/inventory_iface/v1/product_recipe_pb";
import { usePageState } from "../../lib/pagination";
import {
  NO_RECIPE,
  useDeleteProductRecipeItemMutation,
  useProductRecipeQuery,
} from "../../queries/productRecipes";
import ProductRecipeDrawer from "./ProductRecipeDrawer";

// The bill of materials of a COMPOSITE product — what one of it is made from,
// and therefore what selling one deducts from stock.
//
// The headline is `buildable`: how many portions the current ingredient stock
// can produce, bounded by whichever ingredient runs out first. That is the
// question the card exists to answer, so it sits in the header rather than
// being something to work out from the rows.
export default function RecipePanel({ product }: { product: Product }) {
  const { t } = useTranslation();
  const isComposite = product.kind === ProductKind.COMPOSITE;
  const page = usePageState(`recipe:${product.id}`);
  const q = useProductRecipeQuery(product.id, {
    page: page.page,
    pageSize: page.pageSize,
    enabled: isComposite,
  });
  const del = useDeleteProductRecipeItemMutation();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<ProductRecipeItem | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ProductRecipeItem | null>(null);

  // A non-composite product has no recipe by definition — say why rather than
  // showing an empty table that looks broken.
  if (!isComposite) {
    return (
      <Box p={4}>
        <Text fontSize="sm" color="fg.muted">
          {t("inventory.products.recipe.onlyComposite")}
        </Text>
      </Box>
    );
  }

  const onConfirmDelete = async () => {
    if (!pendingDelete) return;
    try {
      await del.mutateAsync(pendingDelete.id);
      setPendingDelete(null);
    } catch {
      /* toast handled globally */
    }
  };

  const buildable = q.buildable;
  const buildableBadge =
    buildable === NO_RECIPE ? (
      <Badge colorPalette="gray">{t("inventory.products.recipe.buildableNoRecipe")}</Badge>
    ) : buildable === 0n ? (
      <Badge colorPalette="red">{t("inventory.products.recipe.buildableNone")}</Badge>
    ) : (
      <Badge colorPalette="green">
        {t("inventory.products.recipe.buildable", { count: Number(buildable) })}
      </Badge>
    );

  return (
    <Box p={4}>
      <Stack gap={3}>
        <HStack justify="space-between" align="start">
          <Stack gap={1}>
            <Text fontSize="sm" color="fg.muted">
              {t("inventory.products.recipe.description")}
            </Text>
            {buildableBadge}
          </Stack>
          <Button
            size="xs"
            colorPalette="blue"
            onClick={() => {
              setEditing(null);
              setDrawerOpen(true);
            }}
          >
            <Plus size={14} />
            {t("inventory.products.recipe.add")}
          </Button>
        </HStack>

        {q.rows.length === 0 ? (
          <Text fontSize="sm" color="fg.muted" py={4} textAlign="center">
            {t("inventory.products.recipe.empty")}
          </Text>
        ) : (
          <Table.Root size="sm">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.products.recipe.component")}</Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.recipe.qtyBase")}
                </Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">
                  {t("inventory.products.recipe.onHand")}
                </Table.ColumnHeader>
                <Table.ColumnHeader />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {q.rows.map((it) => {
                // An ingredient with less than one portion's worth is what makes
                // the whole dish unavailable — flag it on its own row so the fix
                // is obvious, not just in the aggregate badge above.
                const short = it.componentReady < it.qtyBase;
                return (
                  <Table.Row key={it.id}>
                    <Table.Cell>
                      <Stack gap={0}>
                        <Text fontSize="sm">{it.componentName || "—"}</Text>
                        {it.note && (
                          <Text fontSize="xs" color="fg.muted">
                            {it.note}
                          </Text>
                        )}
                      </Stack>
                    </Table.Cell>
                    <Table.Cell textAlign="end" fontFamily="mono" fontSize="sm">
                      {String(it.qtyBase)} {it.componentUnit}
                    </Table.Cell>
                    <Table.Cell textAlign="end" fontFamily="mono" fontSize="sm">
                      <Text color={short ? "red.fg" : undefined}>
                        {String(it.componentReady)} {it.componentUnit}
                      </Text>
                    </Table.Cell>
                    <Table.Cell textAlign="end">
                      <HStack gap={1} justify="end">
                        <IconButton
                          aria-label={t("common.edit")}
                          size="xs"
                          variant="ghost"
                          onClick={() => {
                            setEditing(it);
                            setDrawerOpen(true);
                          }}
                        >
                          <Pencil size={14} />
                        </IconButton>
                        <IconButton
                          aria-label={t("common.delete")}
                          size="xs"
                          variant="ghost"
                          onClick={() => setPendingDelete(it)}
                        >
                          <Trash2 size={14} />
                        </IconButton>
                      </HStack>
                    </Table.Cell>
                  </Table.Row>
                );
              })}
            </Table.Body>
          </Table.Root>
        )}

        <Pagination
          page={page.page}
          pageSize={page.pageSize}
          total={q.total}
          onPageChange={page.setPage}
          onPageSizeChange={page.setPageSize}
        />
      </Stack>

      <ProductRecipeDrawer
        productId={product.id}
        item={editing}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setEditing(null);
        }}
      />

      <ConfirmDialog
        open={pendingDelete != null}
        title={t("inventory.products.recipe.removeTitle")}
        body={t("inventory.products.recipe.removeConfirm", {
          name: pendingDelete?.componentName ?? "",
        })}
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        loading={del.isPending}
        onConfirm={onConfirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </Box>
  );
}
