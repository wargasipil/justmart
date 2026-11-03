import { Stack, Text } from "@chakra-ui/react";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react";

import ManufacturerSelect from "./ManufacturerSelect";
import { ManufacturerService } from "../gen/inventory_iface/v1/manufacturer_connect";
import { MANUFACTURERS, filterManufacturers } from "../routes/dev/fixtures";
import { storyDocs } from "../routes/dev/storyDocs";
import { Emitted } from "../routes/dev/storyDecorators";
import { mockApi, mockRpc } from "../routes/dev/storyMocks";

// MOCKED, unlike its twin SupplierSelect — a deliberate divergence, not an
// oversight. `needs-backend` means the story reaches the dev server, which is
// honest for a live search but leaves the dropdown EMPTY whenever nobody is
// running `make run` — and an empty picker demonstrates nothing. These stories
// mock `searchManufacturers` as a FUNCTION of the request instead (the rule the
// page stories follow), against the same fixture makers and the same predicate
// the server applies: active-only, name-or-code match. So the component is
// always shown doing its actual job. ManufacturerService IS implemented now;
// this stays mocked because determinism is worth more here than a live call.

/** The server's SearchManufacturers, as the story's fake: name/code match, active only. */
const searchHandler = mockRpc(ManufacturerService, "searchManufacturers", (req) => ({
  manufacturers: filterManufacturers(MANUFACTURERS, {
    query: req.query,
    includeInactive: false,
  }).slice(0, req.limit || 20),
}));

/** What a pre-set value resolves its trigger label through. */
const resolveHandler = mockRpc(ManufacturerService, "resolveManufacturers", (req) => ({
  manufacturers: MANUFACTURERS.filter((m) => req.ids.includes(m.id)),
}));

const meta = {
  title: "components/selects/ManufacturerSelect",
  component: ManufacturerSelect,
  parameters: {
    ...storyDocs("manufacturer-select"),
    msw: mockApi(searchHandler, resolveHandler),
  },
  tags: ["autodocs"],
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
          <ManufacturerSelect
            {...args}
            value={value}
            onChange={setValue}
            onSelectItem={(m) => setLabel(m?.name ?? "")}
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
} satisfies Meta<typeof ManufacturerSelect>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Open it: 28 active pabrik, filtered live as you type (try "kalbe", "pt", or a
 * code like "MFR-001"). Rows lead with the NAME over a muted code, behind a
 * Factory glyph — the same mark the Inventaris nav uses for Pabrik, and what
 * tells this field apart from the Building2 supplier field beside it on a
 * product form. The exported `manufacturerLabel()` keeps the other format,
 * `CODE · Name`, for table cells where a column is scanned straight down.
 *
 * The two archived makers never appear: the fake mirrors the server's
 * active-only filter.
 */
export const Default: Story = {};

/**
 * Mounted with a value already set — the product edit form's case. The trigger
 * reads "PT Kalbe Farma Tbk · MFR-0001" straight away because the component
 * resolves its OWN label from the id (one bounded ResolveManufacturers call);
 * without that it would show a raw UUID until the first search fired.
 */
export const Preselected: Story = {
  args: { value: "mfr-1" },
};

/**
 * Read-only, for a locked row: pre-set value, no `onChange`. Still resolves its
 * label, so a disabled field shows the manufacturer rather than its id.
 */
export const ReadOnly: Story = {
  args: { value: "mfr-4", disabled: true },
};

/**
 * The search comes back empty. Worth pinning separately: it is what a typo
 * looks like, and it must read as "nothing matched" rather than as a broken
 * field.
 */
export const NoResults: Story = {
  parameters: {
    msw: mockApi(
      mockRpc(ManufacturerService, "searchManufacturers", { manufacturers: [] }),
      resolveHandler,
    ),
  },
};

export const Small: Story = {
  args: { size: "sm" },
};
