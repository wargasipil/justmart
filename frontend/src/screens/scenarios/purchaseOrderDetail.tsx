import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";

import { PurchaseOrderService } from "../../gen/purchasing_iface/v1/order_connect";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import PurchaseOrderDetail from "../../routes/purchasing/PurchaseOrderDetail";
import Purchasing from "../../routes/purchasing/Purchasing";
import { shopHandlers } from "./restockShop";

// One restock order's detail page — the biggest page in the app that had no
// story, and the one whose interesting states are all refusals. Rendered by
// screens/{desktop,mobile}/pages/purchasing/RestockOrder.stories.tsx; this
// module is the one place its states and their prose are written.
//
// Each story opens a DIFFERENT order rather than a different fixture of the
// same one, because the page is a state machine: which buttons exist is
// derived from the order's status, its outstanding balance and whether any
// delivered lot is still in stock. The seven orders come from
// routes/dev/fixtures.ts and are the seven positions that machine has.
//
// The mocks WRITE (see restockShop.ts), so the actions work: Send advances the
// order and swaps the action row, Receive lands a delivery, Pay closes a
// settled one, Cancel strikes a delivery through and takes the stock back.
//
// There is no cashier story: /purchasing is OWNER + PHARMACIST in the proto, so
// a till is never routed here.

// Mounted under the real <Purchasing> shell, which is what supplies the page's
// "Restock" header and — because a `:id` segment reads as a subpage — hides the
// status tabs and the stat row above them. The list routes are deliberately not
// registered here: this scenario is one order, and the round trip from the list
// and back is what scenarios/purchasing.tsx shows. So the Back button lands on
// the bare shell in these stories.
export const routes = (
  <Route path="/purchasing" element={<Purchasing />}>
    <Route path=":id" element={<PurchaseOrderDetail />} />
  </Route>
);

const at = (id: string) => ({ router: { initialEntries: [`/purchasing/${id}`] } });

/** Everything the page reads and writes, over a fresh copy of the ledger. */
export function detailHandlers() {
  return shopHandlers();
}

/** Everything but `title`, `render` and the device — the story files add those. */
export const meta = {
  component: PurchaseOrderDetail,
  parameters: {
    ...at("po-partial"),
    msw: mockApi(...detailHandlers()),
  },
  decorators: [withPageContext],
};

const story = (description: string, s: StoryObj = {}): StoryObj => ({
  ...s,
  parameters: { ...s.parameters, docs: { description: { story: description } } },
});

/**
 * A story's own copy of the ledger. Each one calls this, so an action taken in
 * one story (Send, Receive, Pay, Cancel) cannot leave the next story's order
 * already advanced.
 */
const withOwnShop = (extra: Record<string, unknown>): StoryObj["parameters"] => ({
  ...extra,
  msw: mockApi(...detailHandlers()),
});

export const stories = {
  Draft: story(
    "A restock order still being written: no PO has gone to the supplier yet, so the only " +
      "actions are Kirim (send it) and Batalkan (void it) — no receive, no payment. Press Kirim " +
      "and the order advances to Terkirim and the action row changes under you, because the " +
      "mocks write rather than serving the same fixture back.",
    { parameters: withOwnShop(at("po-draft")) },
  ),
  Sent: story(
    "Placed and waiting. Terima barang is now the action; Bayar is not offered, because a shop " +
      "does not pay for what has not arrived (canPay excludes DRAFT and anything with nothing " +
      "outstanding). Open Terima barang and record a partial delivery — enter fewer packs than " +
      "were ordered on one line — and watch the order land on Sebagian, which is the state the " +
      "next story starts from.",
    { parameters: withOwnShop(at("po-sent")) },
  ),
  Partial: story(
    "One delivery in, two lines still owed. This is the fullest ordinary state: Terima barang " +
      "(again), Bayar, Retur and no void — an order with goods in the warehouse can no longer " +
      "be voided. The order carries PPN, so the totals card shows the PPN line and the items " +
      "table's derived column reads \"+ PPN 11%\": that column is what the batch's cost_price " +
      "becomes on receive — net of the line discount and inclusive of PPN — which is why it is " +
      "kept apart from the invoice arithmetic above it. The delivery below is cancellable: its " +
      "lots are untouched, so Batalkan is offered and the dialog spells out exactly which stock " +
      "disappears.",
    { parameters: withOwnShop(at("po-partial")) },
  ),
  Received: story(
    "Everything delivered, half paid, and the payment overdue (jatuh tempo is two days past). " +
      "The four deliveries below are the point of this story — they are the four states a " +
      "receipt row has: one still cancellable, one blocked because its lot has already been " +
      "sold, one blocked because an open stocktake is holding the lot, and one already " +
      "cancelled (struck through, badged, with its reason). A blocked row says WHY inline " +
      "instead of offering a dead button, and that reason is the same stable token CancelReceipt " +
      "would have refused with — translated through the shared server-error catalog, not a " +
      "second copy of the wording. The order also carries a per-line percentage discount, a " +
      "per-item fixed one and a cart-level discount.",
    { parameters: withOwnShop(at("po-received")) },
  ),
  Closed: story(
    "Delivered and paid in full — the end of the line. Only Retur remains, because returning " +
      "goods is still possible after settlement while everything else is not. Its one delivery " +
      "is blocked from cancelling for a reason that is not about the stock at all: " +
      "recomputePOStatus deliberately never reopens a CLOSED order, so cancelling here would " +
      "leave it settled with a lowered received quantity.",
    { parameters: withOwnShop(at("po-closed")) },
  ),
  SupplierCredit: story(
    "Paid in full, then a carton went back. The returns section appears with RTN-2026-0003, and " +
      "the header's last figure flips from Sisa (outstanding) to Kredit — outstanding is " +
      "ordered − paid − returned, so it is legitimately negative and means the supplier owes " +
      "the shop, not the other way round. Worth pinning: a naive \"amount due\" would render " +
      "this as a debt.",
    { parameters: withOwnShop(at("po-credit")) },
  ),
  Voided: story(
    "A cancelled order. Every action is gone and the lines are read-only, but the PO number and " +
      "the whole document survive — the number is already burned from the year's counter, so " +
      "removing the row would leave an unexplained gap in the sequence. It is excluded from the " +
      "outstanding filter and from every supplier balance.",
    { parameters: withOwnShop(at("po-voided")) },
  ),
  Loading: story("GetPurchaseOrder never answers: the page's spinner.", {
    parameters: {
      ...at("po-partial"),
      msw: mockApi(
        mockRpc(PurchaseOrderService, "getPurchaseOrder", { order: undefined }, { delay: "infinite" }),
        ...detailHandlers(),
      ),
    },
  }),
  NotFound: story(
    "GetPurchaseOrder fails NotFound (a deleted or mistyped id). The page has no empty state of " +
      "its own — `!po` falls into the same branch as loading, so it spins forever under an " +
      "error toast. Worth seeing: it is the one state here that reads as a hang rather than as " +
      "an answer.",
    {
      parameters: {
        ...at("po-missing"),
        msw: mockApi(
          mockRpcError(PurchaseOrderService, "getPurchaseOrder", Code.NotFound, "purchase order not found"),
          ...detailHandlers(),
        ),
      },
    },
  ),
};
