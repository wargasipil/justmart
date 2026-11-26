import type { PartialMessage } from "@bufbuild/protobuf";
import { useState } from "react";

import { ExpirySource } from "../../gen/inventory_iface/v1/batch_pb";
import type { PurchaseOrder, PurchaseOrderItem } from "../../gen/purchasing_iface/v1/order_pb";
import type { PurchaseReceipt, ReceiveLineInput } from "../../gen/purchasing_iface/v1/receipt_pb";
import { formatDateOnly } from "../../lib/dateRange";
import { daysUntilExpiry, defaultExpiry } from "../../lib/expiry";
import { useProductExpiryDefaultsQuery } from "../../queries/products";
import { useBusinessMode } from "../../queries/settings";

// The Terima barang dialog's draft: the header fields, one line per row, and the
// rules that decide what a row will be saved with. Lifted out of ReceiveDialog
// so the dialog is only its JSX; page-local, like the dialog.
//
// Two modes share it. A NEW delivery starts from the order's outstanding lines.
// Editing a delivery ("ubah penerimaan", `replacing` set) starts from what that
// delivery recorded, because ReplaceReceipt cancels it and receives the
// corrected lines in its place.

// No unit cost here by design: it is fully determined by the PO line (net of
// its discount, plus PPN) and the backend derives it — see common.NetUnitCost.
// The request therefore omits unit_cost_price, whose 0 default IS the "derive
// it" signal. Re-adding an input would only let a receive contradict the order
// it is fulfilling; the proto field survives as an API-level escape hatch. An
// edited delivery is re-derived the same way: the cost a receipt line stores is
// the derived one, and sending it back as an override would apply PPN twice.
export type LineDraft = {
  /** Row identity: the PO line, or — editing — the replaced receipt line. */
  key: string;
  purchaseOrderItemId: string;
  productId: string;
  qty: number; // in the line's purchasable unit
  batchNumber: string;
  // What was TYPED. Until a person types, the line shows its seed (editing) or
  // its product's default (derived each render, so changing the received date
  // moves it).
  expiryTyped: string;
  expiryTouched: boolean;
  /**
   * Editing: the expiry the replaced delivery recorded, with its source, so an
   * untouched line keeps it exactly — a product default stays DEFAULT (on the
   * shelf-check worklist) and "does not expire" stays NONE.
   */
  expirySeed?: { iso: string; source: ExpirySource };
  /**
   * The most this PO line can take, in the purchasable unit: what is still
   * outstanding — plus, editing, what the replaced delivery brought, since the
   * cancel half gives that back before the corrected lines are checked.
   */
  remaining: number;
  unitName: string;
  unitFactor: number;
  productUnitId: string;
};

type Header = { receivedAt: string; note: string; invoiceNo: string; reason: string };

function initialLines(items: PurchaseOrderItem[]): LineDraft[] {
  return items
    .filter((it) => it.receivedQty < it.orderedQty)
    .map((it) => {
      const factor = Number(it.unitFactor) || 1;
      const remainingUnit = (it.orderedQty - it.receivedQty) / factor;
      return {
        key: it.id,
        purchaseOrderItemId: it.id,
        productId: it.productId,
        qty: remainingUnit,
        batchNumber: "",
        expiryTyped: "",
        expiryTouched: false,
        remaining: remainingUnit,
        unitName: it.unitName,
        unitFactor: factor,
        productUnitId: it.productUnitId,
      };
    });
}

function replaceLines(items: PurchaseOrderItem[], receipt: PurchaseReceipt): LineDraft[] {
  // What the replaced delivery brought per PO line, in BASE units.
  const delivered = new Map<string, number>();
  for (const it of receipt.items) {
    delivered.set(it.purchaseOrderItemId, (delivered.get(it.purchaseOrderItemId) ?? 0) + it.qty);
  }
  const fromReceipt = receipt.items.map((it): LineDraft => {
    const poItem = items.find((p) => p.id === it.purchaseOrderItemId);
    const factor = Number(it.unitFactor) || 1;
    const capacity = poItem
      ? poItem.orderedQty - poItem.receivedQty + (delivered.get(poItem.id) ?? 0)
      : it.qty;
    return {
      key: it.id,
      purchaseOrderItemId: it.purchaseOrderItemId,
      productId: it.productId,
      qty: it.qty / factor,
      batchNumber: it.batchNumber,
      expiryTyped: "",
      expiryTouched: false,
      expirySeed: {
        iso: it.expiryDate,
        // UNSPECIFIED is a lot written before the source was recorded: typed.
        source: it.expirySource === ExpirySource.UNSPECIFIED ? ExpirySource.ENTERED : it.expirySource,
      },
      remaining: capacity / factor,
      unitName: it.unitName,
      unitFactor: factor,
      productUnitId: it.productUnitId,
    };
  });
  // Lines still owed that this delivery did not cover, offered at 0 ("not in
  // this delivery"): the correction may be a product that arrived but was left
  // off.
  const others = initialLines(items.filter((p) => !delivered.has(p.id))).map((l) => ({ ...l, qty: 0 }));
  return [...fromReceipt, ...others];
}

function seed(po: PurchaseOrder, replacing: PurchaseReceipt | null | undefined) {
  if (replacing) {
    return {
      lines: replaceLines(po.items, replacing),
      header: {
        receivedAt: replacing.receivedAt,
        note: replacing.note,
        invoiceNo: replacing.invoiceNo,
        reason: "",
      },
    };
  }
  return {
    lines: initialLines(po.items),
    // The LOCAL calendar date. toISOString() is UTC, which on a UTC+7 shop made
    // anything received before 07:00 default to yesterday.
    header: { receivedAt: formatDateOnly(new Date()), note: "", invoiceNo: "", reason: "" },
  };
}

export function useReceiveDraft(
  open: boolean,
  po: PurchaseOrder,
  replacing: PurchaseReceipt | null | undefined,
) {
  const { isPharmacy } = useBusinessMode();
  const [header, setHeader] = useState<Header>(() => seed(po, replacing).header);
  const [lines, setLines] = useState<LineDraft[]>(() => seed(po, replacing).lines);
  // Bumped on every open so each line's <ExpiryInput> remounts with no
  // half-typed text left over from the last delivery.
  const [generation, setGeneration] = useState(0);

  // Each product's expiry setting, fetched in one read. The server only stores
  // the setting; the date is computed here (defaultExpiry) and seeded into the
  // line until somebody types over it.
  const defaultsQ = useProductExpiryDefaultsQuery(
    po.items.map((it) => it.productId),
    { enabled: open },
  );

  // Re-derive everything on each closed → open edge. The dialog stays mounted,
  // so without this a second delivery reopened on the previous one's draft:
  // the old "remaining", batch number and expiry, ready to be submitted again
  // as a new lot. Done during render (not in an effect) so the first open frame
  // already shows the fresh lines.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      const fresh = seed(po, replacing);
      setLines(fresh.lines);
      setHeader(fresh.header);
      setGeneration((g) => g + 1);
    }
  }

  // The expiry a line will be saved with, and where it came from.
  const lineExpiry = (l: LineDraft): { iso: string; source: ExpirySource; isDefault: boolean } => {
    if (l.expiryTouched) return { iso: l.expiryTyped, source: ExpirySource.ENTERED, isDefault: false };
    if (l.expirySeed) return { ...l.expirySeed, isDefault: l.expirySeed.source === ExpirySource.DEFAULT };
    const seeded = defaultExpiry(defaultsQ.byId.get(l.productId), header.receivedAt, isPharmacy);
    if (!seeded) return { iso: "", source: ExpirySource.ENTERED, isDefault: false };
    return { ...seeded, isDefault: seeded.source === ExpirySource.DEFAULT };
  };
  // Ready to save: a "does not expire" line needs nothing; any other needs a
  // date, and in pharmacy mode one that has not already passed (the server
  // refuses it too — retail only warns, in the readout).
  const expiryReady = (l: LineDraft) => {
    const e = lineExpiry(l);
    if (e.source === ExpirySource.NONE) return true;
    if (!e.iso) return false;
    return !(isPharmacy && daysUntilExpiry(e.iso) < 0);
  };

  // A line at 0 is "not in this delivery" and is left off the request, so it
  // stays outstanding on the order. Only the lines that arrived need an expiry.
  const arriving = lines.filter((l) => l.qty > 0);
  // Summed per PO line, in base units: an edited delivery may carry two lots of
  // one line, and together they may not exceed what that line can take.
  const used = new Map<string, number>();
  for (const l of arriving) {
    used.set(l.purchaseOrderItemId, (used.get(l.purchaseOrderItemId) ?? 0) + l.qty * l.unitFactor);
  }
  const withinRemaining = arriving.every(
    (l) => (used.get(l.purchaseOrderItemId) ?? 0) <= l.remaining * l.unitFactor,
  );
  const canSubmit =
    arriving.length > 0 &&
    withinRemaining &&
    arriving.every(expiryReady) &&
    (!replacing || header.reason.trim() !== "");

  const requestLines = (): PartialMessage<ReceiveLineInput>[] =>
    arriving.map((l) => {
      const e = lineExpiry(l);
      return {
        purchaseOrderItemId: l.purchaseOrderItemId,
        qty: l.qty,
        batchNumber: l.batchNumber,
        expiryDate: e.iso,
        expirySource: e.source,
        productUnitId: l.productUnitId,
      };
    });

  const updateLine = (idx: number, patch: Partial<LineDraft>) =>
    setLines((cur) => cur.map((l, i) => (i === idx ? { ...l, ...patch } : l)));
  const patchHeader = (patch: Partial<Header>) => setHeader((cur) => ({ ...cur, ...patch }));

  return {
    isPharmacy,
    header,
    patchHeader,
    lines,
    updateLine,
    generation,
    lineExpiry,
    canSubmit,
    requestLines,
  };
}
