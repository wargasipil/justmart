import { Button } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import ConfirmDialog from "./ConfirmDialog";
import { storyDocs } from "../routes/dev/storyDocs";
import { toast } from "../lib/toaster";

// Driven by a `pending` state, which is the pattern every call site uses: the
// action button sets the record being acted on, `open={pending != null}`, and
// onConfirm runs the mutation.
function Demo(props: Partial<React.ComponentProps<typeof ConfirmDialog>> & { trigger?: string }) {
  const [pending, setPending] = useState<string | null>(null);
  const { trigger = "Arsipkan pemasok", ...rest } = props;
  return (
    <>
      <Button size="sm" colorPalette="red" variant="outline" onClick={() => setPending("SUP-0001")}>
        {trigger}
      </Button>
      <ConfirmDialog
        open={pending != null}
        title="Arsipkan pemasok?"
        body={`${pending ?? ""} akan berhenti muncul di pemilih. Riwayat tetap menyimpannya.`}
        confirmLabel="Arsipkan"
        onConfirm={() => {
          toast.success("Diarsipkan", pending ?? "");
          setPending(null);
        }}
        onCancel={() => setPending(null)}
        {...rest}
      />
    </>
  );
}

const meta = {
  title: "Overlays/ConfirmDialog",
  component: ConfirmDialog,
  parameters: storyDocs("confirm-dialog"),
  tags: ["autodocs"],
  // Every story drives the dialog from a `pending` state inside <Demo/>, so
  // these args are only here to satisfy the required-props contract.
  args: {
    open: false,
    title: "Arsipkan pemasok?",
    confirmLabel: "Arsipkan",
    onConfirm: () => {},
    onCancel: () => {},
  },
} satisfies Meta<typeof ConfirmDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The ONLY confirmation affordance in the app — `window.confirm` is banned, it
 * renders OS chrome that clashes with everything around it.
 *
 * Note for tests: this is `role="alertdialog"`, **not** `role="dialog"`, so
 * `getByRole("dialog")` silently never matches it.
 */
export const Destructive: Story = {
  render: () => <Demo />,
};

/** Non-destructive confirmations override the default red palette. */
export const NonDestructive: Story = {
  render: () => (
    <Demo
      trigger="Jadikan gudang utama"
      title="Jadikan gudang utama?"
      body="Gudang ini akan menjadi default untuk pengguna baru."
      confirmLabel="Jadikan utama"
      confirmColorPalette="blue"
    />
  ),
};

/** `loading` keeps the dialog open with a spinner while the mutation runs. */
export const Pending: Story = {
  render: () => <Demo loading />,
};

/** `body` takes a ReactNode, not only a string — a list of consequences fits. */
export const RichBody: Story = {
  render: () => (
    <Demo
      trigger="Batalkan penerimaan"
      title="Batalkan penerimaan barang?"
      confirmLabel="Batalkan"
      body={
        <>
          Lot RCV-2026-0014 akan dihapus dan stoknya ditarik kembali.
          <br />
          Dokumen penerimaan tetap tersimpan, ditandai VOIDED.
        </>
      }
    />
  ),
};
