import { Button, HStack, Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import EntityDrawer from "./EntityDrawer";
import { storyDocs } from "../routes/dev/storyDocs";

// Every story keeps the drawer MOUNTED and drives `open` — never
// `{open && <EntityDrawer/>}`. Unmounting an open Ark dialog leaves its body
// lock in place and freezes the whole page.
function Demo({
  size,
  startOpen = false,
}: {
  size?: React.ComponentProps<typeof EntityDrawer>["size"];
  startOpen?: boolean;
}) {
  const [open, setOpen] = useState(startOpen);
  return (
    <>
      <Button size="sm" colorPalette="blue" onClick={() => setOpen(true)}>
        Buka drawer
      </Button>
      <EntityDrawer
        open={open}
        onClose={() => setOpen(false)}
        title="Pemasok baru"
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
        <Stack gap={3}>
          <Text fontSize="sm" color="fg.muted">
            Form fields go here — one FormField per row.
          </Text>
          <Text fontSize="sm" color="fg.muted">
            The footer is sticky, so a long form keeps Save reachable.
          </Text>
        </Stack>
      </EntityDrawer>
    </>
  );
}

const meta = {
  title: "Overlays/EntityDrawer",
  component: EntityDrawer,
  parameters: storyDocs("entity-drawer"),
  tags: ["autodocs"],
  argTypes: { size: { control: "inline-radio", options: ["sm", "md", "lg", "xl"] } },
  // The stories drive `open` from <Demo/>; these satisfy the required props.
  args: { open: false, onClose: () => {}, title: "Pemasok baru", children: null },
} satisfies Meta<typeof EntityDrawer>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The default create/edit surface: a slide-over from the right. */
export const Closed: Story = {
  render: () => <Demo />,
};

export const Open: Story = {
  render: () => <Demo startOpen />,
};

export const Large: Story = {
  render: () => <Demo size="lg" startOpen />,
};

export const Small: Story = {
  render: () => <Demo size="sm" startOpen />,
};

/**
 * A form inside a drawer must reset on open — otherwise a half-typed, then
 * cancelled, entry is still sitting in the fields next time. Use
 * `useResetOnOpen(form, open, values)`: the `useForm` call lives in the
 * component that RENDERS the drawer, so it stays mounted while the drawer is
 * closed, and RHF keeps values in a ref regardless. Chakra's
 * `lazyMount`/`unmountOnExit` does not fix it, and neither does RHF's `values`
 * prop when you re-open on the same record.
 */
export const FormResetReminder: Story = {
  render: () => <Demo />,
};
