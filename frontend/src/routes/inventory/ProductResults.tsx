import { Box, Stack, StackSeparator, Table, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import ProductItemMobile from "../../components/products/ProductItemMobile";
import TableScroll from "../../components/TableScroll";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { formatStock, type StockUnitsByBase } from "../../lib/stockUnit";
import type { ProductCol } from "./productListColumns";

type Props = {
  rows: Product[];
  /** The visible table columns (buildProductColumns). */
  cols: ProductCol[];
  /** The Units popover's per-base choice — the phone rows honour it too. */
  stockUnitsByBase: StockUnitsByBase;
  onOpen: (id: string) => void;
};

// One page of the Products list, in whichever layout fits the screen.
export default function ProductResults({ rows, cols, stockUnitsByBase, onOpen }: Props) {
  const { t } = useTranslation();
  return (
    <>
      {/* Two layouts, switched purely by CSS breakpoint like AppShell:
          md+ gets the column table, a phone gets one tappable row per
          product (the columns don't fit at 390px). The rows come AFTER
          the table in the DOM so a desktop `getByText(...).first()`
          still lands on the visible table. */}
      <Box hideBelow="md">
        <TableScroll>
          <Table.Root size="sm" stickyHeader>
            <Table.Header bg="bg.muted">
              <Table.Row>
                {cols.map((c) => (
                  <Table.ColumnHeader
                    key={c.id}
                    textAlign={c.alignEnd ? "end" : undefined}
                  >
                    {c.header}
                  </Table.ColumnHeader>
                ))}
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {rows.map((m) => (
                <Table.Row
                  key={m.id}
                  cursor="pointer"
                  _hover={{ bg: "bg.muted" }}
                  onClick={() => onOpen(m.id)}
                >
                  {cols.map((c) => (
                    <Table.Cell
                      key={c.id}
                      textAlign={c.alignEnd ? "end" : undefined}
                    >
                      {c.render(m)}
                    </Table.Cell>
                  ))}
                </Table.Row>
              ))}
              {rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={cols.length}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      {t("common.noResults")}
                    </Text>
                  </Table.Cell>
                </Table.Row>
              )}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      </Box>
      <Stack hideFrom="md" gap={0} separator={<StackSeparator />}>
        {rows.map((m) => (
          <ProductItemMobile
            key={m.id}
            product={m}
            stockLabel={formatStock(m.readyStock, m.units, m.unit, stockUnitsByBase)}
            onClick={() => onOpen(m.id)}
          />
        ))}
        {rows.length === 0 && (
          <Text color="fg.muted" textAlign="center" py={4}>
            {t("common.noResults")}
          </Text>
        )}
      </Stack>
    </>
  );
}
