import { Stack } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import StockUnitPopover from "./StockUnitPopover";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";
import type { StockUnitGroup, StockUnitsByBase } from "../lib/stockUnit";

const GROUPS: StockUnitGroup[] = [
  { baseName: "tablet", derivatives: ["strip", "box"] },
  { baseName: "ml", derivatives: ["botol"] },
];

const meta = {
  title: "Toolbar & filters/StockUnitPopover",
  component: StockUnitPopover,
  parameters: storyDocs("stock-unit-popover"),
  tags: ["autodocs"],
  argTypes: {
    groups: { control: false },
    byBase: { control: false },
    onChangeBase: { control: false },
  },
  // `onChangeBase` is re-bound to the story's own state in `render`.
  args: { groups: GROUPS, byBase: { tablet: "strip" }, onChangeBase: () => {} },
  render: (args) => {
    function Demo() {
      const [byBase, setByBase] = useState<StockUnitsByBase>(args.byBase);
      return (
        <Stack gap={2} align="flex-start">
          <StockUnitPopover
            {...args}
            byBase={byBase}
            onChangeBase={(baseName, deriv) => setByBase((prev) => ({ ...prev, [baseName]: deriv }))}
          />
          <Emitted>{JSON.stringify(byBase)}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
} satisfies Meta<typeof StockUnitPopover>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Stock is always STORED in the base unit; this only changes how the Products
 * list renders it. `""` means the raw base count.
 */
export const Default: Story = {};

/** Nothing chosen — every base renders as its raw count. */
export const AllBaseUnits: Story = {
  args: { byBase: {} },
};

/** One base unit with a single derivative. */
export const SingleGroup: Story = {
  args: { groups: [GROUPS[1]], byBase: { ml: "botol" } },
};
