import { Button, HStack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import EntityDialog from "./EntityDialog";
import { storyDocs } from "../routes/dev/storyDocs";

function Demo({
  size,
  startOpen = false,
}: {
  size?: React.ComponentProps<typeof EntityDialog>["size"];
  startOpen?: boolean;
}) {
  const [open, setOpen] = useState(startOpen);
  return (
    <>
      <Button size="sm" colorPalette="blue" onClick={() => setOpen(true)}>
        Buka dialog
      </Button>
      <EntityDialog
        open={open}
        onClose={() => setOpen(false)}
        title="Ubah produk"
        size={size}
        footer={
          <HStack justify="flex-end" gap={2}>
            <Button variant="ghost" onClick={() => setOpen(false)}>
              Batal
            </Button>
            <Button colorPalette="blue" onClick={() => setOpen(false)}>
              Simpan
            </Button>
          </HStack>
        }
      >
        <Text fontSize="sm" color="fg.muted">
          Same prop API as EntityDrawer — a caller can swap one for the other.
        </Text>
      </EntityDialog>
    </>
  );
}

const meta = {
  title: "Overlays/EntityDialog",
  component: EntityDialog,
  parameters: storyDocs("entity-dialog"),
  tags: ["autodocs"],
  argTypes: { size: { control: "inline-radio", options: ["sm", "md", "lg", "xl"] } },
  // The stories drive `open` from <Demo/>; these satisfy the required props.
  args: { open: false, onClose: () => {}, title: "Ubah produk", children: null },
} satisfies Meta<typeof EntityDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The centered-modal counterpart of EntityDrawer, with an identical prop API.
 * The product form is the one entity that uses it; other entities slide over.
 */
export const Closed: Story = {
  render: () => <Demo />,
};

export const Open: Story = {
  render: () => <Demo startOpen />,
};

export const Large: Story = {
  render: () => <Demo size="lg" startOpen />,
};
