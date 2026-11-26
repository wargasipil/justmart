import { useMemo, useState } from "react";

import type { Line, PurchaseOrderSeed } from "../../lib/purchaseLine";

/**
 * Every editable field of the restock form, plus `snapshot`: the same state
 * read back as one PurchaseOrderSeed (what the new-order page keeps in the
 * browser). Lifted out of PurchaseOrderForm so that file holds the layout and
 * the actions.
 *
 * `seed` is read at MOUNT only, like before the move: see the seeding note on
 * PurchaseOrderForm.
 */
export function usePurchaseOrderState(seed?: PurchaseOrderSeed) {
  const [supplierId, setSupplierId] = useState(seed?.supplierId ?? "");
  const [invoiceNo, setInvoiceNo] = useState(seed?.invoiceNo ?? "");
  const [invoiceDate, setInvoiceDate] = useState(seed?.invoiceDate ?? "");
  const [dueAt, setDueAt] = useState(seed?.dueAt ?? "");
  const [note, setNote] = useState(seed?.note ?? "");
  const [lines, setLines] = useState<Line[]>(seed?.lines ?? []);
  const [cartDiscount, setCartDiscount] = useState(seed?.cartDiscount ?? 0);
  const [ppnEnabled, setPpnEnabled] = useState(seed?.ppnEnabled ?? false);
  // percent; current Indonesian default
  const [ppnRate, setPpnRate] = useState(seed?.ppnRate ?? 11);

  const snapshot = useMemo<PurchaseOrderSeed>(
    () => ({ supplierId, invoiceNo, invoiceDate, dueAt, note, lines, cartDiscount, ppnEnabled, ppnRate }),
    [supplierId, invoiceNo, invoiceDate, dueAt, note, lines, cartDiscount, ppnEnabled, ppnRate],
  );

  return {
    supplierId, setSupplierId,
    invoiceNo, setInvoiceNo,
    invoiceDate, setInvoiceDate,
    dueAt, setDueAt,
    note, setNote,
    lines, setLines,
    cartDiscount, setCartDiscount,
    ppnEnabled, setPpnEnabled,
    ppnRate, setPpnRate,
    snapshot,
  };
}
