import { Box, Drawer, Heading, IconButton, Portal, Stack } from "@chakra-ui/react";
import { X } from "lucide-react";
import type { ReactNode } from "react";

type Props = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
};

export default function EntityDrawer({
  open,
  onClose,
  title,
  children,
  footer,
  size = "md",
}: Props) {
  return (
    <Drawer.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      placement="end"
      size={size}
    >
      <Portal>
        <Drawer.Backdrop />
        <Drawer.Positioner>
          <Drawer.Content>
            {/* Title + CloseTrigger as direct children: the header recipe is
                the flex row, so it is what pins the [x] to the corner. An
                HStack in between shrinks to the title's width and takes the
                button with it. `asChild` keeps our own Heading type. */}
            <Drawer.Header borderBottomWidth="1px">
              <Drawer.Title asChild>
                <Heading size="lg">{title}</Heading>
              </Drawer.Title>
              <Drawer.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={18} />
                </IconButton>
              </Drawer.CloseTrigger>
            </Drawer.Header>
            <Drawer.Body>
              <Stack gap={4}>{children}</Stack>
            </Drawer.Body>
            {footer && (
              <Drawer.Footer borderTopWidth="1px">
                <Box w="full">{footer}</Box>
              </Drawer.Footer>
            )}
          </Drawer.Content>
        </Drawer.Positioner>
      </Portal>
    </Drawer.Root>
  );
}
