import { Button, Dialog, Heading, HStack, IconButton, Portal, Text } from "@chakra-ui/react";
import { X } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

// Shared confirmation dialog (Chakra). The app NEVER uses native
// window.confirm/alert/prompt — see CLAUDE.md "No native browser dialogs".
// Drive it with a `pending` state: an action button sets pending, this opens,
// onConfirm runs the mutation, onCancel clears pending.
export type ConfirmDialogProps = {
  open: boolean;
  title: string;
  body?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  /** Chakra colorPalette for the confirm button. Default "red" (destructive). */
  confirmColorPalette?: string;
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

export default function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel,
  cancelLabel,
  confirmColorPalette = "red",
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onCancel();
      }}
      size="sm"
      role="alertdialog"
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header borderBottomWidth="1px">
              <HStack justify="space-between" w="full">
                <Heading size="md">{title}</Heading>
                <IconButton aria-label={t("common.close")} variant="ghost" size="sm" onClick={onCancel}>
                  <X size={18} />
                </IconButton>
              </HStack>
            </Dialog.Header>
            {body != null && (
              <Dialog.Body>
                {typeof body === "string" ? <Text fontSize="sm">{body}</Text> : body}
              </Dialog.Body>
            )}
            <Dialog.Footer borderTopWidth="1px">
              <HStack justify="flex-end" w="full" gap={2}>
                <Button variant="ghost" onClick={onCancel}>
                  {cancelLabel ?? t("common.cancel")}
                </Button>
                <Button colorPalette={confirmColorPalette} loading={loading} onClick={onConfirm}>
                  {confirmLabel}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
