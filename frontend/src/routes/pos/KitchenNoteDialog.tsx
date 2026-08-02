import { Button, Dialog, Field, HStack, Input, Portal, Stack, Text } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type { SaleItem } from "../../gen/pos_iface/v1/sale_pb";

// Mirrors the backend's maxKitchenNoteLen — the note prints on a 32-column
// thermal ticket, so an unbounded string is unreadable at the pass.
const MAX_NOTE_LEN = 120;

// The cook-facing note on one cart line ("no ice", "extra pedas").
//
// Deliberately NOT a priced modifier: a note never moves an amount. If an extra
// should cost money it belongs in the cart as its own line (a SERVICE product
// does exactly that), where the discount and grosir rules can see it.
//
// Dialog mount HARD RULE: Dialog.Root stays mounted and is driven by `open` —
// the CONTENT is guarded on the item instead. Unmounting an open Ark dialog
// leaves the body pointer-events lock in place and freezes the page.
export default function KitchenNoteDialog({
  item,
  open,
  isPending,
  onSave,
  onCancel,
}: {
  item: SaleItem | null;
  open: boolean;
  isPending?: boolean;
  onSave: (note: string) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [note, setNote] = useState("");

  useEffect(() => {
    if (open) setNote(item?.kitchenNote ?? "");
  }, [open, item]);

  const alreadyFired = (item?.firedAt ?? 0n) > 0n;

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
              <Dialog.Title>{t("pos.noteTitle")}</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body>
              {item && (
                <Stack gap={3}>
                  <Text fontWeight="medium">{item.productName}</Text>
                  <Field.Root>
                    <Field.Label>{t("pos.noteLabel")}</Field.Label>
                    <Input
                      autoFocus
                      maxLength={MAX_NOTE_LEN}
                      placeholder={t("pos.notePlaceholder")}
                      value={note}
                      onChange={(e) => setNote(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") onSave(note);
                      }}
                    />
                    <Field.HelperText>{t("pos.noteHelp")}</Field.HelperText>
                  </Field.Root>
                  {alreadyFired && (
                    // Editing an already-fired line does not reprint — the
                    // kitchen has the old ticket in hand, so the change has to
                    // be spoken. Say so rather than letting the waiter assume.
                    <Text fontSize="xs" color="orange.fg">
                      {t("pos.noteAfterFire")}
                    </Text>
                  )}
                </Stack>
              )}
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="100%">
                <Button variant="ghost" onClick={onCancel}>
                  {t("common.cancel")}
                </Button>
                <Button colorPalette="blue" loading={isPending} onClick={() => onSave(note)}>
                  {t("common.save")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
