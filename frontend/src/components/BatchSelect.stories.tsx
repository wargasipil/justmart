import { Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import BatchSelect from "./BatchSelect";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

const meta = {
  title: "Selects & pickers/BatchSelect",
  component: BatchSelect,
  parameters: storyDocs("batch-select", { needsBackend: true }),
  tags: ["autodocs", "needs-backend"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onChange: { control: false },
    onSelectItem: { control: false },
    excludeIds: { control: false },
  },
  args: { value: "" },
  render: (args) => {
    function Demo() {
      const [value, setValue] = useState(args.value ?? "");
      const [picked, setPicked] = useState("");
      return (
        <Stack gap={2} maxW="380px">
          <BatchSelect
            {...args}
            value={value}
            onChange={setValue}
            onSelectItem={(b) => setPicked(`${b.productName} · ${b.batchNumber}`)}
          />
          <Emitted>{value}</Emitted>
          {picked ? (
            <Text fontSize="xs" color="fg.muted">
              onSelectItem → {picked}
            </Text>
          ) : null}
        </Stack>
      );
    }
    return <Demo />;
  },
} satisfies Meta<typeof BatchSelect>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Rows are deliberately two-line — a product thumbnail, then the product name
 * over `batch no · expiry · available qty`. That is the whole reason to reach
 * for this over a bare `<SearchableSelect loadOptions={searchBatches}>`: staff
 * never has to memorize a batch number.
 */
export const Default: Story = {};

/**
 * `clearable` puts an inline × on the trigger. Right for a FILTER, wrong for a
 * required line field — which is why it defaults to false.
 */
export const Clearable: Story = {
  args: { clearable: true },
};

/**
 * **The trap.** `onlyInStock` defaults TRUE here while the underlying
 * `searchBatches` defaults it FALSE — so a call site converted from the raw
 * select silently loses depleted lots. A ledger filter and a positive
 * ADJUSTMENT both need them, so both must pass `onlyInStock={false}`.
 */
export const IncludingDepletedLots: Story = {
  args: { onlyInStock: false, clearable: true },
};

/** Scoped to one warehouse's stock — e.g. a transfer source. */
export const ScopedToWarehouse: Story = {
  args: { warehouseId: "", placeholder: "Pilih batch di gudang aktif" },
};

export const Disabled: Story = {
  args: { disabled: true, placeholder: "Pilih gudang asal dulu" },
};
