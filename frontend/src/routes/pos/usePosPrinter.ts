// Receipt-printer selection for the POS header (connector mode). Split out of
// routes/Pos.tsx; behaviour unchanged.
//
// Live list of connectors + their printers (polls 5s); the cashier picks the
// print device from the header. The choice persists per device and drives the
// receipt Print. The header picker is shown only when a connector printer is
// available — TCP/no-connector shops never see it. "" = Auto (the server
// resolves the saved default / sole connector).
import { useCallback, useEffect, useMemo, useState } from "react";

import { POS_PRINTER_KEY, decodePrinter } from "../../lib/printerTarget";
import { useConnectorsQuery } from "../../queries/connectors";

export function usePosPrinter() {
  const connectorsQ = useConnectorsQuery();
  const connectors = useMemo(() => connectorsQ.data ?? [], [connectorsQ.data]);
  const hasPrinters = connectors.some((c) => c.printerNames.length > 0);
  const [printerValue, setPrinterValue] = useState<string>(
    () => localStorage.getItem(POS_PRINTER_KEY) ?? "",
  );

  // Drop the persisted choice if that device/printer is no longer connected.
  useEffect(() => {
    if (!printerValue) return;
    const { deviceId, printerName } = decodePrinter(printerValue);
    const stillThere = connectors.some(
      (c) => c.deviceId === deviceId && c.printerNames.includes(printerName),
    );
    if (!stillThere) {
      setPrinterValue("");
      localStorage.removeItem(POS_PRINTER_KEY);
    }
  }, [connectors, printerValue]);

  const onPickPrinter = useCallback((v: string) => {
    setPrinterValue(v);
    if (v) localStorage.setItem(POS_PRINTER_KEY, v);
    else localStorage.removeItem(POS_PRINTER_KEY);
  }, []);

  return {
    connectors,
    hasPrinters,
    printerValue,
    onPickPrinter,
    /** The decoded target to hand to <ReceiptDialog>. */
    printerTarget: decodePrinter(printerValue),
  };
}
