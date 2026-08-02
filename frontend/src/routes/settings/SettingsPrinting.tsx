import { Badge, Box, Button, Heading, HStack, Spinner, Stack, Text, Textarea } from "@chakra-ui/react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import EnumSelect from "../../components/EnumSelect";
import { toast } from "../../lib/toaster";
import {
  useConnectorsQuery,
  useKitchenPrintTargetQuery,
  usePrintingInfoQuery,
  usePrintTargetQuery,
  useReceiptSettingsQuery,
  useSetKitchenPrintTargetMutation,
  useSetPrintTargetMutation,
  useSetReceiptSettingsMutation,
} from "../../queries/connectors";
import { useBusinessMode } from "../../queries/settings";

type Option = { value: string; label: string };

// SettingsPrinting (OWNER) — mode-aware printing config:
//   connector → pick the default connector + printer the POS Print button uses;
//   usb       → pick the local (server-host) printer for direct spooling;
//   tcp       → network printer is configured in config.yaml (info only).
// Plus the printed-receipt header/footer + paper width (all modes).
export default function SettingsPrinting() {
  const { t } = useTranslation();
  const infoQ = usePrintingInfoQuery();
  const connectorsQ = useConnectorsQuery();
  const targetQ = usePrintTargetQuery();
  const save = useSetPrintTargetMutation();

  const mode = infoQ.data?.mode ?? "tcp";
  const localPrinters = useMemo(() => infoQ.data?.localPrinters ?? [], [infoQ.data]);
  const connectors = useMemo(() => connectorsQ.data ?? [], [connectorsQ.data]);

  const [deviceId, setDeviceId] = useState("");
  const [printerName, setPrinterName] = useState("");

  // Seed the form from the saved default once it loads.
  useEffect(() => {
    if (targetQ.data) {
      setDeviceId(targetQ.data.connectorDeviceId);
      setPrinterName(targetQ.data.printerName);
    }
  }, [targetQ.data]);

  const printers = useMemo(
    () => connectors.find((c) => c.deviceId === deviceId)?.printerNames ?? [],
    [connectors, deviceId],
  );

  const connectorOptions: Option[] = [
    { value: "", label: t("settings.printing.autoSole") },
    ...connectors.map((c) => ({ value: c.deviceId, label: c.deviceName || c.deviceId })),
  ];
  const printerOptions: Option[] = ["", ...printers].map((p) => ({
    value: p,
    label: p || t("settings.printing.connectorDefault"),
  }));
  // usb: "" = host default printer, then each detected local printer.
  const localPrinterOptions: Option[] = [
    { value: "", label: t("settings.printing.hostDefault") },
    ...localPrinters.map((p) => ({ value: p, label: p })),
  ];

  // Connector mode saves device + printer; usb mode saves printer only (device "").
  const onSaveConnector = async () => {
    try {
      await save.mutateAsync({ connectorDeviceId: deviceId.trim(), printerName: printerName.trim() });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };
  const onSaveUsb = async () => {
    try {
      await save.mutateAsync({ connectorDeviceId: "", printerName: printerName.trim() });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  // Kitchen ticket target (restaurant mode). Independent of the receipt target
  // above: an empty device means "use the receipt printer", which is what makes
  // a one-printer shop work with no configuration.
  const { isRestaurant } = useBusinessMode();
  const kitchenQ = useKitchenPrintTargetQuery(isRestaurant);
  const saveKitchen = useSetKitchenPrintTargetMutation();
  const [kitchenDeviceId, setKitchenDeviceId] = useState("");
  const [kitchenPrinter, setKitchenPrinter] = useState("");
  useEffect(() => {
    if (kitchenQ.data) {
      setKitchenDeviceId(kitchenQ.data.connectorDeviceId);
      setKitchenPrinter(kitchenQ.data.printerName);
    }
  }, [kitchenQ.data]);

  const kitchenPrinters = useMemo(
    () => connectors.find((c) => c.deviceId === kitchenDeviceId)?.printerNames ?? [],
    [connectors, kitchenDeviceId],
  );
  // The empty option is the "same as the receipt printer" fallback, stated
  // rather than left as a blank the owner has to guess about.
  const kitchenConnectorOptions: Option[] = [
    { value: "", label: t("settings.printing.kitchenSameAsReceipt") },
    ...connectors.map((c) => ({ value: c.deviceId, label: c.deviceName })),
  ];
  const kitchenPrinterOptions: Option[] = kitchenPrinters.map((p) => ({ value: p, label: p }));

  const onSaveKitchen = async () => {
    try {
      await saveKitchen.mutateAsync({
        connectorDeviceId: kitchenDeviceId.trim(),
        printerName: kitchenPrinter.trim(),
      });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  // Receipt header/footer/width (applies to every print mode).
  const receiptQ = useReceiptSettingsQuery();
  const saveReceipt = useSetReceiptSettingsMutation();
  const [header, setHeader] = useState("");
  const [footer, setFooter] = useState("");
  const [width, setWidth] = useState(32);
  useEffect(() => {
    if (receiptQ.data) {
      setHeader(receiptQ.data.header);
      setFooter(receiptQ.data.footer);
      setWidth(receiptQ.data.width || 32);
    }
  }, [receiptQ.data]);

  const widthOptions: Option[] = [
    { value: "32", label: t("settings.printing.width58") },
    { value: "48", label: t("settings.printing.width80") },
  ];

  const onSaveReceipt = async () => {
    try {
      await saveReceipt.mutateAsync({ header, footer, width });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  if (infoQ.isLoading || targetQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  return (
    <Stack gap={5} maxW="lg">
      {/* ---- Connector mode ---- */}
      {mode === "connector" && (
        <>
          <Stack gap={1}>
            <HStack gap={2}>
              <Text fontSize="sm" fontWeight="medium">
                {t("settings.printing.connectors")}
              </Text>
              {connectorsQ.isFetching && <Spinner size="xs" />}
            </HStack>
            {connectors.length === 0 ? (
              <Text fontSize="sm" color="fg.muted">
                {t("settings.printing.noConnectors")}
              </Text>
            ) : (
              <Stack gap={1}>
                {connectors.map((c) => (
                  <HStack key={c.deviceId} gap={2}>
                    <Badge colorPalette="green">{t("settings.printing.online")}</Badge>
                    <Text fontSize="sm">{c.deviceName}</Text>
                    <Text fontSize="xs" color="fg.muted">
                      {c.printerNames.join(", ") || "—"}
                    </Text>
                  </HStack>
                ))}
              </Stack>
            )}
          </Stack>

          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.printing.defaultConnector")}
            </Text>
            <EnumSelect
              width="320px"
              value={deviceId}
              onChange={(v) => {
                setDeviceId(v);
                setPrinterName("");
              }}
              placeholder={t("settings.printing.selectConnector")}
              items={connectorOptions}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
            />
          </Stack>

          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.printing.defaultPrinter")}
            </Text>
            <EnumSelect
              width="320px"
              value={printerName}
              onChange={setPrinterName}
              placeholder={t("settings.printing.selectPrinter")}
              items={printerOptions}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
              disabled={printers.length === 0}
            />
          </Stack>

          <HStack>
            <Button colorPalette="blue" onClick={onSaveConnector} loading={save.isPending}>
              {t("common.save")}
            </Button>
          </HStack>
          <Text fontSize="xs" color="fg.muted">
            {t("settings.printing.help")}
          </Text>

          {/* ---- Kitchen ticket printer (restaurant mode) ----
              A separate target because the point of a kitchen ticket is that it
              comes out at the PASS, not at the till. Left unset it falls back to
              the receipt printer above, so a one-printer warung never has to
              touch this. Shown only in restaurant mode — in a shop there is no
              kitchen to send anything to. */}
          {isRestaurant && (
            <Stack gap={3} borderTopWidth="1px" pt={4}>
              <Text fontSize="sm" fontWeight="medium">
                {t("settings.printing.kitchenSection")}
              </Text>
              <Text fontSize="xs" color="fg.muted">
                {t("settings.printing.kitchenHelp")}
              </Text>
              <EnumSelect
                width="320px"
                value={kitchenDeviceId}
                onChange={(v) => {
                  setKitchenDeviceId(v);
                  setKitchenPrinter("");
                }}
                placeholder={t("settings.printing.kitchenSameAsReceipt")}
                items={kitchenConnectorOptions}
                itemToString={(o) => o.label}
                itemToValue={(o) => o.value}
              />
              <EnumSelect
                width="320px"
                value={kitchenPrinter}
                onChange={setKitchenPrinter}
                placeholder={t("settings.printing.selectPrinter")}
                items={kitchenPrinterOptions}
                itemToString={(o) => o.label}
                itemToValue={(o) => o.value}
                disabled={kitchenPrinters.length === 0}
              />
              <HStack>
                <Button
                  colorPalette="blue"
                  onClick={onSaveKitchen}
                  loading={saveKitchen.isPending}
                >
                  {t("common.save")}
                </Button>
              </HStack>
            </Stack>
          )}
        </>
      )}

      {/* ---- USB / local-printer mode ---- */}
      {mode === "usb" && (
        <>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.printing.usbPrinter")}
            </Text>
            <EnumSelect
              width="320px"
              value={printerName}
              onChange={setPrinterName}
              placeholder={t("settings.printing.selectLocalPrinter")}
              items={localPrinterOptions}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
            />
            {localPrinters.length === 0 && (
              <Text fontSize="xs" color="fg.muted">
                {t("settings.printing.noLocalPrinters")}
              </Text>
            )}
          </Stack>
          <HStack>
            <Button colorPalette="blue" onClick={onSaveUsb} loading={save.isPending}>
              {t("common.save")}
            </Button>
          </HStack>
          <Text fontSize="xs" color="fg.muted">
            {t("settings.printing.usbHelp")}
          </Text>
        </>
      )}

      {/* ---- TCP / network-printer mode ---- */}
      {mode === "tcp" && (
        <Text fontSize="sm" color="fg.muted">
          {t("settings.printing.tcpMode")}
        </Text>
      )}

      {/* ---- Receipt header/footer + paper width (all modes) ---- */}
      <Box borderTopWidth="1px" pt={5}>
        <Heading size="sm" mb={1}>
          {t("settings.printing.receiptTitle")}
        </Heading>
        <Text fontSize="xs" color="fg.muted" mb={3}>
          {t("settings.printing.receiptHelp")}
        </Text>
        <Stack gap={3} maxW="md">
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.printing.paperWidth")}
            </Text>
            <EnumSelect
              width="320px"
              value={String(width)}
              onChange={(v) => setWidth(Number(v))}
              items={widthOptions}
              itemToString={(o) => o.label}
              itemToValue={(o) => o.value}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.printing.receiptHeader")}
            </Text>
            <Textarea
              rows={3}
              value={header}
              onChange={(e) => setHeader(e.target.value)}
              placeholder={t("settings.printing.receiptHeaderPlaceholder")}
            />
          </Stack>
          <Stack gap={1}>
            <Text fontSize="sm" fontWeight="medium">
              {t("settings.printing.receiptFooter")}
            </Text>
            <Textarea
              rows={2}
              value={footer}
              onChange={(e) => setFooter(e.target.value)}
              placeholder={t("settings.printing.receiptFooterPlaceholder")}
            />
          </Stack>
          <HStack>
            <Button colorPalette="blue" onClick={onSaveReceipt} loading={saveReceipt.isPending}>
              {t("common.save")}
            </Button>
          </HStack>
        </Stack>
      </Box>
    </Stack>
  );
}
