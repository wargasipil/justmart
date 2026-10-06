import { Stack } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import ExpiryInput from "./ExpiryInput";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";

// Dates relative to today, in LOCAL calendar terms (toISOString is UTC and
// drifts a day near midnight), so the soon/expired readouts stay true.
function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function Controlled({ value, ...rest }: React.ComponentProps<typeof ExpiryInput>) {
  const [iso, setIso] = useState(value);
  useEffect(() => setIso(value), [value]);
  return (
    <Stack gap={2} maxW="280px">
      <ExpiryInput {...rest} value={iso} onChange={setIso} aria-label="Expiry" />
      <Emitted>{iso}</Emitted>
    </Stack>
  );
}

const meta = {
  title: "components/forms/ExpiryInput",
  component: ExpiryInput,
  parameters: storyDocs("expiry-input"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md"] },
    onChange: { control: false },
  },
  args: { value: "", onChange: () => {} },
  render: (args) => <Controlled {...args} />,
} satisfies Meta<typeof ExpiryInput>;

export default meta;
type Story = StoryObj<typeof meta>;

// Types like a person reading the pack, then checks what the field EMITS — the
// readout under it is locale text, the emitted date is the contract.
async function typeAndExpect(canvasElement: HTMLElement, text: string, iso: string) {
  const canvas = within(canvasElement);
  const input = canvas.getByLabelText("Expiry");
  await userEvent.clear(input);
  await userEvent.type(input, text);
  await waitFor(() => expect(canvas.getByText(iso)).toBeInTheDocument());
  await expect(input).not.toHaveAttribute("aria-invalid");
}

/** Empty: the readout explains what to type. */
export const Empty: Story = {};

/**
 * The common case: the pack says "EXP 03/2027". Four digits are month + year,
 * stored as the LAST day of that month — how a month-only expiry is read.
 */
export const MonthOnly: Story = {
  play: async ({ canvasElement }) => {
    await typeAndExpect(canvasElement, "0327", "2027-03-31");
    await typeAndExpect(canvasElement, "3/2027", "2027-03-31");
    await typeAndExpect(canvasElement, "022028", "2028-02-29"); // leap year end of month
  },
};

/** A pack that prints the day: day first, whatever the screen language. */
export const ExactDay: Story = {
  play: async ({ canvasElement }) => {
    await typeAndExpect(canvasElement, "050327", "2027-03-05");
    await typeAndExpect(canvasElement, "05/03/27", "2027-03-05");
    await typeAndExpect(canvasElement, "05032027", "2027-03-05");
  },
};

/**
 * Something that is not a date says so instead of being dropped on blur, and
 * emits "" so a required field stays unsatisfied.
 */
export const Invalid: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const input = canvas.getByLabelText("Expiry");
    await userEvent.type(input, "31/02/27"); // there is no 31 February
    await waitFor(() => expect(input).toHaveAttribute("aria-invalid", "true"));
  },
};

/** Pre-filled from the product's setting and not typed over: the readout says so. */
export const ProductDefault: Story = {
  args: { value: isoInDays(400), isDefault: true },
};

/** Within 30 days — the dashboard's "expiring soon" window — reads as a warning. */
export const ExpiringSoon: Story = {
  args: { value: isoInDays(12) },
};

/** Already past, in retail: a warning — the shop may still be recording it. */
export const ExpiredWarns: Story = {
  args: { value: isoInDays(-3) },
};

/** Already past, in pharmacy mode: an error. The receive is refused server-side too. */
export const ExpiredBlocked: Story = {
  args: { value: isoInDays(-3), blockExpired: true },
  play: async ({ canvasElement }) => {
    const input = within(canvasElement).getByLabelText("Expiry");
    await waitFor(() => expect(input).toHaveAttribute("aria-invalid", "true"));
  },
};

export const Small: Story = {
  args: { value: isoInDays(200), size: "sm" },
};
