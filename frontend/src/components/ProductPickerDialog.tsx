import {
  Box,
  Button,
  Checkbox,
  Dialog,
  HStack,
  Heading,
  IconButton,
  Input,
  InputGroup,
  Portal,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Search, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import Pagination from "./Pagination";
import ProductImage from "./ProductImage";
import TableScroll, { TABLE_MAX_H_NESTED } from "./TableScroll";
import type { Product } from "../gen/inventory_iface/v1/product_pb";
import { formatMoney } from "../lib/format";
import { usePageState } from "../lib/pagination";
import { useProductsQuery } from "../queries/products";

export type ProductPickerDialogProps = {
  open: boolean;
  /** Called on Cancel / Esc / backdrop. Does NOT commit the draft selection. */
  onClose: () => void;
  /**
   * Product ids the caller already holds. The dialog opens with these checked,
   * so the checkbox set IS the caller's list — unchecking one is how you remove
   * it.
   */
  selectedIds: readonly string[];
  /**
   * Commit. `ids` is the full checked set (order = the order they were picked,
   * with the pre-checked ones first). `byId` carries every Product the dialog
   * loaded this session — which necessarily includes every NEWLY checked id,
   * since you must see a row to check it. Pre-checked ids the user never paged
   * to may be absent; the caller already has those.
   */
  onConfirm: (ids: string[], byId: Map<string, Product>) => void;
  title?: string;
};

/**
 * Multi-select product picker rendered as a searchable, server-paginated modal.
 *
 * Reach for this when a page builds a LIST of products (restock lines, a return,
 * a price-agreement batch) rather than picking one — that's the split with
 * <SearchableSelect loadOptions={searchProducts}>, which stays the tool for a
 * single-value field.
 *
 * The check set is the caller's list: open pre-checked with what the caller
 * holds, confirm returns the whole set, and the caller reconciles (added ids →
 * new rows, dropped ids → removed rows, untouched ids keep their edits).
 */
export default function ProductPickerDialog({
  open,
  onClose,
  selectedIds,
  onConfirm,
  title,
}: ProductPickerDialogProps) {
  const { t } = useTranslation();
  const [rawQuery, setRawQuery] = useState("");
  const [query, setQuery] = useState("");
  const { page, setPage, pageSize, setPageSize } = usePageState(query);

  // Checked ids in pick order + every Product this session has rendered. The
  // map is what lets a product checked on page 1 survive paging to page 3: the
  // caller needs the Product object (for `units`), not just the id.
  const [checked, setChecked] = useState<string[]>([]);
  const seen = useRef(new Map<string, Product>());

  // Re-seed on each open so a cancelled edit never leaks into the next one.
  // The dialog stays MOUNTED while closed (Ark body-lock rule), so this is the
  // only reset point.
  useEffect(() => {
    if (!open) return;
    setChecked([...selectedIds]);
    setRawQuery("");
    setQuery("");
    setPage(0);
    // `selectedIds` is intentionally read only at open time — a caller that
    // re-renders mid-edit must not clobber the draft.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Debounce the search box (matches <SearchableSelect>'s 250ms).
  useEffect(() => {
    const id = setTimeout(() => setQuery(rawQuery.trim()), 250);
    return () => clearTimeout(id);
  }, [rawQuery]);

  const productsQ = useProductsQuery({ query, page, pageSize, enabled: open });
  const { rows } = productsQ;
  useEffect(() => {
    for (const p of rows) seen.current.set(p.id, p);
  }, [rows]);

  const checkedSet = useMemo(() => new Set(checked), [checked]);
  const toggle = (p: Product, on: boolean) => {
    seen.current.set(p.id, p);
    setChecked((cur) => (on ? [...cur, p.id] : cur.filter((id) => id !== p.id)));
  };

  const confirm = () => {
    onConfirm(checked, new Map(seen.current));
    onClose();
  };

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      size="lg"
      scrollBehavior="inside"
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header borderBottomWidth="1px">
              <HStack justify="space-between" w="full">
                <Heading size="md">{title ?? t("productPicker.title")}</Heading>
                <IconButton
                  aria-label={t("common.close")}
                  variant="ghost"
                  size="sm"
                  onClick={onClose}
                >
                  <X size={18} />
                </IconButton>
              </HStack>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <InputGroup startElement={<Search size={16} />}>
                  <Input
                    autoFocus
                    size="sm"
                    value={rawQuery}
                    onChange={(e) => setRawQuery(e.target.value)}
                    placeholder={t("productPicker.searchPlaceholder")}
                  />
                </InputGroup>

                {productsQ.isLoading ? (
                  <HStack justify="center" py={8}>
                    <Spinner size="sm" />
                  </HStack>
                ) : productsQ.rows.length === 0 ? (
                  <Text py={8} textAlign="center" color="fg.muted" fontSize="sm">
                    {t("productPicker.empty")}
                  </Text>
                ) : (
                  <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
                    <Table.Root size="sm" stickyHeader interactive>
                      <Table.Header>
                        <Table.Row>
                          <Table.ColumnHeader w="40px" />
                          <Table.ColumnHeader>{t("productPicker.product")}</Table.ColumnHeader>
                          <Table.ColumnHeader textAlign="end">
                            {t("productPicker.ready")}
                          </Table.ColumnHeader>
                          <Table.ColumnHeader textAlign="end">
                            {t("productPicker.lastCost")}
                          </Table.ColumnHeader>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {productsQ.rows.map((p) => {
                          const on = checkedSet.has(p.id);
                          return (
                            <Table.Row
                              key={p.id}
                              cursor="pointer"
                              bg={on ? "bg.muted" : undefined}
                              onClick={() => toggle(p, !on)}
                            >
                              <Table.Cell>
                                <Checkbox.Root
                                  size="sm"
                                  checked={on}
                                  // The row handles the toggle; this keeps the
                                  // box from firing it a second time.
                                  onClick={(e) => e.stopPropagation()}
                                  onCheckedChange={(d) => toggle(p, !!d.checked)}
                                >
                                  <Checkbox.HiddenInput />
                                  <Checkbox.Control />
                                </Checkbox.Root>
                              </Table.Cell>
                              <Table.Cell>
                                <HStack gap={2}>
                                  <ProductImage
                                    productId={p.id}
                                    name={p.name}
                                    version={Number(p.imageUpdatedAt)}
                                    size={28}
                                  />
                                  <Box minW={0}>
                                    <Text fontSize="sm" truncate>
                                      {p.name}
                                    </Text>
                                    <Text fontSize="xs" color="fg.muted" truncate>
                                      {p.sku}
                                      {p.unit ? ` · ${p.unit}` : ""}
                                    </Text>
                                  </Box>
                                </HStack>
                              </Table.Cell>
                              <Table.Cell textAlign="end" fontFamily="mono" fontSize="sm">
                                {Number(p.readyStock)}
                              </Table.Cell>
                              <Table.Cell
                                textAlign="end"
                                fontFamily="mono"
                                fontSize="sm"
                                color="fg.muted"
                              >
                                {Number(p.lastRestockPrice) > 0
                                  ? formatMoney(Number(p.lastRestockPrice))
                                  : "—"}
                              </Table.Cell>
                            </Table.Row>
                          );
                        })}
                      </Table.Body>
                    </Table.Root>
                  </TableScroll>
                )}

                <Pagination
                  page={page}
                  pageSize={pageSize}
                  total={productsQ.total}
                  onPageChange={setPage}
                  onPageSizeChange={setPageSize}
                />
              </Stack>
            </Dialog.Body>
            <Dialog.Footer borderTopWidth="1px">
              <HStack justify="space-between" w="full" gap={2}>
                <Text fontSize="sm" color="fg.muted">
                  {t("productPicker.selectedCount", { count: checked.length })}
                </Text>
                <HStack gap={2}>
                  <Button variant="ghost" onClick={onClose}>
                    {t("common.cancel")}
                  </Button>
                  <Button colorPalette="blue" onClick={confirm}>
                    {t("productPicker.confirm")}
                  </Button>
                </HStack>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
