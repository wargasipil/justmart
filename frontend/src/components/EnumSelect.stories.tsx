import { Stack } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import EnumSelect from "./EnumSelect";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

type Option = { value: string; label: string };

const PO_STATUS: Option[] = [
  { value: "DRAFT", label: "Draft" },
  { value: "SENT", label: "Terkirim" },
  { value: "PARTIALLY_RECEIVED", label: "Diterima sebagian" },
  { value: "RECEIVED", label: "Diterima" },
  { value: "CLOSED", label: "Selesai" },
  { value: "VOIDED", label: "Batal" },
];

const PAYMENT: Option[] = [
  { value: "CASH", label: "Tunai" },
  { value: "CARD", label: "Kartu" },
  { value: "TRANSFER", label: "Transfer" },
];

function Controlled({ value, ...rest }: React.ComponentProps<typeof EnumSelect<Option>>) {
  const [picked, setPicked] = useState(value ?? "");
  useEffect(() => setPicked(value ?? ""), [value]);
  return (
    <Stack gap={2} maxW="260px">
      <EnumSelect<Option> {...rest} value={picked} onChange={setPicked} />
      <Emitted>{picked}</Emitted>
    </Stack>
  );
}

const meta = {
  title: "Selects & pickers/EnumSelect",
  component: EnumSelect<Option>,
  parameters: storyDocs("enum-select"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md", "lg"] },
    items: { control: false },
    itemToString: { control: false },
    itemToValue: { control: false },
    onChange: { control: false },
  },
  args: {
    value: "SENT",
    items: PO_STATUS,
    itemToString: (o: Option) => o.label,
    itemToValue: (o: Option) => o.value,
    onChange: () => {},
  },
  render: (args) => <Controlled {...args} />,
} satisfies Meta<typeof EnumSelect<Option>>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * For SHORT fixed option sets only (≤ ~20 hardcoded items): statuses, roles,
 * payment sources, page sizes. Anything from a queryable domain must use
 * `<SearchableSelect loadOptions={…}>` against a Search RPC instead.
 */
export const Default: Story = {};

export const Unselected: Story = {
  args: { value: "", placeholder: "Semua status" },
};

export const ShortList: Story = {
  args: { value: "CASH", items: PAYMENT },
};

export const Disabled: Story = {
  args: { disabled: true },
};

export const Small: Story = {
  args: { size: "sm" },
};
