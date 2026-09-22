import type { HttpHandler } from "msw";

import { ProductService } from "../../gen/inventory_iface/v1/product_connect";
import { SupplierService } from "../../gen/inventory_iface/v1/supplier_connect";
import { POStatus, type PurchaseOrder } from "../../gen/purchasing_iface/v1/order_pb";
import { PurchaseOrderService } from "../../gen/purchasing_iface/v1/order_connect";
import { PurchasePaymentService } from "../../gen/purchasing_iface/v1/payment_connect";
import { PurchaseReceipt, PurchaseReceiptItem } from "../../gen/purchasing_iface/v1/receipt_pb";
import { PurchaseReceiptService } from "../../gen/purchasing_iface/v1/receipt_connect";
import { PurchaseReturn, PurchaseReturnItem } from "../../gen/purchasing_iface/v1/return_pb";
import { PurchaseReturnService } from "../../gen/purchasing_iface/v1/return_connect";
import { SUPPLIERS, WAREHOUSES, dateIn } from "../../routes/dev/fixtures";
import {
  PURCHASE_ORDERS,
  PURCHASE_RECEIPTS,
  PURCHASE_RETURNS,
} from "../../routes/dev/restockFixtures";
import { mockRpc } from "../../routes/dev/storyMocks";

// The fake shop behind the restock-order stories — one writeable copy of the
// ledger plus the handlers the detail page reads and writes. The states and
// their prose live next door in scenarios/purchaseOrderDetail.tsx; this file is
// only "the server".
//
// WHY THE MOCKS WRITE. The order detail page is a state machine with five
// actions on it, and a canned read makes every one of them look broken: Send
// invalidates the order key, the refetch serves the same DRAFT fixture back,
// and the button the operator just pressed is still there. Writing to a
// per-story store instead means the page behaves — Send moves DRAFT to SENT and
// swaps the action row, Receive lands a delivery and advances the order to
// partially- or fully-received, Pay closes a received order once it is settled,
// Cancel strikes a delivery through and hands the stock back. Same reasoning as
// the Products story mocking ListProducts as a function of the request, and as
// settingsShop.ts next door.
//
// The server's own rules are mirrored rather than waved through
// (recomputePOStatus, maybeCloseIfPaid, and the untouched-lot precondition on a
// cancel), because the refusals are the interesting half of this page: a
// delivery you cannot cancel is what sends an operator to a purchase return
// instead.

const DEFAULT_LIMIT = 25;

type Shop = {
  orders: PurchaseOrder[];
  receipts: PurchaseReceipt[];
  returns: PurchaseReturn[];
  nextReceiptNo: number;
  nextReturnNo: number;
};

/** A fresh, independently mutable copy of the ledger. */
export function newShop(): Shop {
  return {
    orders: PURCHASE_ORDERS.map((po) => po.clone()),
    receipts: PURCHASE_RECEIPTS.map((r) => r.clone()),
    returns: PURCHASE_RETURNS.map((r) => r.clone()),
    nextReceiptNo: 89,
    nextReturnNo: 4,
  };
}

const find = (shop: Shop, id: string) => shop.orders.find((po) => po.id === id);

/** recomputePOStatus: derived from received vs ordered, and never reopens CLOSED. */
function recomputeStatus(po: PurchaseOrder) {
  if (po.status === POStatus.PO_STATUS_CLOSED) return;
  const anyReceived = po.items.some((it) => it.receivedQty > 0);
  const allReceived = po.items.every((it) => it.receivedQty >= it.orderedQty);
  po.status =
    allReceived && anyReceived
      ? POStatus.PO_STATUS_RECEIVED
      : anyReceived
        ? POStatus.PO_STATUS_PARTIALLY_RECEIVED
        : POStatus.PO_STATUS_SENT;
  po.receivedAt = anyReceived ? po.receivedAt : 0n;
}

/** maybeCloseIfPaid: a RECEIVED order settles into CLOSED once it is paid off. */
function maybeClose(po: PurchaseOrder) {
  if (po.status === POStatus.PO_STATUS_RECEIVED && po.paidAmount >= po.orderedTotal && po.orderedTotal > 0n) {
    po.status = POStatus.PO_STATUS_CLOSED;
    po.closedAt = BigInt(Math.floor(Date.now() / 1000));
  }
}

function settle(po: PurchaseOrder) {
  po.outstanding = po.orderedTotal - po.paidAmount - po.returnedAmount;
}

function orderHandlers(shop: Shop): HttpHandler[] {
  return [
    mockRpc(PurchaseOrderService, "getPurchaseOrder", (req) => ({ order: find(shop, req.id) })),
    mockRpc(PurchaseOrderService, "sendPurchaseOrder", (req) => {
      const po = find(shop, req.id)!;
      po.status = POStatus.PO_STATUS_SENT;
      po.sentAt = BigInt(Math.floor(Date.now() / 1000));
      return { order: po };
    }),
    mockRpc(PurchaseOrderService, "voidPurchaseOrder", (req) => {
      const po = find(shop, req.id)!;
      po.status = POStatus.PO_STATUS_VOIDED;
      return { order: po };
    }),
    mockRpc(PurchasePaymentService, "payPurchase", (req) => {
      const po = find(shop, req.purchaseOrderId)!;
      po.paidAmount += req.amount;
      settle(po);
      maybeClose(po);
      return { purchaseOrderId: po.id, paidAmount: po.paidAmount, outstanding: po.outstanding };
    }),
  ];
}

function receiptHandlers(shop: Shop): HttpHandler[] {
  return [
    mockRpc(PurchaseReceiptService, "listReceipts", (req) => {
      const rows = shop.receipts
        .filter((r) => r.purchaseOrderId === req.purchaseOrderId)
        .sort((a, b) => b.receiptNo.localeCompare(a.receiptNo));
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { receipts: rows.slice(req.offset, req.offset + limit), total: rows.length };
    }),
    mockRpc(PurchaseReceiptService, "createReceipt", (req) => {
      const po = find(shop, req.purchaseOrderId)!;
      const no = shop.nextReceiptNo++;
      const receipt = new PurchaseReceipt({
        id: `rcv-new-${no}`,
        receiptNo: `RCV-2026-${String(no).padStart(4, "0")}`,
        purchaseOrderId: po.id,
        receivedAt: req.receivedAt || dateIn(0),
        receivedBy: "user-admin",
        invoiceNo: req.invoiceNo,
        note: req.note,
        createdAt: BigInt(Math.floor(Date.now() / 1000)),
        // A brand-new delivery's lots are untouched by definition, so it is
        // always cancellable the moment it lands.
        cancellable: true,
        items: req.lines.map((l, i) => {
          const item = po.items.find((it) => it.id === l.purchaseOrderItemId)!;
          const factor = Number(item.unitFactor) || 1;
          const qty = l.qty * factor;
          item.receivedQty += qty;
          return new PurchaseReceiptItem({
            id: `rcv-new-${no}-i${i + 1}`,
            purchaseReceiptId: `rcv-new-${no}`,
            purchaseOrderItemId: item.id,
            productId: item.productId,
            qty,
            unitCostPrice: l.unitCostPrice || item.unitCostPrice,
            batchNumber: l.batchNumber,
            expiryDate: l.expiryDate,
            batchId: `rcv-new-${no}-b${i + 1}`,
            productUnitId: item.productUnitId,
            unitName: item.unitName,
            unitFactor: item.unitFactor,
            returnableQty: BigInt(qty),
          });
        }),
      });
      shop.receipts.push(receipt);
      po.receivedAt = receipt.createdAt;
      po.invoiceNo = req.invoiceNo || po.invoiceNo;
      recomputeStatus(po);
      settle(po);
      return { receipt };
    }),
    mockRpc(PurchaseReceiptService, "cancelReceipt", (req) => {
      const receipt = shop.receipts.find((r) => r.id === req.id)!;
      const po = find(shop, receipt.purchaseOrderId)!;
      receipt.voidedAt = BigInt(Math.floor(Date.now() / 1000));
      receipt.voidedBy = "user-owner";
      receipt.voidReason = req.reason;
      receipt.cancellable = false;
      for (const it of receipt.items) {
        const line = po.items.find((l) => l.id === it.purchaseOrderItemId);
        if (line) line.receivedQty = Math.max(0, line.receivedQty - it.qty);
        // The lot is deleted outright, not offset by a compensating movement —
        // so the batch link goes with it.
        it.batchId = "";
        it.returnableQty = 0n;
      }
      recomputeStatus(po);
      settle(po);
      return { receipt };
    }),
  ];
}

function returnHandlers(shop: Shop): HttpHandler[] {
  return [
    mockRpc(PurchaseReturnService, "listPurchaseReturns", (req) => {
      const rows = shop.returns.filter((r) => r.purchaseOrderId === req.purchaseOrderId);
      const limit = req.limit > 0 ? req.limit : DEFAULT_LIMIT;
      return { returns: rows.slice(req.offset, req.offset + limit), total: rows.length };
    }),
    mockRpc(PurchaseReturnService, "createPurchaseReturn", (req) => {
      const po = find(shop, req.purchaseOrderId)!;
      const no = shop.nextReturnNo++;
      let refund = 0n;
      const items = req.lines.map((l, i) => {
        const source = shop.receipts
          .flatMap((r) => r.items)
          .find((it) => it.id === l.purchaseReceiptItemId)!;
        const factor = Number(source.unitFactor) || 1;
        const qty = l.qty * factor;
        source.returnableQty -= BigInt(qty);
        const line = po.items.find((it) => it.id === source.purchaseOrderItemId);
        if (line) line.receivedQty = Math.max(0, line.receivedQty - qty);
        refund += BigInt(qty) * source.unitCostPrice;
        return new PurchaseReturnItem({
          id: `rtn-new-${no}-i${i + 1}`,
          purchaseReturnId: `rtn-new-${no}`,
          purchaseReceiptItemId: source.id,
          purchaseOrderItemId: source.purchaseOrderItemId,
          productId: source.productId,
          batchId: source.batchId,
          qty,
          unitCostPrice: source.unitCostPrice,
          unitName: source.unitName,
          unitFactor: source.unitFactor,
        });
      });
      const ret = new PurchaseReturn({
        id: `rtn-new-${no}`,
        returnNo: `RTN-2026-${String(no).padStart(4, "0")}`,
        purchaseOrderId: po.id,
        warehouseId: WAREHOUSES[0].id,
        returnedAt: req.returnedAt || dateIn(0),
        returnedBy: "user-admin",
        reason: req.reason,
        note: req.note,
        refundAmount: refund,
        createdAt: BigInt(Math.floor(Date.now() / 1000)),
        items,
      });
      shop.returns.push(ret);
      po.returnedAmount += refund;
      recomputeStatus(po);
      settle(po);
      return { purchaseReturn: ret };
    }),
  ];
}

/** The names the page resolves by id: this order's supplier and its products. */
function refHandlers(shop: Shop): HttpHandler[] {
  return [
    mockRpc(SupplierService, "resolveSuppliers", (req) => ({
      suppliers: SUPPLIERS.filter((s) => req.ids.includes(s.id)),
    })),
    mockRpc(ProductService, "resolveProducts", (req) => {
      const seen = new Map<string, { id: string; name: string; sku: string }>();
      for (const po of shop.orders) {
        for (const it of po.items) {
          if (req.ids.includes(it.productId)) {
            seen.set(it.productId, { id: it.productId, name: it.productName, sku: it.productSku });
          }
        }
      }
      return { products: Array.from(seen.values()) };
    }),
  ];
}

/** Everything one order's detail page reads and writes, over a fresh ledger. */
export function shopHandlers(shop = newShop()): HttpHandler[] {
  return [
    ...orderHandlers(shop),
    ...receiptHandlers(shop),
    ...returnHandlers(shop),
    ...refHandlers(shop),
  ];
}
