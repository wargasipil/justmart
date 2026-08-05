import { type ReactNode } from "react";
import { HStack, Stack, Text } from "@chakra-ui/react";

import { type GroupSpec } from "../../components/ColumnsPopover";
import ProductImage from "../../components/ProductImage";
import { Product } from "../../gen/inventory_iface/v1/product_pb";
import { downloadCsv } from "../../lib/csv";
import { formatDiscount, formatMoney, formatUnixOrDash } from "../../lib/format";
import { formatStock } from "../../lib/stockUnit";
import { fetchProductsForExport } from "../../queries/products";

// The Products list's column registry and its CSV export. Both are flat
// declarations keyed off the same field ids, so they live together and away from
// the page — which is then just filters + table + pager.

export type ProductCol = {
  id: string;
  header: string;
  alignEnd?: boolean;
  always?: boolean;
  render: (m: Product) => ReactNode;
};

// The whole restock group is purchase data (what we paid, to whom, when), so it
// drops out for the till — from the table AND from the Columns popover, or the
// popover would offer toggles for columns that can never render.
const RESTOCK_COL_IDS = [
  "lastRestockPrice",
  "lastRestockQty",
  "lastRestockDiscount",
  "lastRestockCreated",
  "lastRestockArrived",
  "lastSupplier",
];

type SupplierRefs = { get(id: string): { name: string } | undefined };

type ColumnArgs = {
  t: (k: string) => string;
  /** Off for the till: the restock (cost) group is omitted entirely. */
  showCost: boolean;
  /** Ids the user has ticked in the Columns popover. */
  visibleSet: Set<string>;
  /** Per-base-unit display choice from the Units popover. */
  stockUnitsByBase: Record<string, string>;
  supplierRefs: SupplierRefs;
};

// Column registry, in display order. `always` columns ignore the visibility
// toggle (name = identity). Restock columns render "—" until a receipt records
// one (lastRestockArrivedAt is the "has restock" sentinel).
export function buildProductColumns({
  t,
  showCost,
  visibleSet,
  stockUnitsByBase,
  supplierRefs,
}: ColumnArgs): { cols: ProductCol[]; columnGroups: GroupSpec[] } {
  const stockCell = (qty: bigint, m: Product) =>
    formatStock(qty, m.units, m.unit, stockUnitsByBase);

  const allCols: ProductCol[] = [
    // Identity cell: photo + name + SKU read as one thing (which product is
    // this?), so they share a column rather than competing as three. Always
    // shown and not toggleable — a row with its identity hidden is unusable.
    {
      id: "name",
      header: t("inventory.products.name"),
      always: true,
      render: (m) => (
        <HStack gap={3}>
          <ProductImage
            productId={m.id}
            name={m.name}
            version={Number(m.imageUpdatedAt)}
            size={56}
            zoomable
          />
          <Stack gap={0} minW={0}>
            <Text>{m.name}</Text>
            <Text fontSize="xs" color="fg.muted" fontFamily="mono">
              {m.sku}
            </Text>
          </Stack>
        </HStack>
      ),
    },
    { id: "unit", header: t("inventory.products.unit"), render: (m) => m.unit },
    {
      id: "unitPrice",
      header: t("inventory.products.unitPrice"),
      render: (m) => formatMoney(m.unitPrice),
    },
    {
      id: "ready",
      header: t("inventory.products.readyStock"),
      alignEnd: true,
      render: (m) => stockCell(m.readyStock, m),
    },
    {
      id: "onOrder",
      header: t("inventory.products.onOrder"),
      alignEnd: true,
      render: (m) => (m.onOrderStock > 0n ? stockCell(m.onOrderStock, m) : "—"),
    },
    {
      id: "lastStocktake",
      header: t("inventory.products.lastStocktake"),
      render: (m) => m.lastStocktakeDate || "—",
    },
    {
      id: "lastRestockPrice",
      header: t("inventory.products.lastRestockPrice"),
      alignEnd: true,
      render: (m) =>
        m.lastRestockArrivedAt > 0n ? formatMoney(m.lastRestockPrice) : "—",
    },
    {
      id: "lastRestockQty",
      header: t("inventory.products.lastRestockQty"),
      alignEnd: true,
      render: (m) =>
        m.lastRestockArrivedAt > 0n ? m.lastRestockQty.toString() : "—",
    },
    {
      id: "lastRestockDiscount",
      header: t("inventory.products.lastRestockDiscount"),
      alignEnd: true,
      render: (m) =>
        m.lastRestockArrivedAt > 0n
          ? formatDiscount(
              m.lastRestockDiscountType,
              m.lastRestockDiscountValue,
            )
          : "—",
    },
    {
      id: "lastRestockCreated",
      header: t("inventory.products.lastRestockCreated"),
      render: (m) => formatUnixOrDash(m.lastRestockCreatedAt),
    },
    {
      id: "lastRestockArrived",
      header: t("inventory.products.lastRestockArrived"),
      render: (m) => formatUnixOrDash(m.lastRestockArrivedAt),
    },
    {
      id: "lastSupplier",
      header: t("inventory.products.lastSupplier"),
      render: (m) =>
        m.lastRestockSupplierId
          ? (supplierRefs.get(m.lastRestockSupplierId)?.name ?? "—")
          : "—",
    },
  ];

  const cols = allCols.filter(
    (c) =>
      (c.always || visibleSet.has(c.id)) &&
      (showCost || !RESTOCK_COL_IDS.includes(c.id)),
  );

  const columnGroups: GroupSpec[] = [
    {
      id: "standard",
      label: t("inventory.products.colGroupStandard"),
      fields: [
        // No `image` / `sku` entries: both are folded into the always-on
        // identity column, so there is nothing to toggle.
        { id: "unit", label: t("inventory.products.unit") },
        { id: "unitPrice", label: t("inventory.products.unitPrice") },
        { id: "ready", label: t("inventory.products.readyStock") },
        { id: "onOrder", label: t("inventory.products.onOrder") },
        { id: "lastStocktake", label: t("inventory.products.lastStocktake") },
      ],
    },
    ...(showCost
      ? [
          {
            id: "restock",
            label: t("inventory.products.colGroupRestock"),
            fields: [
              {
                id: "lastRestockPrice",
                label: t("inventory.products.lastRestockPrice"),
              },
              {
                id: "lastRestockQty",
                label: t("inventory.products.lastRestockQty"),
              },
              {
                id: "lastRestockDiscount",
                label: t("inventory.products.lastRestockDiscount"),
              },
              {
                id: "lastRestockCreated",
                label: t("inventory.products.lastRestockCreated"),
              },
              {
                id: "lastRestockArrived",
                label: t("inventory.products.lastRestockArrived"),
              },
              {
                id: "lastSupplier",
                label: t("inventory.products.lastSupplier"),
              },
            ],
          },
        ]
      : []),
  ];

  return { cols, columnGroups };
}

// Downloads every row matching the active filters (not just the page). Carries
// the restock/cost columns, so the button that calls it is manager-only.
export async function exportProductsCsv({
  t,
  query,
  opnameBefore,
  onlyArchived,
  supplierRefs,
}: {
  t: (k: string) => string;
  query: string;
  opnameBefore: string;
  onlyArchived: boolean;
  supplierRefs: SupplierRefs;
}) {
  const rows = await fetchProductsForExport({
    query,
    opnameBefore,
    onlyArchived,
  });
  downloadCsv(
    `products-${new Date().toISOString().slice(0, 10)}.csv`,
    rows.map((m) => ({
      sku: m.sku,
      name: m.name,
      unit: m.unit,
      unitPrice: Number(m.unitPrice),
      ready: Number(m.readyStock),
      onOrder: Number(m.onOrderStock),
      lastOpname: m.lastStocktakeDate || "",
      lastRestockPrice:
        m.lastRestockArrivedAt > 0n ? Number(m.lastRestockPrice) : "",
      lastRestockQty:
        m.lastRestockArrivedAt > 0n ? Number(m.lastRestockQty) : "",
      lastRestockDiscount:
        m.lastRestockArrivedAt > 0n
          ? formatDiscount(m.lastRestockDiscountType, m.lastRestockDiscountValue)
          : "",
      lastRestockArrived: formatUnixOrDash(m.lastRestockArrivedAt),
      lastSupplier: m.lastRestockSupplierId
        ? (supplierRefs.get(m.lastRestockSupplierId)?.name ?? "")
        : "",
    })),
    [
      { key: "sku", header: t("inventory.products.sku"), text: true },
      { key: "name", header: t("inventory.products.name") },
      { key: "unit", header: t("inventory.products.unit") },
      { key: "unitPrice", header: t("inventory.products.unitPrice") },
      { key: "ready", header: t("inventory.products.readyStock") },
      { key: "onOrder", header: t("inventory.products.onOrder") },
      { key: "lastOpname", header: t("inventory.products.lastStocktake") },
      {
        key: "lastRestockPrice",
        header: t("inventory.products.lastRestockPrice"),
      },
      {
        key: "lastRestockQty",
        header: t("inventory.products.lastRestockQty"),
      },
      {
        key: "lastRestockDiscount",
        header: t("inventory.products.lastRestockDiscount"),
      },
      {
        key: "lastRestockArrived",
        header: t("inventory.products.lastRestockArrived"),
      },
      { key: "lastSupplier", header: t("inventory.products.lastSupplier") },
    ],
  );
}
