import { Box, Card, Table, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import TableScroll, { TABLE_MAX_H_NESTED } from "./TableScroll";
import { storyDocs } from "../routes/dev/storyDocs";

function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

// 24 lots — enough to overflow the cap so the header actually sticks.
const ROWS = Array.from({ length: 24 }, (_, i) => ({
  product: "Paracetamol 500mg",
  batch: `B-2026-${String(i + 1).padStart(4, "0")}`,
  expiry: isoInDays(30 * (i + 1)),
  qty: 120 - i * 3,
}));

function Rows({ wide = false }: { wide?: boolean }) {
  return (
    <Table.Root size="sm" stickyHeader>
      <Table.Header>
        <Table.Row>
          <Table.ColumnHeader>Produk</Table.ColumnHeader>
          <Table.ColumnHeader>No. batch</Table.ColumnHeader>
          <Table.ColumnHeader>Kedaluwarsa</Table.ColumnHeader>
          {wide ? (
            <>
              <Table.ColumnHeader>Pemasok</Table.ColumnHeader>
              <Table.ColumnHeader>Gudang</Table.ColumnHeader>
              <Table.ColumnHeader>Harga beli</Table.ColumnHeader>
              <Table.ColumnHeader>Diterima</Table.ColumnHeader>
            </>
          ) : null}
          <Table.ColumnHeader textAlign="end">Qty</Table.ColumnHeader>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {ROWS.map((r) => (
          <Table.Row key={r.batch}>
            <Table.Cell whiteSpace="nowrap">{r.product}</Table.Cell>
            <Table.Cell fontFamily="mono">{r.batch}</Table.Cell>
            <Table.Cell>{r.expiry}</Table.Cell>
            {wide ? (
              <>
                <Table.Cell whiteSpace="nowrap">PT Kimia Farma Trading</Table.Cell>
                <Table.Cell whiteSpace="nowrap">Gudang Utama</Table.Cell>
                <Table.Cell whiteSpace="nowrap">Rp 1.850</Table.Cell>
                <Table.Cell whiteSpace="nowrap">{isoInDays(-90)}</Table.Cell>
              </>
            ) : null}
            <Table.Cell textAlign="end">{r.qty}</Table.Cell>
          </Table.Row>
        ))}
      </Table.Body>
    </Table.Root>
  );
}

const meta = {
  title: "Data display/TableScroll",
  component: TableScroll,
  parameters: storyDocs("table-scroll"),
  tags: ["autodocs"],
  argTypes: { children: { control: false } },
  // Each story passes its own table via `render`.
  args: { children: null },
} satisfies Meta<typeof TableScroll>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Scroll it — the header stays put and the page behind never moves. */
export const Default: Story = {
  render: () => (
    <>
      <TableScroll maxH="240px">
        <Rows />
      </TableScroll>
      <Text fontSize="xs" color="fg.muted" mt={2}>
        The header background is applied by TableScroll — Chakra's `line` variant gives headers
        none, and a transparent sticky header lets rows scroll straight through it.
      </Text>
    </>
  ),
};

/**
 * A table wider than its frame scrolls on BOTH axes inside its own box. This is
 * what stops a wide table pushing the whole page sideways.
 */
export const HorizontalOverflow: Story = {
  render: () => (
    <Box maxW="520px">
      <TableScroll maxH="240px">
        <Rows wide />
      </TableScroll>
    </Box>
  ),
};

/**
 * `framed={false}` inside a Card — otherwise you get a double border. Pair it
 * with `TABLE_MAX_H_NESTED` rather than the list-page default.
 */
export const InsideACard: Story = {
  render: () => (
    <Card.Root>
      <Card.Header>
        <Card.Title>Batch</Card.Title>
      </Card.Header>
      <Card.Body>
        <TableScroll framed={false} maxH={TABLE_MAX_H_NESTED}>
          <Rows />
        </TableScroll>
      </Card.Body>
    </Card.Root>
  ),
};
