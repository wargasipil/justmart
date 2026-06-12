import { Badge, Box, Button, HStack, Spinner, Stack, Text } from "@chakra-ui/react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import EnumSelect from "../../components/EnumSelect";
import { toast } from "../../lib/toaster";
import {
  useConnectorsQuery,
  usePrintTargetQuery,
  useSetPrintTargetMutation,
} from "../../queries/connectors";

type Option = { value: string; label: string };

// SettingsPrinting (OWNER) — pick the default print connector + printer used by
// the POS receipt Print button (connector mode). A single-connector shop can
// leave the connector on "auto" and the server targets the sole connected one.
export default function SettingsPrinting() {
  const { t } = useTranslation();
  const connectorsQ = useConnectorsQuery();
  const targetQ = usePrintTargetQuery();
  const save = useSetPrintTargetMutation();

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

  const onSave = async () => {
    try {
      await save.mutateAsync({
        connectorDeviceId: deviceId.trim(),
        printerName: printerName.trim(),
      });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  if (targetQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  return (
    <Stack gap={5} maxW="lg">
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
        <Button colorPalette="blue" onClick={onSave} loading={save.isPending}>
          {t("common.save")}
        </Button>
      </HStack>

      <Text fontSize="xs" color="fg.muted">
        {t("settings.printing.help")}
      </Text>
    </Stack>
  );
}
