import type { JsonValue } from "@bufbuild/protobuf";

import { ProductUnit } from "../gen/inventory_iface/v1/product_pb";
import type { Line, PurchaseOrderSeed } from "./purchaseLine";

// The unfinished NEW restock order, kept in this browser so that leaving the
// page, a refresh or a closed tab does not cost the buyer a half-typed faktur.
//
// Browser-only on purpose: nothing reaches the server until Buat, which is
// what creates the DRAFT order (and a DRAFT is already editable from any PC).
// This covers the gap before that, while Buat is still disabled because a line
// is incomplete. One draft per USER, since a shop PC is shared and one buyer's
// half-entered invoice is not the next person's.
//
// Line.units are protobuf messages (bigint fields), which JSON.stringify
// cannot write, so they round-trip through toJson / fromJson.

const KEY_PREFIX = "justmart_restock_draft:";
// Bump when Line or PurchaseOrderSeed changes shape: an older draft is then
// dropped instead of seeding the form with fields it no longer has.
const VERSION = 1;

export type RestockDraft = { savedAt: number; seed: PurchaseOrderSeed };

type StoredLine = Omit<Line, "units"> & { units: JsonValue[] };
type Stored = {
  v: number;
  savedAt: number;
  seed: Omit<PurchaseOrderSeed, "lines"> & { lines: StoredLine[] };
};

const keyFor = (userId: string) => KEY_PREFIX + userId;

/** Whether the form holds anything worth offering back. */
export function hasRestockInput(s: PurchaseOrderSeed): boolean {
  return (
    !!s.supplierId ||
    s.invoiceNo.trim() !== "" ||
    !!s.invoiceDate ||
    !!s.dueAt ||
    s.note.trim() !== "" ||
    s.lines.length > 0 ||
    s.cartDiscount > 0 ||
    s.ppnEnabled
  );
}

/** Keep `seed` for `userId`; an empty form removes the kept draft instead. */
export function saveRestockDraft(userId: string, seed: PurchaseOrderSeed, now = Date.now()) {
  if (!userId) return;
  if (!hasRestockInput(seed)) {
    clearRestockDraft(userId);
    return;
  }
  const stored: Stored = {
    v: VERSION,
    savedAt: now,
    seed: { ...seed, lines: seed.lines.map((l) => ({ ...l, units: l.units.map((u) => u.toJson()) })) },
  };
  try {
    localStorage.setItem(keyFor(userId), JSON.stringify(stored));
  } catch {
    // Storage full or disabled: keeping a draft is best-effort, never an error.
  }
}

/**
 * The kept draft, or null. A malformed or older-version entry is removed, so a
 * bad value cannot wedge every later visit to the form.
 */
export function loadRestockDraft(userId: string): RestockDraft | null {
  if (!userId) return null;
  const raw = localStorage.getItem(keyFor(userId));
  if (!raw) return null;
  try {
    const s = JSON.parse(raw) as Stored;
    if (s.v !== VERSION || !s.seed || !Array.isArray(s.seed.lines)) throw new Error("stale draft");
    const seed: PurchaseOrderSeed = {
      ...s.seed,
      lines: s.seed.lines.map((l) => ({ ...l, units: l.units.map((u) => ProductUnit.fromJson(u)) })),
    };
    return hasRestockInput(seed) ? { savedAt: s.savedAt, seed } : null;
  } catch {
    clearRestockDraft(userId);
    return null;
  }
}

export function clearRestockDraft(userId: string) {
  if (userId) localStorage.removeItem(keyFor(userId));
}
