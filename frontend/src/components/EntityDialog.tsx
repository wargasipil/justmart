import { Box, Dialog, Heading, IconButton, Portal, Stack } from "@chakra-ui/react";
import { X } from "lucide-react";
import type { ReactNode } from "react";

// Centered modal counterpart of <EntityDrawer> — same prop API, so a caller can
// swap one for the other. Use for create/edit flows that should feel like a
// quick popup rather than a slide-over (e.g. the Obat product form).
type Props = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
};

export default function EntityDialog({
  open,
  onClose,
  title,
  children,
  footer,
  size = "md",
}: Props) {
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      size={size}
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            {/* Same as EntityDrawer: the header recipe is the flex row, so
                Title and CloseTrigger are its direct children. */}
            <Dialog.Header borderBottomWidth="1px">
              <Dialog.Title asChild>
                <Heading size="lg">{title}</Heading>
              </Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={18} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={4}>{children}</Stack>
            </Dialog.Body>
            {footer && (
              <Dialog.Footer borderTopWidth="1px">
                <Box w="full">{footer}</Box>
              </Dialog.Footer>
            )}
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
