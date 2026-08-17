import { Stack } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import MoneyInput from "./MoneyInput";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

// MoneyInput is controlled, so every story drives it through local state seeded
// from `args.value` — the Controls panel sets the starting value, typing then
// works for real and the readout shows exactly what onChange handed back.
function Controlled({ value, ...rest }: React.ComponentProps<typeof MoneyInput>) {
  const [raw, setRaw] = useState(String(value ?? ""));
  useEffect(() => setRaw(String(value ?? "")), [value]);
  return (
    <Stack gap={2} maxW="240px">
      <MoneyInput {...rest} value={raw} onChange={setRaw} aria-label="price" />
      <Emitted>{raw}</Emitted>
    </Stack>
  );
}

const meta = {
  title: "Forms & inputs/MoneyInput",
  component: MoneyInput,
  parameters: storyDocs("money-input"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onChange: { control: false },
  },
  args: { value: "200000", onChange: () => {} },
  render: (args) => <Controlled {...args} />,
} satisfies Meta<typeof MoneyInput>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Type in it: input groups by the active locale, output stays raw digits. */
export const Default: Story = {};

/**
 * Zero renders EMPTY with a grey "0" placeholder — not a literal `0` you have
 * to delete first, which is what produces prices like `09000`.
 */
export const Zero: Story = {
  args: { value: 0 },
};

/** Grouping follows the active locale — flip the toolbar globe to compare. */
export const LargeAmount: Story = {
  args: { value: "184320000" },
};

export const Disabled: Story = {
  args: { value: "50000", disabled: true },
};

export const Small: Story = {
  args: { value: "50000", size: "sm" },
};

export const WithPlaceholder: Story = {
  args: { value: 0, placeholder: "Harga jual" },
};
