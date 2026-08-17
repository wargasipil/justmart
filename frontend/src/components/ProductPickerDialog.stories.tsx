import { Button, Stack } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import ProductPickerDialog from "./ProductPickerDialog";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

const meta = {
  title: "Overlays/ProductPickerDialog",
  component: ProductPickerDialog,
  parameters: storyDocs("product-picker-dialog", { needsBackend: true }),
  tags: ["autodocs", "needs-backend"],
  argTypes: { onConfirm: { control: false }, onClose: { control: false } },
  // Each story owns the open/selection state; these satisfy the required props.
  args: { open: false, onClose: () => {}, selectedIds: [], onConfirm: () => {} },
} satisfies Meta<typeof ProductPickerDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Multi-select over a server-paginated search — for building a LIST of products
 * (restock lines). `<SearchableSelect loadOptions={searchProducts}>` stays the
 * single-value tool.
 *
 * The check set IS the caller's list: reconcile on confirm — new ids become
 * rows, dropped ids are removed, kept ids keep their edits.
 */
export const Default: Story = {
  render: () => {
    function Demo() {
      const [open, setOpen] = useState(false);
      const [ids, setIds] = useState<string[]>([]);
      const [names, setNames] = useState<string[]>([]);
      return (
        <Stack gap={2} maxW="420px">
          <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
            Tambah produk
          </Button>
          <Emitted>{names.length ? names.join(", ") : ""}</Emitted>
          <ProductPickerDialog
            open={open}
            onClose={() => setOpen(false)}
            selectedIds={ids}
            onConfirm={(picked, byId) => {
              setIds(picked);
              setNames(picked.map((id) => byId.get(id)?.name ?? id));
            }}
          />
        </Stack>
      );
    }
    return <Demo />;
  },
};

/**
 * `selectedIds` is read at OPEN time, so the dialog opens pre-checked and a
 * re-render mid-edit cannot clobber the draft. Cancel / Esc / backdrop do not
 * commit.
 */
export const PreChecked: Story = {
  render: () => {
    function Demo() {
      const [open, setOpen] = useState(false);
      const [ids, setIds] = useState<string[]>([]);
      return (
        <Stack gap={2} maxW="420px">
          <Button size="sm" variant="outline" onClick={() => setOpen(true)}>
            Ubah pilihan ({ids.length})
          </Button>
          <Emitted>{ids.join(", ")}</Emitted>
          <ProductPickerDialog
            open={open}
            onClose={() => setOpen(false)}
            selectedIds={ids}
            onConfirm={(picked) => setIds(picked)}
            title="Pilih produk untuk restock"
          />
        </Stack>
      );
    }
    return <Demo />;
  },
};
