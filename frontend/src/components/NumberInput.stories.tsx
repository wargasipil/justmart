import { Stack } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import NumberInput from "./NumberInput";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

function Controlled({ value, ...rest }: React.ComponentProps<typeof NumberInput>) {
  const [raw, setRaw] = useState(String(value ?? ""));
  useEffect(() => setRaw(String(value ?? "")), [value]);
  return (
    <Stack gap={2} maxW="240px">
      <NumberInput {...rest} value={raw} onChange={setRaw} aria-label="quantity" />
      <Emitted>{raw}</Emitted>
    </Stack>
  );
}

const meta = {
  title: "Forms & inputs/NumberInput",
  component: NumberInput,
  parameters: storyDocs("number-input"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onChange: { control: false },
  },
  args: { value: "12", onChange: () => {} },
  render: (args) => <Controlled {...args} />,
} satisfies Meta<typeof NumberInput>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Same emit contract as MoneyInput — digit string out — but no grouping. */
export const Default: Story = {};

export const Zero: Story = {
  args: { value: 0 },
};

/**
 * `max` clamps what gets emitted: type past the cap and the value snaps back
 * to it. Used for "can't sell more than the lot holds".
 */
export const ClampedToMax: Story = {
  args: { value: "95", max: 100 },
};

export const Disabled: Story = {
  args: { value: "12", disabled: true },
};
