import { Code } from "@connectrpc/connect";
import type { StoryObj } from "@storybook/react";
import { Route } from "react-router-dom";
import { expect, userEvent, within } from "storybook/test";

import { PurchaseOrderService } from "../../gen/purchasing_iface/v1/order_connect";
import { withPageContext } from "../../routes/dev/storyDecorators";
import { mockApi, mockRpc, mockRpcError } from "../../routes/dev/storyMocks";
import PurchaseOrderDetail from "../../routes/purchasing/PurchaseOrderDetail";
import Purchasing from "../../routes/purchasing/Purchasing";
import { newShop, shopHandlers } from "./restockShop";

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
// settled one, Cancel strikes a delivery through and takes the stock back, and
// Edit (Ubah) does that and lands the corrected delivery under a new number.
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

/** The header's Ubah (Edit) action, which follows PurchaseOrder.editable. */
const EDIT_BUTTON = { name: /^(edit|ubah)$/i };

/**
 * A delivery row's own Ubah. It reads "Ubah" like the header's, but is NAMED
 * per delivery ("Ubah penerimaan RCV-…"), so EDIT_BUTTON never matches it.
 * Each delivery row is a group named by its RCV number.
 */
const DELIVERY_EDIT = { name: /^(edit delivery|ubah penerimaan) RCV-/i };

/**
 * po-received's deliveries: 0084 is cancellable, 0083 is blocked (its lot has
 * been sold from). The next number the shop hands out is 0089.
 */
const CANCELLABLE_DELIVERY = "RCV-2026-0084";
const BLOCKED_DELIVERY = "RCV-2026-0083";

/** Opens Ubah on the cancellable delivery after checking the blocked one has none. */
async function openEditDelivery(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  // A blocked delivery offers neither Batalkan nor Ubah: the edit IS a cancel,
  // so the row says why instead, once, for both.
  const blocked = await canvas.findByRole("group", { name: BLOCKED_DELIVERY });
  await expect(within(blocked).queryByRole("button", DELIVERY_EDIT)).toBeNull();
  const row = await canvas.findByRole("group", { name: CANCELLABLE_DELIVERY });
  await userEvent.click(within(row).getByRole("button", DELIVERY_EDIT));
  const body = within(canvasElement.ownerDocument.body);
  return body.findByRole("dialog", {
    name: new RegExp(`^(edit delivery|ubah penerimaan) ${CANCELLABLE_DELIVERY}$`, "i"),
  });
}

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

/**
 * A ledger whose orders never recorded a pabrik -- every line's Pabrik cell a
 * dash. This is what every order placed before the field existed looks like,
 * and it is permanent: the maker is read off the distributor's invoice at
 * order time, so there is nowhere to recover it from afterwards.
 */
const withoutMakers = (extra: Record<string, unknown>): StoryObj["parameters"] => {
  const shop = newShop();
  shop.orders.forEach((po) => po.items.forEach((it) => (it.manufacturerId = "")));
  return { ...extra, msw: mockApi(...shopHandlers(shop)) };
};

export const stories = {
  Draft: story(
    "A restock order still being written: no PO has gone to the supplier yet, so the only " +
      "actions are Kirim (send it) and Batalkan (void it) — no receive, no payment. Press Kirim " +
      "and the order advances to Terkirim and the action row changes under you, because the " +
      "mocks write rather than serving the same fixture back. The header names who entered the " +
      "order (Dibuat oleh), resolved from created_by through ResolveUsers.",
    {
      parameters: withOwnShop(at("po-draft")),
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await canvas.findByText(/^(created by|dibuat oleh)$/i);
        await canvas.findByText("Sari Wulandari");
        await canvas.findByRole("button", EDIT_BUTTON);
      },
    },
  ),
  Sent: story(
    "Placed and waiting. Terima barang is now the action; Bayar is not offered, because a shop " +
      "does not pay for what has not arrived (canPay excludes DRAFT and anything with nothing " +
      "outstanding). Ubah is still offered: an order that has never had a delivery can be " +
      "corrected in place instead of being cancelled and typed in again (the edit page warns " +
      "that the change is not re-sent to the supplier). Open Terima barang and record a partial " +
      "delivery — enter fewer packs than were ordered on one line — and watch the order land on " +
      "Sebagian with Ubah gone: from the first delivery on, the order is not editable, even if " +
      "that delivery is later cancelled.",
    {
      parameters: withOwnShop(at("po-sent")),
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await canvas.findByRole("button", EDIT_BUTTON);
      },
    },
  ),
  ReceiveExpiryDefaults: story(
    "The same order with Terima barang already open, to show how expiry is filled in. Each " +
      "line starts from its product's setting: Indomie Goreng and Aqua pre-fill a date N months " +
      "after the received date (end of that month), marked as the product default until " +
      "somebody types over it; Indomie Soto has no default, so it is typed from the pack — `0327` " +
      "is end of March 2027, `05/03/27` that exact day, and the line under the field says what " +
      "will be saved; the batteries do not expire, so there is no field at all. Change the " +
      "received date and the untouched defaults move with it. Enter jumps to the next line.",
    {
      parameters: withOwnShop(at("po-sent")),
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        await userEvent.click(await canvas.findByRole("button", { name: /^(receive|terima)$/i }));
      },
    },
  ),
  Partial: story(
    "One delivery in, two lines still owed. This is the fullest ordinary state: Terima barang " +
      "(again), Bayar, Retur and no void — an order with goods in the warehouse can no longer " +
      "be voided. The order carries PPN, so the totals card shows the PPN line and the items " +
      "table's derived column reads \"+ PPN 11%\": that column is what the batch's cost_price " +
      "becomes on receive — net of the line discount and inclusive of PPN — which is why it is " +
      "kept apart from the invoice arithmetic above it. The delivery below is cancellable: its " +
      "lots are untouched, so Batalkan is offered and the dialog spells out exactly which stock " +
      "disappears, and so is the delivery's own Ubah (correct it — see EditDelivery). There is no " +
      "Ubah in the header: an order that has had a delivery is not editable.",
    {
      parameters: withOwnShop(at("po-partial")),
      play: async ({ canvasElement }) => {
        const canvas = within(canvasElement);
        // Wait for the order's own action row before asserting what is absent.
        await canvas.findByRole("button", { name: /^(receive|terima)$/i });
        await expect(canvas.queryByRole("button", EDIT_BUTTON)).toBeNull();
      },
    },
  ),
  Received: story(
    "Everything delivered, half paid, and the payment overdue (jatuh tempo is two days past). " +
      "The four deliveries below are the point of this story — they are the four states a " +
      "receipt row has: one still cancellable, one blocked because its lot has already been " +
      "sold, one blocked because an open stocktake is holding the lot, and one already " +
      "cancelled (struck through, badged, with its reason). The cancellable one offers Ubah " +
      "beside Batalkan (see EditDelivery). A blocked row says WHY inline " +
      "instead of offering a dead button, and that reason is the same stable token CancelReceipt " +
      "would have refused with — translated through the shared server-error catalog, not a " +
      "second copy of the wording. The order also carries a per-line percentage discount, a " +
      "per-item fixed one and a cart-level discount.",
    { parameters: withOwnShop(at("po-received")) },
  ),
  EditDelivery: story(
    "The fully-delivered order with Ubah pressed on its one cancellable delivery. Ubah opens " +
      "Terima barang in edit mode, pre-filled with what that delivery recorded — the quantity in " +
      "the line's pack unit, the batch number, the expiry (a product default stays marked as " +
      "one), the received date, the faktur no and the note — under \"Ubah penerimaan RCV-…\". " +
      "Each line's maximum is what is still owed plus what this delivery brought, because saving " +
      "is ReplaceReceipt: the server cancels this delivery and records the corrected one under a " +
      "NEW RCV number in one transaction, so the old one stays listed as cancelled and the " +
      "change can be traced. A reason is required, as for Batalkan. Ubah is offered exactly " +
      "where Batalkan is: the delivery whose lot has been sold from offers neither.",
    {
      parameters: withOwnShop(at("po-received")),
      play: async ({ canvasElement }) => {
        await openEditDelivery(canvasElement);
      },
    },
  ),
  DeliveryReplaced: story(
    "The edit carried through: the reason typed and Simpan perubahan pressed. The edited " +
      "delivery is struck through as cancelled, with its reason and \"diganti oleh " +
      "RCV-2026-0089\" beside it, and the corrected delivery is listed above it as a live, " +
      "cancellable row of its own — so it can be corrected again.",
    {
      parameters: withOwnShop(at("po-received")),
      play: async ({ canvasElement }) => {
        const dialog = within(await openEditDelivery(canvasElement));
        await userEvent.type(dialog.getByRole("textbox", { name: /^(reason|alasan)$/i }), "Jumlah salah ketik");
        await userEvent.click(dialog.getByRole("button", { name: /^(save changes|simpan perubahan)$/i }));
        const canvas = within(canvasElement);
        const old = await canvas.findByRole("group", { name: CANCELLABLE_DELIVERY });
        await within(old).findByText(/^(replaced by|diganti oleh) RCV-2026-0089$/i);
        const created = await canvas.findByRole("group", { name: "RCV-2026-0089" });
        await expect(within(created).getByRole("button", DELIVERY_EDIT)).toBeVisible();
      },
    },
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
  UnrecordedMaker: story(
    "The same order with no pabrik on any line — a dash in every Pabrik cell. Orders placed " +
      "before the field existed all read this way, and they stay that way: the maker is copied " +
      "off the distributor's invoice when the order is written, so there is nothing to recover " +
      "it from later. It matters here more than anywhere else, because this column is what " +
      "CreateReceipt stamps onto the lot — a line with no pabrik produces a batch with no " +
      "pabrik, and that lot then answers no recall. The dash has to read as not-recorded, not " +
      "as a column that failed to load.",
    { parameters: withoutMakers(at("po-partial")) },
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
