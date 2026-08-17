import { Stack } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import DatePickerField from "./DatePicker";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

function Controlled({ value, ...rest }: React.ComponentProps<typeof DatePickerField>) {
  const [date, setDate] = useState(value ?? "");
  useEffect(() => setDate(value ?? ""), [value]);
  return (
    <Stack gap={2} maxW="260px">
      <DatePickerField {...rest} value={date} onChange={setDate} />
      <Emitted>{date}</Emitted>
    </Stack>
  );
}

const meta = {
  title: "Forms & inputs/DatePicker",
  component: DatePickerField,
  parameters: storyDocs("date-picker"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    onChange: { control: false },
  },
  args: { value: isoInDays(0), onChange: () => {} },
  render: (args) => <Controlled {...args} />,
} satisfies Meta<typeof DatePickerField>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Open it: day / month / year views, and the ISO date is derived from the
 * DateValue rather than parsed back out of the locale-formatted string.
 */
export const Default: Story = {};

export const Empty: Story = {
  args: { value: "", placeholder: "Pilih tanggal" },
};

/** Bounded — an expiry that can't be in the past, for instance. */
export const Bounded: Story = {
  args: { value: isoInDays(30), min: isoInDays(0), max: isoInDays(365) },
};

/** `clearable` (default true) puts an inline × in the control; it emits "". */
export const NotClearable: Story = {
  args: { value: isoInDays(0), clearable: false },
};

export const Disabled: Story = {
  args: { value: isoInDays(0), disabled: true },
};
