// Pricing helpers for the markup/margin affordance in the product form + detail.
// Convention (matches the analytics margin definition): markup % is over COST,
// margin % is over SELL. Money is whole rupiah (BIGINT minor units).

// marginPct returns the margin percentage of `sell` over `cost`, or null when it
// can't be computed (unknown/zero cost or non-positive price) — callers render "—".
export function marginPct(
  sell: number | bigint | string,
  cost: number | bigint,
): number | null {
  const s = Number(typeof sell === "string" ? sell || "0" : sell);
  const c = Number(cost);
  if (c <= 0 || s <= 0) return null;
  return ((s - c) / s) * 100;
}

// marginValue returns the absolute margin (sell − cost) in whole rupiah, or null
// under exactly the same conditions marginPct returns null — so a caller can
// render the pair "Rp 5.000 (20%)" and never end up with one half showing "—".
export function marginValue(
  sell: number | bigint | string,
  cost: number | bigint,
): bigint | null {
  const s = Number(typeof sell === "string" ? sell || "0" : sell);
  const c = Number(cost);
  if (c <= 0 || s <= 0) return null;
  return BigInt(Math.round(s - c));
}

// priceFromMarkup derives a whole-rupiah sell price from a base cost + markup %.
// Returns 0n when cost is non-positive or the markup isn't a finite number.
export function priceFromMarkup(cost: number | bigint, markupPct: number): bigint {
  const c = Number(cost);
  if (c <= 0 || !Number.isFinite(markupPct)) return 0n;
  return BigInt(Math.round(c * (1 + markupPct / 100)));
}
