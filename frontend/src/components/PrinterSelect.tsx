import { useState } from "react";
import { Button, Dialog, Flex, IconButton, Portal, Stack, Text } from "@chakra-ui/react";
import { Check, ChevronDown, Printer, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Connector } from "../gen/connector_iface/v1/connector_pb";

type Props = {
  connectors: readonly Connector[];
  value: string; // "<deviceId>|<printerName>", or "" for Auto
  onChange: (value: string) => void;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
};

type Opt = { value: string; label: string };

// POS receipt-printer picker rendered as a button + centered popup (mirrors
// WarehouseSelect). The button shows the chosen printer; the popup lists "Auto"
// plus every connected connector's printers. The value encodes the target as
// "<deviceId>|<printerName>" ("" = Auto → server resolves the saved default /
// sole connector). The caller persists the choice (localStorage).
export default function PrinterSelect({ connectors, value, onChange, size = "sm", width = "100%" }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const options: Opt[] = [
    { value: "", label: t("pos.printerAuto") },
    ...connectors.flatMap((c) =>
      c.printerNames.map((p) => ({
        value: `${c.deviceId}|${p}`,
        label: connectors.length > 1 ? `${c.deviceName} · ${p}` : p,
      })),
    ),
  ];
  const selected = options.find((o) => o.value === value) ?? options[0];

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size={size}
        width={width}
        justifyContent="space-between"
        fontWeight="normal"
        onClick={() => setOpen(true)}
      >
        <Flex align="center" gap={2} minW={0}>
          <Printer size={14} />
          <Text truncate>{selected.label}</Text>
        </Flex>
        <ChevronDown size={16} />
      </Button>

      <Dialog.Root open={open} onOpenChange={(d) => !d.open && setOpen(false)}>
        <Portal>
          <Dialog.Backdrop />
          <Dialog.Positioner>
            <Dialog.Content>
              <Dialog.Header>
                <Dialog.Title>{t("pos.selectPrinter")}</Dialog.Title>
                <Dialog.CloseTrigger asChild>
                  <IconButton aria-label="close" variant="ghost" size="sm">
                    <X size={16} />
                  </IconButton>
                </Dialog.CloseTrigger>
              </Dialog.Header>
              <Dialog.Body>
                <Stack gap={1} maxH="320px" overflowY="auto">
                  {options.map((o) => (
                    <Flex
                      key={o.value || "auto"}
                      px={3}
                      py={2}
                      borderRadius="md"
                      _hover={{ bg: "bg.muted" }}
                      cursor="pointer"
                      justify="space-between"
                      align="center"
                      bg={o.value === value ? "bg.muted" : undefined}
                      onClick={() => {
                        onChange(o.value);
                        setOpen(false);
                      }}
                    >
                      <Text fontSize="sm">{o.label}</Text>
                      {o.value === value && <Check size={14} />}
                    </Flex>
                  ))}
                </Stack>
              </Dialog.Body>
            </Dialog.Content>
          </Dialog.Positioner>
        </Portal>
      </Dialog.Root>
    </>
  );
}
