import { HStack, Stack, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import Barcode from "./Barcode";
import { storyDocs } from "../routes/dev/storyDocs";

const meta = {
  title: "Data display/Barcode",
  component: Barcode,
  parameters: storyDocs("barcode"),
  tags: ["autodocs"],
  args: { value: "SKU-00123" },
} satisfies Meta<typeof Barcode>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The SKU **is** the barcode — POS scans by exact-SKU match, so a label printed
 * from the catalog is guaranteed to resolve at the till. There is no separate
 * barcode column, and adding one would create two codes that can disagree.
 */
export const Default: Story = {};

/**
 * Black on white in BOTH themes — flip the theme toggle and it does not invert.
 * A barcode is an optical target, not decoration: inverting it makes it
 * unscannable and prints as a black rectangle.
 */
export const ThemeInvariant: Story = {};

export const Compact: Story = {
  args: { value: "OBAT-88219", height: 40, width: 1.5, fontSize: 11 },
};

export const WithoutText: Story = {
  args: { displayValue: false, height: 50 },
};

/** Wider modules scan more reliably — at the cost of paper width. */
export const ModuleWidths: Story = {
  render: () => (
    <HStack gap={6} flexWrap="wrap" align="flex-start">
      {[1, 2, 3].map((w) => (
        <Stack key={w} gap={1} align="center">
          <Barcode value="SKU-00123" width={w} height={50} fontSize={12} />
          <Text fontSize="xs" color="fg.muted">
            width={w}
          </Text>
        </Stack>
      ))}
    </HStack>
  ),
};

/**
 * CODE128 code set B covers printable ASCII only. A non-encodable value renders
 * a WARNING, never a fake symbol — a symbol that prints perfectly and scans as
 * a different string is the failure mode worth engineering against. Check
 * `isCode128Encodable` before offering a Print action; the backend's
 * `printer.Code128Encodable` refuses the same inputs.
 */
export const Unencodable: Story = {
  args: { value: "obat-é" },
};

/**
 * Long values need narrow modules to fit thermal paper. The backend DERIVES the
 * module width from the SKU length and the paper (`printer.FitModuleWidth`) and
 * refuses rather than emitting a truncated, dead label; the browser label-sheet
 * path has no such limit.
 */
export const LongValue: Story = {
  args: { value: "PRODUCT-2026-000123456", width: 1.4, fontSize: 11 },
};
