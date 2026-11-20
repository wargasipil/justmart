import { useState } from "react";
import { Table } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";
import { expect, userEvent, within } from "storybook/test";
import { useTranslation } from "react-i18next";

import SortableColumnHeader, { cycleColumnSort, type ColumnSort } from "./SortableColumnHeader";
import TableScroll from "./TableScroll";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "components/data/SortableColumnHeader",
  component: SortableColumnHeader,
  parameters: storyDocs("sortable-column-header"),
  tags: ["autodocs"],
  argTypes: { onClick: { control: false } },
  // onClick is a no-op here: the single-header stories pin one visual state;
  // `Interactive` below holds the sort in state.
  args: { label: "Qty", active: false, desc: false, onClick: () => {} },
  // A <th> is only valid inside a header row, so every story renders in one.
  render: (args) => (
    <Table.Root size="sm" maxW="xs">
      <Table.Header>
        <Table.Row>
          <SortableColumnHeader {...args} />
        </Table.Row>
      </Table.Header>
    </Table.Root>
  ),
} satisfies Meta<typeof SortableColumnHeader>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Not the sorted column: no arrow, but the 12px slot is still reserved. */
export const Unsorted: Story = {};

export const Ascending: Story = {
  args: { active: true },
};

export const Descending: Story = {
  args: { active: true, desc: true },
};

/** Over a right-aligned numeric column. */
export const EndAligned: Story = {
  args: { active: true, desc: true, align: "end" },
};

const ROWS = [
  { batch: "B-2026-0031", expiry: "2027-09-30", qty: 40 },
  { batch: "B-2026-0007", expiry: "2027-02-28", qty: 120 },
  { batch: "B-2026-0019", expiry: "2027-05-31", qty: 75 },
];
type Row = (typeof ROWS)[number];

/**
 * The real cycle: click (or Tab + Enter) a header — ascending, then descending,
 * then back to the default order. Sorted in memory here; a paginated list
 * sends the sort to its List RPC instead. The play walks the Qty column through
 * all three steps, once by keyboard, which also pins that one Enter fires ONE
 * step (the button's click bubbles to the header's single handler).
 */
export const Interactive: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const qty = canvas.getAllByRole("columnheader")[2];
    const firstLot = () => within(canvas.getAllByRole("row")[1]).getAllByRole("cell")[0];

    await expect(qty).toHaveAttribute("aria-sort", "none");
    await userEvent.click(qty);
    await expect(qty).toHaveAttribute("aria-sort", "ascending");
    await expect(firstLot()).toHaveTextContent("B-2026-0031"); // qty 40

    within(qty).getByRole("button").focus();
    await userEvent.keyboard("{Enter}");
    await expect(qty).toHaveAttribute("aria-sort", "descending");
    await expect(firstLot()).toHaveTextContent("B-2026-0007"); // qty 120

    await userEvent.click(within(qty).getByRole("button"));
    await expect(qty).toHaveAttribute("aria-sort", "none"); // back to the default order
    await expect(firstLot()).toHaveTextContent("B-2026-0031");
  },
  render: () => {
    function Demo() {
      const { t } = useTranslation();
      const [sort, setSort] = useState<ColumnSort>(null);
      const rows = [...ROWS];
      if (sort) {
        const key = sort.by as keyof Row;
        rows.sort((a, b) => (a[key] < b[key] ? -1 : a[key] > b[key] ? 1 : 0) * (sort.desc ? -1 : 1));
      }
      const sortable = (by: keyof Row) => ({
        active: sort?.by === by,
        desc: sort?.by === by && sort.desc,
        onClick: () => setSort((s) => cycleColumnSort(s, by)),
      });
      return (
        <TableScroll maxH="240px">
          <Table.Root size="sm" stickyHeader>
            <Table.Header>
              <Table.Row>
                <SortableColumnHeader label={t("inventory.batches.batchNumber")} {...sortable("batch")} />
                <SortableColumnHeader label={t("inventory.batches.expiry")} {...sortable("expiry")} />
                <SortableColumnHeader label={t("inventory.batches.qty")} align="end" {...sortable("qty")} />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {rows.map((r) => (
                <Table.Row key={r.batch}>
                  <Table.Cell fontFamily="mono">{r.batch}</Table.Cell>
                  <Table.Cell>{r.expiry}</Table.Cell>
                  <Table.Cell textAlign="end">{r.qty}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </TableScroll>
      );
    }
    return <Demo />;
  },
};
