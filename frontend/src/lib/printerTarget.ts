// Shared receipt-printer target (connector mode). The cashier's chosen printer
// is persisted per device under this key so it sticks across sales — and so
// reprints from order history target the same printer POS uses. Value is
// "<deviceId>|<printerName>", or "" for Auto (server resolves the saved default
// / sole connected device / TCP).
export const POS_PRINTER_KEY = "justmart_pos_printer";

// decodePrinter splits the persisted "<deviceId>|<printerName>" value. Split on
// the FIRST "|" — deviceId is a uuid (no "|"); a printer name may contain one.
export function decodePrinter(v: string): { deviceId: string; printerName: string } {
  if (!v) return { deviceId: "", printerName: "" };
  const i = v.indexOf("|");
  return i < 0
    ? { deviceId: v, printerName: "" }
    : { deviceId: v.slice(0, i), printerName: v.slice(i + 1) };
}

// savedPrinterTarget reads the persisted POS printer choice. An empty result
// (no choice, or a TCP/no-connector shop) lets the server resolve the saved
// default / sole connected device / TCP address.
export function savedPrinterTarget(): { deviceId: string; printerName: string } {
  return decodePrinter(localStorage.getItem(POS_PRINTER_KEY) ?? "");
}
