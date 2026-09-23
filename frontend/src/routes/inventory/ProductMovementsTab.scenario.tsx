import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";

import { MovementType, StockMovement } from "../../gen/inventory_iface/v1/stock_pb";
import { StockMovementService } from "../../gen/inventory_iface/v1/stock_connect";
import { PHARMACY_CATALOG, daysAgo } from "../dev/fixtures";
import { movementsFor } from "../dev/productDetailFixtures";
import { mockApi, mockRpc, mockRpcError } from "../dev/storyMocks";
import ProductMovementsTab from "./ProductMovementsTab";

// The "Mutasi Terbaru" tab: this product's stock ledger in the ACTIVE
// warehouse. The ledger is insert-only and stock is the SUM of it, so every row
// here is a fact that already happened — nothing on this tab can be edited, and
// that is the point of showing it.
//
// Manager-only (ListMovements is OWNER+PHARMACIST), so no till story.
//
// Rendered by ProductMovementsTab.stories.tsx
// ("components/products/ProductMovementsTab").

const PRODUCT = PHARMACY_CATALOG[0];

const listMovements = (rows = movementsFor(PRODUCT)) =>
  mockRpc(StockMovementService, "listMovements", (req) => ({
    movements: rows.slice(req.offset, req.offset + (req.limit || 25)),
    total: rows.length,
  }));

// A month of trade: mostly sales, with a delivery and a correction in it.
const busyLedger = () => {
  const base = movementsFor(PRODUCT);
  return [
    ...Array.from({ length: 28 }, (_, i) =>
      new StockMovement({
        id: `${PRODUCT.id}-ms${i}`,
        batchId: base[0].batchId,
        qty: -((i % 6) + 1) * 2,
        type: MovementType.SALE,
        reason: `INV-2026-${String(400 - i).padStart(4, "0")}`,
        createdAt: daysAgo(i * 0.4),
      }),
    ),
    ...base.slice(1),
  ];
};

/** Everything but `title` and the device — the story file adds those. */
export const meta = {
  component: ProductMovementsTab,
  args: { productId: PRODUCT.id },
  parameters: { msw: mockApi(listMovements()) },
};

type Story = StoryObj<typeof ProductMovementsTab>;

const story = (description: string, s: Story = {}): Story => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

export const stories = {
  Manager: story(
    "One row per movement type the column can label — sale, purchase, adjustment, write-off — " +
      "rather than four sales, so the whole vocabulary is on screen at once. Quantities carry " +
      "their sign (+500 arrived, −20 sold) and the reason column names the document that " +
      "caused it: an invoice no, a receipt no, the opname session, the note someone typed.",
  ),
  Busy: story(
    "A month of real trade: a wall of small sales with one delivery and one correction buried " +
      "in it. This is what the tab actually looks like on a fast-moving product, and it is the " +
      "argument for the type column — the two rows that are not sales are the only ones anyone " +
      "is ever looking for.",
    { parameters: { msw: mockApi(listMovements(busyLedger())) } },
  ),
  Empty: story(
    "No movements in the active warehouse. Not the same as \"no stock ever\" — the ledger is " +
      "warehouse-scoped, so a product living entirely in another gudang reads exactly like " +
      "this. The pager stays and reports 0.",
    { parameters: { msw: mockApi(listMovements([])) } },
  ),
  Loading: story("ListMovements never answers: the empty table under a working pager.", {
    parameters: {
      msw: mockApi(
        mockRpc(StockMovementService, "listMovements", { movements: [], total: 0 }, { delay: "infinite" }),
      ),
    },
  }),
  LoadFailed: story(
    "The read fails: the table renders empty and the global QueryCache raises the toast. " +
      "Indistinguishable from Empty without it, which is exactly why the rule is global.",
    {
      parameters: {
        msw: mockApi(mockRpcError(StockMovementService, "listMovements", Code.Unavailable, "backend down")),
      },
    },
  ),
};
