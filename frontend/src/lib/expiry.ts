import { ExpirySource } from "../gen/inventory_iface/v1/batch_pb";
import { ExpiryDefault } from "../gen/inventory_iface/v1/product_pb";

// Expiry entry and expiry defaults — the logic behind <ExpiryInput> and the
// Receive dialog's pre-fill. Pure: no React, no i18n, so the rules live in one
// place and the stories can pin them.
//
// Dates on the wire are "YYYY-MM-DD" calendar dates with no zone. Everything
// here works in LOCAL calendar terms: `new Date("2027-03-31")` would parse as
// UTC midnight and show the previous day west of Greenwich.

/** A lot is "expiring soon" within this many days — the dashboard tile's window. */
export const EXPIRY_SOON_DAYS = 30;

export type ParsedExpiry = {
  /** "YYYY-MM-DD" */
  iso: string;
  /** Typed as month + year only; stored as the last day of that month. */
  monthOnly: boolean;
};

const pad2 = (n: number) => String(n).padStart(2, "0");
const isoOf = (y: number, m: number, d: number) => `${y}-${pad2(m)}-${pad2(d)}`;
/** Days in month `m` (1-based) of year `y`. Day 0 of the next month is the last of this one. */
const daysIn = (y: number, m: number) => new Date(y, m, 0).getDate();

/** The last day of month `m` (1-based) — what a month-only expiry means. */
export function endOfMonthIso(y: number, m: number): string {
  return isoOf(y, m, daysIn(y, m));
}

// Two-digit years are 20YY; four-digit ones must be 2000–2099. Narrowing the
// range is what makes six bare digits unambiguous (see parseExpiryInput).
function year4(s: string): number | null {
  if (s.length === 2) return 2000 + Number(s);
  if (s.length === 4) {
    const y = Number(s);
    return y >= 2000 && y <= 2099 ? y : null;
  }
  return null;
}

function monthOnly(mm: string, yy: string): ParsedExpiry | null {
  const m = Number(mm);
  const y = year4(yy);
  if (y == null || mm.length > 2 || m < 1 || m > 12) return null;
  return { iso: endOfMonthIso(y, m), monthOnly: true };
}

function exact(dd: string, mm: string, yy: string): ParsedExpiry | null {
  const d = Number(dd);
  const m = Number(mm);
  const y = year4(yy);
  if (y == null || dd.length > 2 || mm.length > 2 || m < 1 || m > 12) return null;
  if (d < 1 || d > daysIn(y, m)) return null;
  return { iso: isoOf(y, m, d), monthOnly: false };
}

/**
 * Read an expiry the way it is printed on an Indonesian pack: day first, or
 * month + year only. Separators are optional (/ . - or space).
 *
 *   "0327", "03/27", "3/2027", "032027"  → 31 Mar 2027 (month only)
 *   "050327", "05/03/27", "05032027"     → 5 Mar 2027
 *
 * Six bare digits are either MMYYYY or DDMMYY, and the 2000–2099 year range
 * decides: MMYYYY needs its last four digits to read 20xx, which as DDMMYY
 * would be month 20 — so at most one reading is ever valid.
 *
 * Returns null for anything incomplete or impossible (31/02, month 13).
 */
export function parseExpiryInput(text: string): ParsedExpiry | null {
  const t = text.trim();
  if (!t) return null;
  const parts = t.split(/[\s/.\-]+/).filter(Boolean);
  if (parts.length === 0 || parts.some((p) => !/^\d+$/.test(p))) return null;
  if (parts.length === 1) {
    const s = parts[0];
    if (s.length === 4) return monthOnly(s.slice(0, 2), s.slice(2));
    if (s.length === 6) return monthOnly(s.slice(0, 2), s.slice(2)) ?? exact(s.slice(0, 2), s.slice(2, 4), s.slice(4));
    if (s.length === 8) return exact(s.slice(0, 2), s.slice(2, 4), s.slice(4));
    return null;
  }
  if (parts.length === 2) return monthOnly(parts[0], parts[1]);
  if (parts.length === 3) return exact(parts[0], parts[1], parts[2]);
  return null;
}

/** "2027-03-31" → "31/03/2027", the form <ExpiryInput> shows a value it was given. */
export function formatExpiryText(iso: string): string {
  const [y, m, d] = iso.split("-");
  return y && m && d ? `${d}/${m}/${y}` : "";
}

/** The calendar date as a LOCAL Date (midnight), for display and day math. */
export function isoToLocalDate(iso: string): Date {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, (m || 1) - 1, d || 1);
}

/** Whole calendar days from today to the expiry; negative once it has passed. */
export function daysUntilExpiry(iso: string, now: Date = new Date()): number {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  return Math.round((isoToLocalDate(iso).getTime() - today.getTime()) / 86_400_000);
}

/** The product fields a default is computed from — on both Product and ProductExpiryDefault. */
export type ExpiryDefaultConfig = {
  expiryDefault: ExpiryDefault;
  expiryDefaultMonths: number;
  prescriptionRequired: boolean;
};

export type ExpirySeed = {
  /** "YYYY-MM-DD"; "" for NONE, where no date is shown or sent. */
  iso: string;
  source: ExpirySource.DEFAULT | ExpirySource.NONE;
};

/**
 * What a new lot's expiry is pre-filled with, or null when the field starts
 * empty and must be typed. The server only stores the product's setting — this
 * is the one place the date is computed.
 *
 * MONTHS = received date + N months, snapped to the END of that month (packs
 * print month/year). In pharmacy mode a prescription medicine is never
 * pre-filled at all: an estimate that runs late is how expired stock gets sold,
 * so those dates are always read off the pack.
 */
export function defaultExpiry(
  cfg: ExpiryDefaultConfig | undefined,
  receivedAtIso: string,
  isPharmacy: boolean,
): ExpirySeed | null {
  if (!cfg) return null;
  if (isPharmacy && cfg.prescriptionRequired) return null;
  if (cfg.expiryDefault === ExpiryDefault.NONE) return { iso: "", source: ExpirySource.NONE };
  if (cfg.expiryDefault !== ExpiryDefault.MONTHS || cfg.expiryDefaultMonths <= 0) return null;
  const [y, m] = receivedAtIso.split("-").map(Number);
  if (!y || !m) return null;
  const idx = m - 1 + cfg.expiryDefaultMonths;
  return {
    iso: endOfMonthIso(y + Math.floor(idx / 12), (idx % 12) + 1),
    source: ExpirySource.DEFAULT,
  };
}
