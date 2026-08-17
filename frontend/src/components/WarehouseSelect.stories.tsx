import { HStack, Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import WarehouseSelect from "./WarehouseSelect";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";
import { Warehouse } from "../gen/warehouse_iface/v1/warehouse_pb";

const WAREHOUSES = [
  new Warehouse({ id: "w1", code: "MAIN", name: "Gudang Utama" }),
  new Warehouse({ id: "w2", code: "CAB-01", name: "Cabang Kemang" }),
  new Warehouse({ id: "w3", code: "CAB-02", name: "Cabang Bintaro" }),
  new Warehouse({ id: "w4", code: "CAB-03", name: "Cabang Serpong" }),
];

const meta = {
  title: "Selects & pickers/WarehouseSelect",
  component: WarehouseSelect,
  parameters: storyDocs("warehouse-select"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    warehouses: { control: false },
    loadOptions: { control: false },
    onChange: { control: false },
  },
  args: { warehouses: WAREHOUSES, value: "w1", onChange: () => {} },
  render: (args) => {
    function Demo() {
      const [value, setValue] = useState(args.value);
      return (
        <Stack gap={2} maxW="320px">
          <WarehouseSelect {...args} value={value} onChange={setValue} />
          <Emitted>{value}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
} satisfies Meta<typeof WarehouseSelect>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * A button that opens a searchable modal — not a dropdown. It is the one
 * warehouse affordance: TopBar, POS header and Transfers all mount this.
 */
export const Default: Story = {};

/**
 * Static mode filters in memory, which is right for a small bounded set. The
 * TopBar instead passes `loadOptions` for a debounced server search.
 */
export const AsyncLoadOptions: Story = {
  args: {
    warehouses: undefined,
    loadOptions: (query: string) => {
      const needle = query.trim().toLowerCase();
      return Promise.resolve(
        needle
          ? WAREHOUSES.filter(
              (w) =>
                w.name.toLowerCase().includes(needle) || w.code.toLowerCase().includes(needle),
            )
          : WAREHOUSES,
      );
    },
  },
};

/**
 * `excludeId` — the Transfers "To" picker hides whatever "From" already holds,
 * because a transfer to the same warehouse is rejected server-side anyway.
 */
export const TransferPair: Story = {
  render: () => {
    function Demo() {
      const [from, setFrom] = useState("w1");
      const [to, setTo] = useState("w2");
      return (
        <Stack gap={3} maxW="640px">
          <HStack gap={3} align="flex-end">
            <Stack gap={1} flex="1">
              <Text fontSize="xs" color="fg.muted">
                Dari
              </Text>
              <WarehouseSelect warehouses={WAREHOUSES} value={from} onChange={setFrom} />
            </Stack>
            <Stack gap={1} flex="1">
              <Text fontSize="xs" color="fg.muted">
                Ke
              </Text>
              <WarehouseSelect
                warehouses={WAREHOUSES}
                value={to}
                onChange={setTo}
                excludeId={from}
              />
            </Stack>
          </HStack>
          <Emitted>{`${from} → ${to}`}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

export const Disabled: Story = {
  args: { disabled: true },
};
