import { Stack, StackSeparator } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";
import { fn } from "storybook/test";

import ManufacturerListItemMobile, {
  ManufacturerListItemMobileSkeleton,
} from "./ManufacturerListItemMobile";
import { MANUFACTURERS } from "../../routes/dev/fixtures";
import { storyDocs } from "../../routes/dev/storyDocs";
import { MOBILE, MOBILE_PAGE } from "../../screens/viewports";

// Pinned to the mobile viewport — that is the only width this row is for.
// Fixture pabrik carry no image of any kind, so no story here makes a request.

const KALBE = MANUFACTURERS[0];
// MFR-0029 in the fixtures: closed, and with neither phone nor email.
const ARCHIVED = MANUFACTURERS.find((m) => !m.active)!;

const meta = {
  title: "components/manufacturer/ManufacturerListItemMobile",
  component: ManufacturerListItemMobile,
  globals: MOBILE,
  parameters: { ...storyDocs("manufacturer-list-item-mobile"), ...MOBILE_PAGE },
  tags: ["autodocs"],
  args: { manufacturer: KALBE, onClick: fn() },
} satisfies Meta<typeof ManufacturerListItemMobile>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Tappable row: name, code, contact line, chevron. */
export const Default: Story = {};

/** No onClick: a plain row — no chevron, not a button. */
export const ReadOnly: Story = { args: { onClick: undefined } };

/** No phone: the contact line falls through to the email. */
export const EmailOnly: Story = {
  args: { manufacturer: { ...KALBE, phone: "" } },
};

/** Neither phone nor email: the address carries the row instead. */
export const AddressOnly: Story = {
  args: { manufacturer: { ...KALBE, phone: "", contactEmail: "" } },
};

/** No contact details at all — the line is dropped rather than printing "—". */
export const NoContact: Story = {
  args: { manufacturer: { ...KALBE, phone: "", contactEmail: "", address: "" } },
};

/** Archived pabrik: dimmed, with a badge beside the name. */
export const Archived: Story = { args: { manufacturer: ARCHIVED } };

/** Long names clamp to two lines; the code and chevron keep their place. */
export const LongName: Story = {
  args: {
    manufacturer: {
      ...KALBE,
      name: "PT Industri Farmasi Nusantara Sejahtera Abadi Makmur Bersama Tbk",
      address: "Kawasan Industri Jababeka II, Blok CC No. 12, Cikarang Selatan, Bekasi",
    },
  },
};

/** As a list: a page of rows with hairlines between them, archived ones included. */
export const List: Story = {
  render: (args) => (
    <Stack gap={0} separator={<StackSeparator />}>
      {[...MANUFACTURERS.slice(0, 6), ARCHIVED].map((m) => (
        <ManufacturerListItemMobile
          key={m.id}
          manufacturer={m}
          onClick={args.onClick}
        />
      ))}
    </Stack>
  ),
};

/** Loading placeholder: one skeleton row in the same shape as a real row. */
export const LoadingSkeleton: Story = {
  render: () => <ManufacturerListItemMobileSkeleton chevron />,
};

/** Loading a page: skeleton rows with the same hairlines as List. */
export const LoadingSkeletonList: Story = {
  render: () => (
    <Stack gap={0} separator={<StackSeparator />}>
      {Array.from({ length: 8 }, (_, i) => (
        <ManufacturerListItemMobileSkeleton key={i} chevron />
      ))}
    </Stack>
  ),
};
