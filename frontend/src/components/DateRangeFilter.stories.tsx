import { Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import DateRangeFilter from "./DateRangeFilter";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";
import { formatAbsolute, rangeBounds, resolveRange, type DateRange } from "../lib/dateRange";

const meta = {
  title: "Toolbar & filters/DateRangeFilter",
  component: DateRangeFilter,
  parameters: storyDocs("date-range-filter"),
  tags: ["autodocs"],
  argTypes: {
    size: { control: "inline-radio", options: ["xs", "sm", "md"] },
    value: { control: false },
    fields: { control: false },
    onChange: { control: false },
    onFieldChange: { control: false },
  },
  // Each story owns its own DateRange state; these satisfy the required props.
  args: { value: resolveRange("30d"), onChange: () => {} },
} satisfies Meta<typeof DateRangeFilter>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * One button opening a two-pane popover: absolute range on the left (down to
 * the second, plus an inline range calendar), searchable quick ranges on the
 * right. Don't hand-roll a preset `<EnumSelect>` plus two date pickers — this
 * is what replaced that.
 *
 * The readout shows the caller's half of the contract: a `DateRange` always
 * carries resolved `fromUnix`/`toUnix`, so the page never branches on
 * quick-vs-absolute.
 */
export const Default: Story = {
  render: (args) => {
    function Demo() {
      const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
      const { from, to } = rangeBounds(range);
      return (
        <Stack gap={2} align="flex-start">
          <DateRangeFilter {...args} value={range} onChange={setRange} />
          <Emitted>{`${range.preset} · ${formatAbsolute(from)} → ${formatAbsolute(to)}`}</Emitted>
          <Emitted>{`fromUnix ${range.fromUnix} · toUnix ${range.toUnix}`}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

/** A short quick range — the seconds-level bounds are visible in the readout. */
export const ShortRange: Story = {
  render: (args) => {
    function Demo() {
      const [range, setRange] = useState<DateRange>(() => resolveRange("6h"));
      return (
        <Stack gap={2} align="flex-start">
          <DateRangeFilter {...args} value={range} onChange={setRange} />
          <Emitted>{`${range.preset} · fromUnix ${range.fromUnix} · toUnix ${range.toUnix}`}</Emitted>
        </Stack>
      );
    }
    return <Demo />;
  },
};

/**
 * **Which date? is the `fields` prop, not a second control.** A list with more
 * than one filterable date renders a SegmentGroup row at the top of the
 * popover, with "Any date" prepended.
 *
 * "Any date" reports back as `field: ""` and is the filter's OFF state — the
 * caller then sends `dateField: ""` **and no bounds**, which is what the second
 * readout tracks.
 */
export const MultipleDateFields: Story = {
  render: (args) => {
    function Demo() {
      const [range, setRange] = useState<DateRange>(() => resolveRange("30d"));
      const [field, setField] = useState("");
      return (
        <Stack gap={2} align="flex-start">
          <DateRangeFilter
            {...args}
            value={range}
            onChange={setRange}
            fields={[
              { value: "created", label: "Dibuat" },
              { value: "received", label: "Diterima" },
            ]}
            field={field}
            onFieldChange={setField}
          />
          <Emitted>{`dateField "${field}"`}</Emitted>
          <Emitted>
            {field ? `fromUnix ${range.fromUnix} · toUnix ${range.toUnix}` : "fromUnix 0 · toUnix 0"}
          </Emitted>
          <Text fontSize="xs" color="fg.muted">
            Field labels are bare nouns — they render as segments, and the trigger already reads
            &ldquo;Dibuat · 30 hari terakhir&rdquo;.
          </Text>
        </Stack>
      );
    }
    return <Demo />;
  },
};

export const ExtraSmall: Story = {
  args: { size: "xs" },
  render: (args) => {
    function Demo() {
      const [range, setRange] = useState<DateRange>(() => resolveRange("7d"));
      return <DateRangeFilter {...args} value={range} onChange={setRange} />;
    }
    return <Demo />;
  },
};
