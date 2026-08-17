import { Badge, Button, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import PageHeader from "./PageHeader";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "Layout & page chrome/PageHeader",
  component: PageHeader,
  parameters: storyDocs("page-header"),
  tags: ["autodocs"],
  argTypes: {
    titleBadge: { control: false },
    actions: { control: false },
  },
  args: { title: "Batch" },
} satisfies Meta<typeof PageHeader>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Title only — the floor every page starts from. */
export const Default: Story = {};

export const WithDescription: Story = {
  args: { description: "Every lot with stock in the active warehouse." },
};

/**
 * A status badge belongs in `titleBadge`, beside the title — not loose in the
 * page body where it reads as data rather than as the state of this record.
 */
export const WithBadge: Story = {
  args: {
    title: "SUP-0001 · Kimia Farma",
    titleBadge: <Badge colorPalette="red">Arsip</Badge>,
    description: "Archived suppliers stay on historical purchase orders.",
  },
};

export const WithActions: Story = {
  args: {
    description: "Every lot with stock in the active warehouse.",
    actions: (
      <Button size="sm" colorPalette="blue">
        Add
      </Button>
    ),
  },
};

/** Everything at once, followed by page body copy so the divider is visible. */
export const Full: Story = {
  args: {
    title: "Paracetamol 500mg",
    titleBadge: <Badge colorPalette="green">Aktif</Badge>,
    description: "SKU-00123 · tablet",
    actions: (
      <>
        <Button size="sm" variant="outline">
          Print barcode
        </Button>
        <Button size="sm" colorPalette="blue">
          Edit
        </Button>
      </>
    ),
  },
  render: (args) => (
    <>
      <PageHeader {...args} />
      <Text fontSize="sm" color="fg.muted">
        Page body starts here.
      </Text>
    </>
  ),
};
