import { Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import SupplierSelect from "./SupplierSelect";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

// NEEDS A BACKEND. The baked-in `searchSuppliers` IS this component — a fixture
// list would demo the opposite of the thing — so these stories talk to the dev
// server through the app's own vite `/api` proxy. Start it with `make run`.
// Nothing fires until the popover is opened, so browsing costs nothing.

const meta = {
  title: "Selects & pickers/SupplierSelect",
  component: SupplierSelect,
  parameters: storyDocs("supplier-select", { needsBackend: true }),
  tags: ["autodocs", "needs-backend"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onChange: { control: false },
    onSelectItem: { control: false },
  },
  args: { value: "" },
  render: (args) => {
    function Demo() {
      const [value, setValue] = useState(args.value);
      const [label, setLabel] = useState("");
      return (
        <Stack gap={2} maxW="320px">
          <SupplierSelect
            {...args}
            value={value}
            onChange={setValue}
            onSelectItem={(s) => setLabel(s?.name ?? "")}
          />
          <Emitted>{value}</Emitted>
          {label ? (
            <Text fontSize="xs" color="fg.muted">
              onSelectItem → {label}
            </Text>
          ) : null}
        </Stack>
      );
    }
    return <Demo />;
  },
} satisfies Meta<typeof SupplierSelect>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Rows lead with the NAME over a muted code, behind a Building2 glyph. The
 * exported `supplierLabel()` keeps the other format — `CODE · Name` — for table
 * cells, where a column is scanned straight down and the fixed-width code is
 * what makes that scan work.
 */
export const Default: Story = {};

/**
 * Read-only, as an edit drawer renders it once the agreement is locked. It
 * resolves its own trigger label from the id, so no raw UUID flashes.
 */
export const ReadOnly: Story = {
  args: { disabled: true },
};

export const Small: Story = {
  args: { size: "sm" },
};
