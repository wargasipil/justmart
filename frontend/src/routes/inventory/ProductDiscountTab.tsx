import { useMemo, useState } from "react";
import { Badge, Box, Button, HStack, Table, Text } from "@chakra-ui/react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import ConfirmDialog from "../../components/ConfirmDialog";
import Pagination from "../../components/Pagination";
import TableScroll, { TABLE_MAX_H_NESTED } from "../../components/TableScroll";
import { ProductDiscount } from "../../gen/inventory_iface/v1/product_discount_pb";
import { formatMoney } from "../../lib/format";
import { usePageState } from "../../lib/pagination";
import {
  formatDiscountValue,
  useDeleteProductDiscountMutation,
  useProductDiscountsQuery,
} from "../../queries/productDiscounts";
import ProductDiscountDrawer from "./ProductDiscountDrawer";

// Local YYYY-MM-DD for the "expired" check (expires_at is a date string).
function todayStr() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// Per-product discount manager: the Product detail "Discount" tab.
export default function DiscountTab({ productId }: { productId: string }) {
  const { t } = useTranslation();
  const page = usePageState(`discounts:${productId}`);
  const q = useProductDiscountsQuery(productId, {
    page: page.page,
    pageSize: page.pageSize,
  });
  const del = useDeleteProductDiscountMutation();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<ProductDiscount | null>(null);
  const [pendingDelete, setPendingDelete] = useState<ProductDiscount | null>(
    null,
  );
  const today = useMemo(() => todayStr(), []);
  const rows = q.rows;

  const modeLabel = (d: ProductDiscount) => {
    if (d.perItem)
      return d.discountType === "PERCENT"
        ? t("productDiscounts.modePercentItem")
        : t("productDiscounts.modeFixedItem");
    return d.discountType === "PERCENT"
      ? t("productDiscounts.modePercent")
      : t("productDiscounts.modeFixed");
  };

  return (
    <Box>
      <HStack justify="flex-end" mb={3}>
        <Button
          size="sm"
          colorPalette="blue"
          onClick={() => {
            setEditing(null);
            setDrawerOpen(true);
          }}
        >
          <Plus size={16} />
          {t("productDiscounts.add")}
        </Button>
      </HStack>
      <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
        <Table.Root size="sm" stickyHeader>
          <Table.Header bg="bg.muted">
            <Table.Row>
              <Table.ColumnHeader>
                {t("productDiscounts.mode")}
              </Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">
                {t("productDiscounts.value")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>
                {t("productDiscounts.rule")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>
                {t("productDiscounts.expiresAt")}
              </Table.ColumnHeader>
              <Table.ColumnHeader>{t("common.actions")}</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {rows.map((d) => {
              const expired = d.expiresAt !== "" && d.expiresAt < today;
              return (
                <Table.Row key={d.id}>
                  <Table.Cell>{modeLabel(d)}</Table.Cell>
                  <Table.Cell textAlign="end" fontFamily="mono">
                    {formatDiscountValue(
                      d.discountType,
                      d.perItem,
                      d.value,
                      formatMoney,
                      "/item",
                    )}
                  </Table.Cell>
                  <Table.Cell color="fg.muted">
                    {d.minQty > 0 && d.minQtyUnitName
                      ? t("productDiscounts.minQtyRuleUnit", {
                          count: d.minQty,
                          unit: d.minQtyUnitName,
                        })
                      : d.minQty > 1
                        ? t("productDiscounts.minQtyRule", { count: d.minQty })
                        : t("productDiscounts.noRule")}
                  </Table.Cell>
                  <Table.Cell>
                    {d.expiresAt ? (
                      <HStack gap={2}>
                        <Text>{d.expiresAt}</Text>
                        {expired && (
                          <Badge colorPalette="red">
                            {t("productDiscounts.expired")}
                          </Badge>
                        )}
                      </HStack>
                    ) : (
                      <Text color="fg.muted">
                        {t("productDiscounts.noExpiry")}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    <HStack gap={1}>
                      <Button
                        size="xs"
                        variant="ghost"
                        onClick={() => {
                          setEditing(d);
                          setDrawerOpen(true);
                        }}
                      >
                        <Pencil size={14} />
                      </Button>
                      <Button
                        size="xs"
                        variant="ghost"
                        colorPalette="red"
                        onClick={() => setPendingDelete(d)}
                      >
                        <Trash2 size={14} />
                      </Button>
                    </HStack>
                  </Table.Cell>
                </Table.Row>
              );
            })}
            {rows.length === 0 && (
              <Table.Row>
                <Table.Cell colSpan={5}>
                  <Text color="fg.muted" textAlign="center" py={4}>
                    {t("productDiscounts.empty")}
                  </Text>
                </Table.Cell>
              </Table.Row>
            )}
          </Table.Body>
        </Table.Root>
      </TableScroll>

      <Pagination
        page={page.page}
        pageSize={page.pageSize}
        total={q.total}
        onPageChange={page.setPage}
        onPageSizeChange={page.setPageSize}
      />

      <ProductDiscountDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        productId={productId}
        editing={editing}
      />
      <ConfirmDialog
        open={pendingDelete != null}
        title={t("productDiscounts.deleteTitle")}
        body={t("productDiscounts.deleteBody")}
        confirmLabel={t("common.delete")}
        confirmColorPalette="red"
        loading={del.isPending}
        onConfirm={async () => {
          if (!pendingDelete) return;
          try {
            await del.mutateAsync(pendingDelete.id);
          } finally {
            setPendingDelete(null);
          }
        }}
        onCancel={() => setPendingDelete(null)}
      />
    </Box>
  );
}
