import { formatMoney } from "./format";
import i18n from "./i18n";

// Chart styling, expressed in Chakra `defaultSystem` tokens.
//
// Recharts draws raw SVG and can't read Chakra style props, so every visual
// value here is the CSS variable Chakra already emits for a stock token
// (`var(--chakra-colors-…)`). Those variables flip on the `data-theme`
// attribute the preferences store sets, so the charts follow light/dark with no
// JS colour-mode branch, no custom system, and no hardcoded hex — the same
// "Chakra defaultSystem only" rule the rest of the UI follows.
//
// Import these from the shared chart components (ChartCard / TrendChart)
// instead of restyling a Recharts element per page.

export const chartTokens = {
  /** Horizontal grid lines. Recessive — sits behind the marks. */
  grid: "var(--chakra-colors-border)",
  /** Axis tick labels. Text wears text tokens, never a series colour. */
  tick: "var(--chakra-colors-fg-muted)",
  /** Hover crosshair. */
  cursor: "var(--chakra-colors-border-emphasized)",
  /** The chart surface — also the ring that lifts a hovered dot off the line. */
  surface: "var(--chakra-colors-bg-subtle)",
  /** Chakra's `xs` font size, for tick + legend text. */
  fontSize: "var(--chakra-font-sizes-xs)",
} as const;

// Categorical series colours, in FIXED order: series 1 is always blue (the
// app's accent, so "revenue" reads blue on every surface), then orange, teal,
// purple. Never cycle and never reorder by rank — the colour belongs to the
// entity, so hiding one series must not repaint the others. A 5th series is not
// a generated hue: fold the tail into "Other", or split into small multiples.
//
// These are Chakra's own 600 steps (#2563eb, #ea580c, #0d9488, #9333ea), which
// resolve to the same value in light and dark, so one token works on both
// surfaces. Checked with the dataviz palette validator against `bg.subtle` in
// both modes — lightness band, chroma floor, CVD separation (worst adjacent
// ΔE 13.8, protan), normal-vision floor (ΔE 28.8) and ≥3:1 contrast all pass.
// Re-run the validator before changing a step or adding a hue.
export const CHART_SERIES = [
  "var(--chakra-colors-blue-600)",
  "var(--chakra-colors-orange-600)",
  "var(--chakra-colors-teal-600)",
  "var(--chakra-colors-purple-600)",
] as const;

export function seriesColor(index: number): string {
  return CHART_SERIES[index] ?? CHART_SERIES[CHART_SERIES.length - 1];
}

function currentLocale(): string {
  return i18n.language || "id";
}

const compactMap = new Map<string, Intl.NumberFormat>();
const countMap = new Map<string, Intl.NumberFormat>();

function formatter(cache: Map<string, Intl.NumberFormat>, opts: Intl.NumberFormatOptions) {
  const locale = currentLocale();
  let fmt = cache.get(locale);
  if (!fmt) {
    fmt = new Intl.NumberFormat(locale, opts);
    cache.set(locale, fmt);
  }
  return fmt;
}

// Axis ticks are compact so the Y axis stays narrow and never truncates:
// id -> "1,3 jt", en -> "1.3M". The currency symbol is dropped (the chart title
// names the metric); the tooltip still shows the exact formatted amount.
export function formatAxisTick(value: number): string {
  return formatter(compactMap, { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

/** Grouped integer, 0 included (unlike formatThousands, which renders 0 as ""). */
export function formatCount(value: number): string {
  return formatter(countMap, { maximumFractionDigits: 0 }).format(value);
}

/** Exact value for tooltips + direct labels. */
export function formatChartValue(value: number, money?: boolean): string {
  return money ? formatMoney(value) : formatCount(value);
}
