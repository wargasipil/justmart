import type { Meta, StoryObj } from "@storybook/react";
import { expect, fn, userEvent, within } from "storybook/test";

import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { StockMovementService } from "../../gen/inventory_iface/v1/stock_connect";
import { StockLevel } from "../../gen/inventory_iface/v1/stock_pb";
import { saveShowOutOfStock } from "../../lib/posStorage";
import { useBusinessMode } from "../../queries/settings";
import { DESKTOP, DESKTOP_PAGE } from "../../screens/viewports";
import { dateIn, makeProduct } from "../dev/fixtures";
import { withBusinessMode } from "../dev/storyDecorators";
import { mockApi, mockRpc } from "../dev/storyMocks";
import PosSearchPanel from "./PosSearchPanel";
import { usePosSearch } from "./usePosSearch";

// The POS search panel on the bench, driven by the REAL usePosSearch hook over
// mocked ListProducts + GetStockLevels, so the stock figure on each row is the
// one the cashier would see: per-lot levels summed by sumStockByProduct, the
// expiry rule from lib/expiry.ts (isExpiredOn).
//
// In pharmacy mode a lot whose expiry date is before today cannot be sold —
// CompleteSale skips it and refuses the sale with sale.stock_expired — so the
// row shows only the SELLABLE stock plus an "N kedaluwarsa" hint. Retail sells
// every lot and renders exactly as before.
//
// Page-local JSX: no /components gallery entry, the prose lives here. The lot
// dates are relative (dateIn), so the stories never age into a different state.

const tablet = { unit: "tablet", price: 1_000n, cost: 700n };
const PARTLY = makeProduct({ id: "pos-partly", sku: "PCT-500", name: "Paracetamol 500 mg", ...tablet });
const ALL_EXPIRED = makeProduct({ id: "pos-expired", sku: "AMB-30", name: "Ambroxol 30 mg", ...tablet });
const TODAY = makeProduct({ id: "pos-today", sku: "CTM-4", name: "CTM 4 mg", ...tablet });
const CATALOG: Product[] = [PARTLY, ALL_EXPIRED, TODAY];

const lot = (product: Product, n: number, expiryDate: string, qty: bigint) =>
  new StockLevel({ batchId: `${product.id}-lot${n}`, productId: product.id, expiryDate, currentQuantity: qty });

const LEVELS: StockLevel[] = [
  // 4 expired three days ago + 6 good for another quarter.
  lot(PARTLY, 1, dateIn(-3), 4n),
  lot(PARTLY, 2, dateIn(90), 6n),
  // Everything left expired yesterday.
  lot(ALL_EXPIRED, 1, dateIn(-1), 10n),
  // Expires TODAY: still sellable until the day is over.
  lot(TODAY, 1, dateIn(0), 5n),
];

const handlers = mockApi(
  mockRpc(ProductService, "listProducts", () => ({ products: CATALOG, total: CATALOG.length })),
  mockRpc(StockMovementService, "getStockLevels", () => ({ levels: LEVELS })),
);

/** The panel wired the way routes/Pos.tsx wires it (minus the cart). */
function PosSearchBench({ onAdd }: { onAdd: (product: Product, unitId: string, available?: number) => void }) {
  const { isPharmacy } = useBusinessMode();
  const s = usePosSearch({ onAdd });
  return (
    <PosSearchPanel
      searchRef={s.searchRef}
      query={s.query}
      setQuery={s.setQuery}
      onSearchKeyDown={s.onSearchKeyDown}
      showOutOfStock={s.showOutOfStock}
      onToggleOutOfStock={s.onToggleOutOfStock}
      unitRows={s.unitRows}
      highlight={s.highlight}
      setHighlight={s.setHighlight}
      isPharmacy={isPharmacy}
      hasPrescription={false}
      onAdd={onAdd}
    />
  );
}

const meta = {
  title: "components/pos/PosSearchPanel",
  component: PosSearchBench,
  args: { onAdd: fn() },
  globals: DESKTOP,
  parameters: { ...DESKTOP_PAGE, msw: handlers },
  // The out-of-stock toggle persists per device (localStorage). Start every
  // story from the default (shown) and put it back after one that flips it.
  beforeEach: () => {
    saveShowOutOfStock(true);
    return () => saveShowOutOfStock(true);
  },
} satisfies Meta<typeof PosSearchBench>;

export default meta;

type Story = StoryObj<typeof meta>;

/** The stock line under a row's name: "<sku> · <qty> <unit>[ · <hint>]". */
const stockLine = (text: string) => (_: string, el: Element | null) =>
  el?.tagName === "P" && el.textContent === text;

export const PharmacyExpiredStock: Story = {
  decorators: [withBusinessMode("pharmacy")],
  parameters: {
    docs: {
      description: {
        story:
          "Paracetamol has 10 tablets on hand, 4 of them in a lot that expired three days ago: " +
          "the row offers the 6 it can sell and says 4 are expired. Ambroxol's only lot expired " +
          "yesterday, so it reads 0 and behaves like any out-of-stock product — dimmed, not " +
          "addable, hidden by the toggle. CTM's lot expires today and is still sold in full.",
      },
    },
  },
  play: async ({ canvasElement, args }) => {
    const canvas = within(canvasElement);
    const partly = await canvas.findByText(stockLine("PCT-500 · 6 tablet · 4 kedaluwarsa"));
    const expired = canvas.getByText(stockLine("AMB-30 · 0 tablet · 10 kedaluwarsa"));
    await expect(canvas.getByText(stockLine("CTM-4 · 5 tablet"))).toBeInTheDocument();

    // All expired = out of stock: clicking it adds nothing.
    await userEvent.click(expired);
    await expect(args.onAdd).not.toHaveBeenCalled();
    // Partly expired: added with the SELLABLE count, not the shelf total.
    await userEvent.click(partly);
    await expect(args.onAdd).toHaveBeenCalledWith(
      expect.objectContaining({ id: PARTLY.id }),
      `${PARTLY.id}-u1`,
      6,
    );

    // Hiding out-of-stock rows hides the all-expired product with them.
    await userEvent.click(canvas.getByText("Tampilkan stok habis"));
    await expect(canvas.queryByText(stockLine("AMB-30 · 0 tablet · 10 kedaluwarsa"))).toBeNull();
    await expect(canvas.getByText(stockLine("PCT-500 · 6 tablet · 4 kedaluwarsa"))).toBeInTheDocument();
  },
};

export const RetailUnchanged: Story = {
  decorators: [withBusinessMode("retail")],
  parameters: {
    docs: {
      description: {
        story:
          "The same lots in a retail shop: every lot is sold (FEFO, oldest first), so each row " +
          "shows the whole shelf and there is no expired hint.",
      },
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText(stockLine("PCT-500 · 10 tablet"))).toBeInTheDocument();
    await expect(canvas.getByText(stockLine("AMB-30 · 10 tablet"))).toBeInTheDocument();
    await expect(canvas.getByText(stockLine("CTM-4 · 5 tablet"))).toBeInTheDocument();
    await expect(canvas.queryByText(/kedaluwarsa/)).toBeNull();
  },
};
