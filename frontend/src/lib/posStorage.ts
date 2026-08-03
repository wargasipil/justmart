// Per-device POS preferences + the scratch state that survives the create-resep
// round-trip. Split out of routes/Pos.tsx so the page holds no raw localStorage
// key strings. The receipt-printer target lives in lib/printerTarget.ts (it is
// shared with order-history reprint); everything here is POS-only.

// The in-progress DRAFT cart, preserved across POS → /prescriptions/new → POS.
// Without it the POS unmount cleanup would hard-delete the draft.
const POS_DRAFT_KEY = "justmart_pos_draft";
// The Rx-required product whose add was deferred pending a covering
// prescription, so it can be re-added once the new resep is attached on return.
const POS_DEFERRED_KEY = "justmart_pos_deferred";
// Whether the search list includes rows the cashier can't sell (zero stock in
// the active warehouse). Defaults ON (they stay visible, dimmed) so a cashier
// can still confirm a product exists; per device, since it's a per-till
// preference rather than a shop-wide setting.
const POS_SHOW_OOS_KEY = "justmart_pos_show_oos";

export type DeferredAdd = { productId: string; unitId: string };

/** Persist the draft cart + the pending Rx-required product before routing away. */
export function saveResepRoundTrip(saleId: string | null, deferred: DeferredAdd | null) {
  if (saleId) localStorage.setItem(POS_DRAFT_KEY, saleId);
  if (deferred) localStorage.setItem(POS_DEFERRED_KEY, JSON.stringify(deferred));
  else localStorage.removeItem(POS_DEFERRED_KEY);
}

/**
 * Read back and clear what saveResepRoundTrip stored. Always clears, even on a
 * malformed payload, so a bad value can't wedge every subsequent POS mount.
 */
export function takeResepRoundTrip(): { saleId: string; deferred: DeferredAdd | null } {
  const saleId = localStorage.getItem(POS_DRAFT_KEY) ?? "";
  const raw = localStorage.getItem(POS_DEFERRED_KEY);
  localStorage.removeItem(POS_DRAFT_KEY);
  localStorage.removeItem(POS_DEFERRED_KEY);
  let deferred: DeferredAdd | null = null;
  if (raw) {
    try {
      deferred = JSON.parse(raw) as DeferredAdd;
    } catch {
      /* malformed — treat as absent */
    }
  }
  return { saleId, deferred };
}

export function loadShowOutOfStock(): boolean {
  return localStorage.getItem(POS_SHOW_OOS_KEY) !== "0";
}

export function saveShowOutOfStock(show: boolean) {
  localStorage.setItem(POS_SHOW_OOS_KEY, show ? "1" : "0");
}
