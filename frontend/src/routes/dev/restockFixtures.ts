import type { Product, ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import type { ListPurchaseOrdersRequest } from "../../gen/purchasing_iface/v1/order_pb";
import { POStatus, PurchaseOrder, PurchaseOrderItem } from "../../gen/purchasing_iface/v1/order_pb";
import { SupplierBalance } from "../../gen/purchasing_iface/v1/payment_pb";
import { PurchaseReceipt, PurchaseReceiptItem } from "../../gen/purchasing_iface/v1/receipt_pb";
import { PurchaseReturn, PurchaseReturnItem } from "../../gen/purchasing_iface/v1/return_pb";
import { RETAIL_CATALOG, SUPPLIERS, WAREHOUSES, dateIn, daysAgo } from "./fixtures";

// The restock ledger behind /purchasing, for the screen stories: orders across
// every status, the deliveries recorded against them, and one return. Split out
// of fixtures.ts (which stays the product catalog) because it is a second
// domain with its own arithmetic — it reads that catalog and nothing reads it
// back, so the dependency runs one way.
//
// Everything here is DERIVED rather than transcribed: every total is computed
// by the same arithmetic the server uses (lineNetSubtotal -> computePOTotals),
// so a story cannot show an order whose lines fail to add up to its total, and
// changing a product's cost moves these figures with it.

/** A line on a restock order, in the product's purchasable unit. */
type POLine = {
  product: Product;
  /** Packs ordered (of the unit `purchaseUnitOf` picks). */
  packs: number;
  /** Packs already delivered. Defaults to all of them once the order is settled. */
  receivedPacks?: number;
  discountType?: "PERCENT" | "FIXED";
  /** Minor units when FIXED; basis points (2.5% = 250) when PERCENT. */
  discountValue?: bigint;
  discountPerItem?: boolean;
};

type POSeed = {
  id: string;
  /** The NNNN in PO-2026-NNNN. */
  no: number;
  supplier: (typeof SUPPLIERS)[number];
  status: POStatus;
  lines: POLine[];
  createdDaysAgo: number;
  /** Most recent delivery; omitted = nothing has arrived. */
  receivedDaysAgo?: number;
  invoiceNo?: string;
  /** Payment due, days from today (negative = overdue). */
  dueInDays?: number;
  paid?: bigint;
  returned?: bigint;
  ppn?: boolean;
  cartDiscount?: bigint;
  note?: string;
};

/**
 * What the shop buys this product by: its largest purchasable pack, or the base
 * unit when it has none — which is exactly what `resolvePurchaseUnit` resolves
 * an empty unit id to.
 */
function purchaseUnitOf(p: Product): ProductUnit {
  return p.units.find((u) => u.purchasable && u.active) ?? p.units[0];
}

/** `lineNetSubtotal` in TypeScript: the line total net of its own discount. */
function lineNet(gross: bigint, packs: number, l: POLine): bigint {
  const value = l.discountValue ?? 0n;
  if (value <= 0n) return gross;
  const percent = l.discountType === "PERCENT";
  let disc: bigint;
  if (l.discountPerItem && packs > 0) {
    const perPack = gross / BigInt(packs);
    let d = percent ? (perPack * value + 5_000n) / 10_000n : value;
    if (d > perPack) d = perPack;
    disc = d * BigInt(packs);
  } else {
    disc = percent ? (gross * value + 5_000n) / 10_000n : value;
  }
  return disc > gross ? 0n : gross - disc;
}

function makePurchaseOrder(seed: POSeed): PurchaseOrder {
  const settled =
    seed.status === POStatus.PO_STATUS_RECEIVED || seed.status === POStatus.PO_STATUS_CLOSED;
  const items = seed.lines.map((l, i) => {
    const unit = purchaseUnitOf(l.product);
    const factor = Number(unit.factor);
    const receivedPacks = l.receivedPacks ?? (settled ? l.packs : 0);
    // ordered_qty / received_qty are BASE units on the wire; the unit fields
    // beside them describe what was ordered in (see "Units of measure").
    const gross = BigInt(l.packs * factor) * l.product.referenceCost;
    return new PurchaseOrderItem({
      id: `${seed.id}-i${i + 1}`,
      purchaseOrderId: seed.id,
      productId: l.product.id,
      productName: l.product.name,
      productSku: l.product.sku,
      orderedQty: l.packs * factor,
      receivedQty: receivedPacks * factor,
      unitCostPrice: l.product.referenceCost,
      subtotal: lineNet(gross, l.packs, l),
      productUnitId: unit.id,
      unitName: unit.name,
      unitFactor: unit.factor,
      discountType: l.discountType ?? "",
      discountValue: l.discountValue ?? 0n,
      discountPerItem: l.discountPerItem ?? false,
    });
  });

  const subtotal = items.reduce((sum, it) => sum + it.subtotal, 0n);
  const cartDiscount = seed.cartDiscount ?? 0n;
  const dpp = subtotal - cartDiscount;
  const ppnAmount = seed.ppn ? (dpp * 11n + 50n) / 100n : 0n;
  const orderedTotal = dpp + ppnAmount;
  const paid = seed.paid ?? 0n;
  const returned = seed.returned ?? 0n;

  return new PurchaseOrder({
    id: seed.id,
    poNo: `PO-2026-${String(seed.no).padStart(4, "0")}`,
    supplierId: seed.supplier.id,
    status: seed.status,
    items,
    subtotal,
    cartDiscount,
    ppnEnabled: seed.ppn ?? false,
    ppnRate: seed.ppn ? 11 : 0,
    ppnAmount,
    orderedTotal,
    paidAmount: paid,
    returnedAmount: returned,
    outstanding: orderedTotal - paid - returned,
    invoiceNo: seed.invoiceNo ?? "",
    invoiceDate: seed.receivedDaysAgo !== undefined ? dateIn(-seed.receivedDaysAgo) : "",
    dueAt: seed.dueInDays !== undefined ? dateIn(seed.dueInDays) : "",
    note: seed.note ?? "",
    createdAt: daysAgo(seed.createdDaysAgo),
    sentAt: seed.status === POStatus.PO_STATUS_DRAFT ? 0n : daysAgo(seed.createdDaysAgo - 0.5),
    receivedAt: seed.receivedDaysAgo !== undefined ? daysAgo(seed.receivedDaysAgo) : 0n,
    warehouseId: WAREHOUSES[0].id,
    warehouseName: WAREHOUSES[0].name,
    createdBy: "user-admin",
  });
}

// Named so the orders below read as a shop's own paperwork rather than as
// catalog indexes.
const [MIE_GORENG, MIE_SOTO, AQUA_600] = RETAIL_CATALOG;
const [MINYAK, TEH_BOTOL, KOPI_ABC] = [RETAIL_CATALOG[6], RETAIL_CATALOG[7], RETAIL_CATALOG[8]];
const [PEPSODENT, LIFEBUOY, SUNSILK] = [RETAIL_CATALOG[10], RETAIL_CATALOG[11], RETAIL_CATALOG[12]];
const [RINSO, SUNLIGHT, ULTRA_MILK] = [RETAIL_CATALOG[13], RETAIL_CATALOG[14], RETAIL_CATALOG[15]];
const [KECAP, MAMYPOKO, PASEO] = [RETAIL_CATALOG[21], RETAIL_CATALOG[24], RETAIL_CATALOG[25]];

// The seven orders the detail stories are written against — one per status of
// the state machine, plus the supplier-credit case. Newest first, which is also
// the list's order (ListPurchaseOrders sorts created_at DESC).
const PO_SEEDS: POSeed[] = [
  {
    id: "po-draft",
    no: 148,
    supplier: SUPPLIERS[0],
    status: POStatus.PO_STATUS_DRAFT,
    lines: [
      { product: PEPSODENT, packs: 4 },
      { product: LIFEBUOY, packs: 6 },
    ],
    createdDaysAgo: 1,
    note: "Menunggu konfirmasi harga dari sales.",
  },
  {
    id: "po-sent",
    no: 147,
    supplier: SUPPLIERS[2],
    status: POStatus.PO_STATUS_SENT,
    lines: [
      { product: MIE_GORENG, packs: 10 },
      { product: MIE_SOTO, packs: 6 },
      { product: AQUA_600, packs: 8 },
    ],
    createdDaysAgo: 3,
    dueInDays: 27,
  },
  {
    id: "po-partial",
    no: 146,
    supplier: SUPPLIERS[2],
    status: POStatus.PO_STATUS_PARTIALLY_RECEIVED,
    lines: [
      { product: TEH_BOTOL, packs: 8, receivedPacks: 8 },
      { product: KOPI_ABC, packs: 4, receivedPacks: 2 },
      { product: ULTRA_MILK, packs: 6, receivedPacks: 0 },
    ],
    createdDaysAgo: 9,
    receivedDaysAgo: 2,
    invoiceNo: "INV/2026/09/0412",
    dueInDays: 21,
    ppn: true,
  },
  {
    id: "po-received",
    no: 145,
    supplier: SUPPLIERS[3],
    status: POStatus.PO_STATUS_RECEIVED,
    lines: [
      { product: RINSO, packs: 5, discountType: "PERCENT", discountValue: 250n },
      { product: SUNLIGHT, packs: 5 },
      { product: KECAP, packs: 3, discountType: "FIXED", discountValue: 5_000n, discountPerItem: true },
    ],
    createdDaysAgo: 16,
    receivedDaysAgo: 10,
    invoiceNo: "MS-24091",
    dueInDays: -2,
    cartDiscount: 25_000n,
    paid: 500_000n,
  },
  {
    id: "po-closed",
    no: 144,
    supplier: SUPPLIERS[0],
    status: POStatus.PO_STATUS_CLOSED,
    lines: [
      { product: MAMYPOKO, packs: 2 },
      { product: PASEO, packs: 4 },
    ],
    createdDaysAgo: 34,
    receivedDaysAgo: 29,
    invoiceNo: "SF/26/0881",
  },
  {
    id: "po-credit",
    no: 143,
    supplier: SUPPLIERS[3],
    status: POStatus.PO_STATUS_RECEIVED,
    lines: [{ product: MINYAK, packs: 6 }],
    createdDaysAgo: 40,
    receivedDaysAgo: 35,
    invoiceNo: "MS-23884",
    returned: 204_600n,
  },
  {
    id: "po-voided",
    no: 142,
    supplier: SUPPLIERS[1],
    status: POStatus.PO_STATUS_VOIDED,
    lines: [{ product: SUNSILK, packs: 3 }],
    createdDaysAgo: 22,
    note: "Salah pemasok — dibuat ulang ke PT Indo Grosir Jaya.",
  },
];

// A year of ordinary restocks behind them, so "all" pages at 25 and every tab
// has more than its headline order. Rotates supplier, products and status the
// way a working ledger does: mostly settled, a couple still in transit.
const FILLER_STATUSES = [
  POStatus.PO_STATUS_CLOSED,
  POStatus.PO_STATUS_CLOSED,
  POStatus.PO_STATUS_RECEIVED,
  POStatus.PO_STATUS_CLOSED,
  POStatus.PO_STATUS_SENT,
  POStatus.PO_STATUS_RECEIVED,
  POStatus.PO_STATUS_VOIDED,
];

const FILLER_SEEDS: POSeed[] = Array.from({ length: 21 }, (_, i): POSeed => {
  const status = FILLER_STATUSES[i % FILLER_STATUSES.length];
  const arrived = status !== POStatus.PO_STATUS_SENT && status !== POStatus.PO_STATUS_VOIDED;
  const created = 48 + i * 11;
  const supplier = SUPPLIERS[i % SUPPLIERS.length];
  return {
    id: `po-f${i + 1}`,
    no: 141 - i,
    supplier,
    status,
    lines: [
      { product: RETAIL_CATALOG[(i * 3) % 30], packs: 2 + (i % 4) },
      { product: RETAIL_CATALOG[(i * 3 + 7) % 30], packs: 1 + (i % 3) },
    ],
    createdDaysAgo: created,
    receivedDaysAgo: arrived ? created - 5 : undefined,
    invoiceNo: arrived ? `${supplier.code.slice(4)}-${2400 + i}` : "",
    ppn: i % 4 === 0,
  };
});

export const PURCHASE_ORDERS: PurchaseOrder[] = [...PO_SEEDS, ...FILLER_SEEDS].map(makePurchaseOrder);

// Two things are defined by what is LEFT to pay rather than by a figure typed
// into a seed: CLOSED means settled in full (that is what closes an order), and
// the credit case is paid in full and then partly returned. Both need the
// computed total, so they are settled here instead of with a hand-copied number
// that would drift the moment a line or a cost changes.
for (const po of PURCHASE_ORDERS) {
  if (po.status === POStatus.PO_STATUS_CLOSED || po.id === "po-credit") {
    po.paidAmount = po.orderedTotal;
    po.closedAt = po.status === POStatus.PO_STATUS_CLOSED ? po.receivedAt : 0n;
  } else if (po.status === POStatus.PO_STATUS_RECEIVED && po.paidAmount === 0n) {
    po.paidAmount = po.orderedTotal / 2n; // delivered, half settled
  }
  po.outstanding = po.orderedTotal - po.paidAmount - po.returnedAmount;
}

export const PURCHASE_ORDERS_BY_ID = new Map(PURCHASE_ORDERS.map((po) => [po.id, po]));

/**
 * ListPurchaseOrders, faithfully enough that the toolbar works live in a story:
 * the same predicate as `applyPOFilters` (status tab, supplier, the outstanding
 * toggle, a PO-no / supplier / product substring, and a range over either the
 * created or the received date), ordered created_at DESC like the server.
 */
export function filterPurchaseOrders(
  rows: PurchaseOrder[],
  req: Pick<
    ListPurchaseOrdersRequest,
    "status" | "supplierId" | "onlyOutstanding" | "query" | "fromUnix" | "toUnix" | "dateField"
  >,
): PurchaseOrder[] {
  const q = req.query.trim().toLowerCase();
  return rows
    .filter((po) => {
      if (req.status !== POStatus.PO_STATUS_UNSPECIFIED && po.status !== req.status) return false;
      if (req.supplierId && po.supplierId !== req.supplierId) return false;
      if (req.onlyOutstanding) {
        if (po.status === POStatus.PO_STATUS_VOIDED || po.status === POStatus.PO_STATUS_DRAFT) {
          return false;
        }
        if (po.outstanding <= 0n) return false;
      }
      if (q) {
        const supplier = SUPPLIERS.find((s) => s.id === po.supplierId);
        const hay = [po.poNo, supplier?.name ?? "", supplier?.code ?? ""]
          .concat(po.items.map((it) => it.productName))
          .join(" ")
          .toLowerCase();
        if (!hay.includes(q)) return false;
      }
      if (req.fromUnix > 0n || req.toUnix > 0n) {
        const at = req.dateField === "received" ? po.receivedAt : po.createdAt;
        if (at === 0n) return false; // "received" excludes what never arrived
        if (req.fromUnix > 0n && at < req.fromUnix) return false;
        if (req.toUnix > 0n && at >= req.toUnix) return false;
      }
      return true;
    })
    .sort((a, b) => Number(b.createdAt - a.createdAt));
}

/**
 * GetPurchaseOrdersSummary over the SAME set the list shows — the filter-parity
 * rule the two handlers share `applyPOFilters` for. Aggregating a different
 * array here would let the story agree with itself while the app disagrees.
 */
export function summarizePurchaseOrders(rows: PurchaseOrder[]) {
  const products = new Set<string>();
  let itemCount = 0n;
  let total = 0n;
  for (const po of rows) {
    total += po.orderedTotal;
    for (const it of po.items) {
      products.add(it.productId);
      itemCount += BigInt(it.orderedQty);
    }
  }
  return {
    orderCount: BigInt(rows.length),
    productCount: BigInt(products.size),
    itemCount,
    total,
  };
}

// --- deliveries (receipts) + returns ----------------------------------------

type ReceiptSeed = {
  id: string;
  /** The NNNN in RCV-2026-NNNN. */
  no: number;
  poId: string;
  /** Which of the order's lines this delivery covered, and how many packs each. */
  lines: Array<{
    itemIndex: number;
    packs: number;
    batchSuffix: string;
    expiresInDays: number;
    /** On-hand left of this lot; defaults to the whole delivery. */
    returnablePacks?: number;
  }>;
  receivedDaysAgo: number;
  invoiceNo?: string;
  /** Empty = cancellable. Otherwise the stable token the handler would refuse with. */
  blocked?: string;
  voided?: { daysAgo: number; reason: string };
};

function makeReceipt(seed: ReceiptSeed): PurchaseReceipt {
  const po = PURCHASE_ORDERS_BY_ID.get(seed.poId)!;
  const voided = seed.voided !== undefined;
  return new PurchaseReceipt({
    id: seed.id,
    receiptNo: `RCV-2026-${String(seed.no).padStart(4, "0")}`,
    purchaseOrderId: seed.poId,
    receivedAt: dateIn(-seed.receivedDaysAgo),
    receivedBy: "user-admin",
    invoiceNo: seed.invoiceNo ?? po.invoiceNo,
    createdAt: daysAgo(seed.receivedDaysAgo),
    voidedAt: seed.voided ? daysAgo(seed.voided.daysAgo) : 0n,
    voidedBy: seed.voided ? "user-owner" : "",
    voidReason: seed.voided?.reason ?? "",
    // A cancelled receipt is never offered the action, so these two say nothing
    // about it — the row renders from voidedAt alone.
    cancellable: !voided && !seed.blocked,
    cancelBlockedReason: voided ? "" : (seed.blocked ?? ""),
    items: seed.lines.map((l, i) => {
      const item = po.items[l.itemIndex];
      const factor = Number(item.unitFactor);
      return new PurchaseReceiptItem({
        id: `${seed.id}-i${i + 1}`,
        purchaseReceiptId: seed.id,
        purchaseOrderItemId: item.id,
        productId: item.productId,
        qty: l.packs * factor,
        unitCostPrice: item.unitCostPrice,
        batchNumber: `${item.productSku.slice(-4)}-${l.batchSuffix}`,
        expiryDate: dateIn(l.expiresInDays),
        // Cancelling clears the batch link, because the lot is deleted outright.
        batchId: voided ? "" : `${seed.id}-b${i + 1}`,
        productUnitId: item.productUnitId,
        unitName: item.unitName,
        unitFactor: item.unitFactor,
        returnableQty: BigInt((l.returnablePacks ?? (voided ? 0 : l.packs)) * factor),
      });
    }),
  });
}

// One delivery per state worth showing. The three blocked reasons are the three
// the server precomputes onto `cancel_blocked_reason` (markCancellable): the lot
// has been touched, it is held by an open stocktake, or the order is already
// settled. Those are what send an operator to a return instead of a cancel, so
// the page has to be able to say each of them.
const RECEIPT_SEEDS: ReceiptSeed[] = [
  {
    id: "rcv-partial-1",
    no: 88,
    poId: "po-partial",
    lines: [
      { itemIndex: 0, packs: 8, batchSuffix: "A", expiresInDays: 300 },
      { itemIndex: 1, packs: 2, batchSuffix: "A", expiresInDays: 420 },
    ],
    receivedDaysAgo: 2,
  },
  {
    id: "rcv-received-1",
    no: 84,
    poId: "po-received",
    lines: [{ itemIndex: 0, packs: 5, batchSuffix: "C", expiresInDays: 500 }],
    receivedDaysAgo: 10,
  },
  {
    id: "rcv-received-2",
    no: 83,
    poId: "po-received",
    lines: [{ itemIndex: 1, packs: 5, batchSuffix: "C", expiresInDays: 520, returnablePacks: 3 }],
    receivedDaysAgo: 12,
    blocked: "purchasing.receipt_lot_consumed",
  },
  {
    id: "rcv-received-3",
    no: 82,
    poId: "po-received",
    lines: [{ itemIndex: 2, packs: 3, batchSuffix: "B", expiresInDays: 260 }],
    receivedDaysAgo: 13,
    blocked: "purchasing.receipt_lot_in_stocktake",
  },
  {
    // Already cancelled: entered twice, so its own lots are gone and the order's
    // received_qty went back — which is why its packs exceed what the line
    // ordered without the arithmetic being wrong.
    id: "rcv-received-4",
    no: 81,
    poId: "po-received",
    lines: [{ itemIndex: 2, packs: 2, batchSuffix: "X", expiresInDays: 260 }],
    receivedDaysAgo: 14,
    voided: { daysAgo: 13, reason: "Terinput dua kali" },
  },
  {
    id: "rcv-closed-1",
    no: 70,
    poId: "po-closed",
    lines: [
      { itemIndex: 0, packs: 2, batchSuffix: "A", expiresInDays: 700 },
      { itemIndex: 1, packs: 4, batchSuffix: "A", expiresInDays: 700 },
    ],
    receivedDaysAgo: 29,
    blocked: "purchasing.receipt_cancel_po_closed",
  },
  {
    id: "rcv-credit-1",
    no: 64,
    poId: "po-credit",
    lines: [{ itemIndex: 0, packs: 6, batchSuffix: "A", expiresInDays: 380, returnablePacks: 4 }],
    receivedDaysAgo: 35,
    blocked: "purchasing.receipt_lot_consumed",
  },
];

export const PURCHASE_RECEIPTS: PurchaseReceipt[] = RECEIPT_SEEDS.map(makeReceipt);

/** The deliveries recorded against one order, newest first (the server's order). */
export function receiptsFor(poId: string): PurchaseReceipt[] {
  return PURCHASE_RECEIPTS.filter((r) => r.purchaseOrderId === poId).sort((a, b) =>
    b.receiptNo.localeCompare(a.receiptNo),
  );
}

// The one purchase return in the set: a dented carton of cooking oil went back,
// which is why po-credit reads as paid in full AND carries a supplier credit.
// The quantity is a whole pack on purpose — fmtUnitQty divides base units by
// the pack factor, so a part-pack return would render as "0.333 dus".
const CREDIT_LINE = PURCHASE_ORDERS_BY_ID.get("po-credit")!.items[0];

export const PURCHASE_RETURNS: PurchaseReturn[] = [
  new PurchaseReturn({
    id: "rtn-1",
    returnNo: "RTN-2026-0003",
    purchaseOrderId: "po-credit",
    warehouseId: WAREHOUSES[0].id,
    returnedAt: dateIn(-30),
    returnedBy: "user-admin",
    reason: "1 dus penyok, isinya bocor",
    refundAmount: 204_600n,
    createdAt: daysAgo(30),
    items: [
      new PurchaseReturnItem({
        id: "rtn-1-i1",
        purchaseReturnId: "rtn-1",
        purchaseReceiptItemId: "rcv-credit-1-i1",
        purchaseOrderItemId: CREDIT_LINE.id,
        productId: CREDIT_LINE.productId,
        batchId: "rcv-credit-1-b1",
        qty: 1 * Number(CREDIT_LINE.unitFactor),
        unitCostPrice: CREDIT_LINE.unitCostPrice,
        unitName: CREDIT_LINE.unitName,
        unitFactor: CREDIT_LINE.unitFactor,
      }),
    ],
  }),
];

export function returnsFor(poId: string): PurchaseReturn[] {
  return PURCHASE_RETURNS.filter((r) => r.purchaseOrderId === poId);
}

/**
 * GetSupplierBalances, derived from the orders above rather than typed out —
 * the ledger tab and the restock list are two views of one set of orders, and a
 * hand-written balance is how the two come to disagree.
 */
export function supplierBalances(onlyOutstanding = false): SupplierBalance[] {
  return SUPPLIERS.map((s) => {
    const orders = PURCHASE_ORDERS.filter(
      (po) => po.supplierId === s.id && po.status !== POStatus.PO_STATUS_VOIDED,
    );
    return new SupplierBalance({
      supplierId: s.id,
      supplierName: s.name,
      orderedTotal: orders.reduce((n, po) => n + po.orderedTotal, 0n),
      paidTotal: orders.reduce((n, po) => n + po.paidAmount, 0n),
      outstanding: orders.reduce((n, po) => n + po.outstanding, 0n),
      openPoCount: orders.filter(
        (po) => po.status !== POStatus.PO_STATUS_CLOSED && po.status !== POStatus.PO_STATUS_VOIDED,
      ).length,
    });
  }).filter((b) => !onlyOutstanding || b.outstanding > 0n);
}

/** SearchSuppliers, for the restock toolbar's supplier picker. */
export function filterSuppliers(query: string) {
  const q = query.trim().toLowerCase();
  return SUPPLIERS.filter(
    (s) => !q || s.name.toLowerCase().includes(q) || s.code.toLowerCase().includes(q),
  );
}
