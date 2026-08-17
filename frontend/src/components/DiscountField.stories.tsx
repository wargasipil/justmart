import { Stack } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import DiscountField, { type DiscountType } from "./DiscountField";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

function Controlled({ type, value, ...rest }: React.ComponentProps<typeof DiscountField>) {
  const [kind, setKind] = useState<DiscountType>(type);
  const [amount, setAmount] = useState(value);
  useEffect(() => setKind(type), [type]);
  useEffect(() => setAmount(value), [value]);
  return (
    <Stack gap={2} align="flex-start">
      <DiscountField
        {...rest}
        type={kind}
        value={amount}
        onChange={(nextType, nextValue) => {
          setKind(nextType);
          setAmount(nextValue);
        }}
      />
      <Emitted>{`${kind} / ${amount}`}</Emitted>
    </Stack>
  );
}

const meta = {
  title: "Forms & inputs/DiscountField",
  component: DiscountField,
  parameters: storyDocs("discount-field"),
  tags: ["autodocs"],
  argTypes: {
    type: { control: "inline-radio", options: ["FIXED", "PERCENT"] },
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onChange: { control: false },
  },
  args: { type: "FIXED", value: 5000, onChange: () => {} },
  render: (args) => <Controlled {...args} />,
} satisfies Meta<typeof DiscountField>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Rupiah off the line — `value` is minor units, same as everywhere else. */
export const Fixed: Story = {};

/**
 * Percent — `value` is a PLAIN percent here (10 means 10%). The caller
 * converts to basis points (×100) before it goes to the backend.
 */
export const Percent: Story = {
  args: { type: "PERCENT", value: 10 },
};

/** Switching the type resets the value to 0 — 5000 rupiah isn't 5000 percent. */
export const SwitchingResets: Story = {
  args: { type: "FIXED", value: 25000 },
};

export const Disabled: Story = {
  args: { disabled: true },
};
