import { Button, Dialog, Field, HStack, Input, Portal, Stack, Text } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type { DiningTable } from "../../gen/table_iface/v1/table_pb";

// Asks for the cover count before seating a party.
//
// Dialog mount HARD RULE: the Dialog.Root stays mounted and is driven purely by
// `open` — never `if (!table) return null`. Unmounting an OPEN Ark dialog leaves
// the body pointer-events lock in place and freezes the whole page, so the
// CONTENT is guarded on the data instead.
export default function OpenTableDialog({
  table,
  open,
  isPending,
  onConfirm,
  onCancel,
}: {
  table: DiningTable | null;
  open: boolean;
  isPending?: boolean;
  onConfirm: (guestCount: number) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [guests, setGuests] = useState("");

  // Re-seed on each open: default to the table's capacity, which is the right
  // guess often enough to save a keystroke, and clear otherwise.
  useEffect(() => {
    if (open) setGuests(table?.seats ? String(table.seats) : "");
  }, [open, table]);

  const submit = () => onConfirm(Math.max(0, Number(guests) || 0));

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onCancel();
      }}
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>
                {t("tables.openTitle", { code: table?.code ?? "" })}
              </Dialog.Title>
            </Dialog.Header>
            <Dialog.Body>
              {table && (
                <Stack gap={3}>
                  <Text color="fg.muted" fontSize="sm">
                    {t("tables.openHelp")}
                  </Text>
                  <Field.Root>
                    <Field.Label>{t("tables.guestCount")}</Field.Label>
                    <Input
                      type="number"
                      min={0}
                      autoFocus
                      value={guests}
                      onChange={(e) => setGuests(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") submit();
                      }}
                    />
                  </Field.Root>
                </Stack>
              )}
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="100%">
                <Button variant="ghost" onClick={onCancel}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" onClick={submit} loading={isPending}>
                  {t("tables.openAction")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
