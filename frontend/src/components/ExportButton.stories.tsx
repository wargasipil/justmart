import type { Meta, StoryObj } from "@storybook/react";

import ExportButton from "./ExportButton";
import { storyDocs } from "../routes/dev/storyDocs";
import { toast } from "../lib/toaster";

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const meta = {
  title: "Toolbar & filters/ExportButton",
  component: ExportButton,
  parameters: storyDocs("export-button"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onExport: { control: false },
  },
  args: {
    onExport: async () => {
      await wait(900);
      toast.success("Diekspor", "137 baris");
    },
  },
} satisfies Meta<typeof ExportButton>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Click it: the button holds a loading state for the whole fetch-all. `onExport`
 * is responsible for fetching every row matching the ACTIVE filters — not just
 * the current page — then serializing and triggering the download.
 */
export const Default: Story = {};

export const Slow: Story = {
  args: {
    onExport: async () => {
      await wait(3000);
      toast.success("Diekspor", "8.412 baris");
    },
  },
};

/**
 * A rejected export routes to the global toast automatically — the button just
 * returns to rest.
 */
export const Failing: Story = {
  args: {
    onExport: async () => {
      await wait(700);
      throw new Error("export failed");
    },
  },
};

/** Disabled while there is nothing to export. */
export const Disabled: Story = {
  args: { disabled: true },
};
