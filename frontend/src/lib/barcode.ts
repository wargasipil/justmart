import JsBarcode from "jsbarcode";

// CODE128 is the symbology for both label paths. On the thermal printer the
// firmware encodes it (the backend only frames the payload with `GS k`); here in
// the browser JsBarcode does it. Deliberately a vetted library rather than a
// hand-rolled module table: a wrong table produces a barcode that prints
// perfectly and scans as a different string.

/** The barcode's visual knobs. Defaults suit an on-screen preview. */
export type BarcodeOptions = {
  /** Narrow-module width in px. Higher = wider, more scannable. */
  width?: number;
  /** Bar height in px. */
  height?: number;
  /** Print the value as text under the bars. */
  displayValue?: boolean;
  /** Font size of that text, px. */
  fontSize?: number;
  /** Quiet-zone margin in px. Scanners need one; 10 is the library default. */
  margin?: number;
};

/**
 * Whether `value` can be carried by CODE128 code set B (printable ASCII,
 * 0x20-0x7E).
 *
 * Mirrors `printer.Code128Encodable` in the Go backend — the two MUST agree, or
 * the UI offers a Print button for a SKU the server then refuses. Empty is not
 * encodable: there is nothing to scan.
 */
export function isCode128Encodable(value: string): boolean {
  if (!value) return false;
  for (let i = 0; i < value.length; i++) {
    const c = value.charCodeAt(i);
    if (c < 0x20 || c > 0x7e) return false;
  }
  return true;
}

/**
 * Renders `value` into `svg` as a CODE128 barcode. Returns false (leaving the
 * element empty) when the value cannot be encoded, so callers render a fallback
 * instead of showing a scannable-looking symbol that isn't.
 *
 * Bars are always black on white, in both themes: a barcode is an optical
 * target, not decoration — inverting it in dark mode makes it unscannable and
 * prints as a black rectangle.
 */
export function renderBarcodeInto(
  svg: SVGSVGElement,
  value: string,
  opts: BarcodeOptions = {},
): boolean {
  if (!isCode128Encodable(value)) return false;
  try {
    JsBarcode(svg, value, {
      format: "CODE128",
      width: opts.width ?? 2,
      height: opts.height ?? 60,
      displayValue: opts.displayValue ?? true,
      fontSize: opts.fontSize ?? 14,
      margin: opts.margin ?? 10,
      background: "#ffffff",
      lineColor: "#000000",
    });
    return true;
  } catch {
    // JsBarcode throws on input it cannot encode. isCode128Encodable should
    // have caught that already; this is the belt-and-braces path.
    return false;
  }
}

/**
 * Builds standalone `<svg>` markup for a barcode, for injection into a
 * document we don't render with React (the print sheet). Returns "" when the
 * value is not encodable.
 */
export function barcodeSvgMarkup(
  value: string,
  opts: BarcodeOptions = {},
): string {
  // A detached element is enough — JsBarcode only sets attributes and appends
  // children, it never measures layout.
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  if (!renderBarcodeInto(svg, value, opts)) return "";
  return new XMLSerializer().serializeToString(svg);
}
