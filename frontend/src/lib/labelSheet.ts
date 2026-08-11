import { barcodeSvgMarkup } from "./barcode";

// The browser half of "print barcode": lays labels out as a grid on ordinary
// paper (sticker sheets, A4/Letter) and hands it to the OS print dialog. The
// thermal half goes through PrintProductLabel on the backend instead.

export type LabelSpec = {
  name: string;
  sku: string;
  /** Selling unit the price belongs to; "" hides the suffix. */
  unitName: string;
  /** Already formatted for the active locale by the caller (formatMoney). */
  price: string;
};

export type SheetOptions = {
  /** Labels per row. */
  columns: number;
  /** How many identical labels to lay out. */
  copies: number;
};

/** Escapes text for safe interpolation into the generated document. */
function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

/**
 * Builds the full HTML document for a sheet of labels.
 *
 * Exported for testing/preview; `printLabelSheet` is what callers use.
 * Returns "" when the SKU cannot be encoded as CODE128 — callers should have
 * blocked that earlier, and printing a label with no barcode is worse than not
 * printing.
 */
export function buildLabelSheetHtml(
  label: LabelSpec,
  opts: SheetOptions,
  title: string,
): string {
  // Generate the symbol ONCE and repeat the markup: encoding is deterministic,
  // so N copies of one string beats N JsBarcode runs.
  const svg = barcodeSvgMarkup(label.sku, {
    width: 2,
    height: 50,
    displayValue: true,
    fontSize: 13,
    margin: 4,
  });
  if (!svg) return "";

  const priceText = label.unitName
    ? `${label.price} / ${label.unitName}`
    : label.price;

  const cell = `
    <div class="label">
      <div class="name">${escapeHtml(label.name)}</div>
      <div class="code">${svg}</div>
      <div class="price">${escapeHtml(priceText)}</div>
    </div>`;

  const copies = Math.max(1, opts.copies);
  const columns = Math.max(1, opts.columns);

  return `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>${escapeHtml(title)}</title>
<style>
  @page { margin: 8mm; }
  /* Force the actual colors: browsers drop backgrounds when printing unless
     told otherwise, and a barcode needs its white quiet zone. */
  * { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  body {
    margin: 0;
    background: #fff;
    color: #000;
    font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  }
  .sheet {
    display: grid;
    grid-template-columns: repeat(${columns}, 1fr);
    gap: 4mm;
  }
  .label {
    border: 1px dashed #bbb;   /* cut guide; prints faint */
    border-radius: 2mm;
    padding: 2mm;
    text-align: center;
    break-inside: avoid;
    page-break-inside: avoid;
    background: #fff;
  }
  .name {
    font-size: 10pt;
    font-weight: 600;
    line-height: 1.2;
    margin-bottom: 1mm;
    /* Long names wrap rather than pushing the barcode out of the cell. */
    overflow-wrap: anywhere;
  }
  .code svg { max-width: 100%; height: auto; }
  .price { font-size: 12pt; font-weight: 700; margin-top: 1mm; }
</style>
</head>
<body>
  <div class="sheet">${cell.repeat(copies)}</div>
</body>
</html>`;
}

/**
 * Renders the sheet into a hidden iframe and opens the OS print dialog.
 *
 * An iframe rather than window.open: a popup blocker can silently swallow a new
 * window, and an isolated document also keeps the app's own styles (and Chakra's
 * portals) from leaking into the printout.
 *
 * Returns false when the label could not be built (unencodable SKU).
 */
export function printLabelSheet(
  label: LabelSpec,
  opts: SheetOptions,
  title: string,
): boolean {
  const html = buildLabelSheetHtml(label, opts, title);
  if (!html) return false;

  const iframe = document.createElement("iframe");
  iframe.setAttribute("aria-hidden", "true");
  iframe.setAttribute("title", title);
  Object.assign(iframe.style, {
    position: "fixed",
    right: "0",
    bottom: "0",
    width: "0",
    height: "0",
    border: "0",
    visibility: "hidden",
  });

  iframe.onload = () => {
    const win = iframe.contentWindow;
    if (!win) {
      iframe.remove();
      return;
    }
    let done = false;
    const cleanup = () => {
      if (done) return;
      done = true;
      // Detach on the next tick: removing the frame from inside its own
      // afterprint handler can cancel the job on some engines.
      window.setTimeout(() => iframe.remove(), 0);
    };
    // print() is modal in Chrome but returns immediately in Firefox, so the
    // frame must outlive the call — afterprint is the real completion signal,
    // with a long timeout in case it never fires.
    win.addEventListener("afterprint", cleanup, { once: true });
    window.setTimeout(cleanup, 60_000);
    win.focus();
    win.print();
  };

  iframe.srcdoc = html;
  document.body.appendChild(iframe);
  return true;
}
