import { Stack } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import ColumnsPopover from "./ColumnsPopover";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

const GROUPS = [
  {
    id: "order",
    label: "Order",
    fields: [
      { id: "order.terjual", label: "Terjual" },
      { id: "order.hpp", label: "HPP" },
      { id: "order.profit", label: "Profit" },
    ],
  },
  {
    id: "stock",
    label: "Stok",
    fields: [
      { id: "stock.ready", label: "Siap" },
      { id: "stock.ongoing", label: "Dalam perjalanan" },
    ],
  },
];

const meta = {
  title: "Toolbar & filters/ColumnsPopover",
  component: ColumnsPopover,
  parameters: storyDocs("columns-popover"),
  tags: ["autodocs"],
  argTypes: {
    groups: { control: false },
    value: { control: false },
    defaults: { control: false },
    onChange: { control: false },
  },
  // `value`/`onChange` are re-bound to the story's own state in `render`.
  args: {
    groups: GROUPS,
    value: new Set(["order.terjual", "order.profit", "stock.ready"]),
    onChange: () => {},
  },
  render: (args) => {
    function Demo() {
      const [visible, setVisible] = useState<Set<string>>(
        () => new Set(args.value ?? ["order.terjual", "order.profit", "stock.ready"]),
      );
      return (
        <Stack gap={2} align="flex-start">
          <ColumnsPopover {...args} value={visible} onChange={setVisible} />
          <Emitted>{[...visible].sort().join(", ")}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
} satisfies Meta<typeof ColumnsPopover>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * One button with a `visible/total` counter badge — this is what scales to any
 * number of metric groups without adding another control to the toolbar.
 */
export const Default: Story = {};

export const AllVisible: Story = {
  args: { value: new Set(GROUPS.flatMap((g) => g.fields.map((f) => f.id))) },
};

/**
 * A whole group can be disabled with an inline reason — the User analytics page
 * disables Stock, because stock has no meaning per user.
 */
export const GroupDisabled: Story = {
  args: {
    groups: [
      GROUPS[0],
      { ...GROUPS[1], disabled: true, disabledReason: "Stok tidak berlaku per pengguna" },
    ],
  },
};

/** `defaults` is what Reset restores — not necessarily everything. */
export const CustomResetDefaults: Story = {
  args: {
    value: new Set(["order.terjual"]),
    defaults: new Set(["order.terjual", "order.profit"]),
  },
};
