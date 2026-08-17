import { HStack, Stack, Text } from "@chakra-ui/react";
import type { Meta, StoryObj } from "@storybook/react";

import ExpiryBadge from "./ExpiryBadge";
import { storyDocs } from "../routes/dev/storyDocs";

function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

const meta = {
  title: "Data display/ExpiryBadge",
  component: ExpiryBadge,
  parameters: storyDocs("expiry-badge"),
  tags: ["autodocs"],
  args: { expiry: isoInDays(45) },
} satisfies Meta<typeof ExpiryBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The whole scale, worst first. ≤30d red · ≤90d orange · else green. */
export const Thresholds: Story = {
  render: () => (
    <Stack gap={3} align="flex-start">
      {[
        [-3, "already expired"],
        [12, "≤ 30 days — red"],
        [60, "≤ 90 days — orange"],
        [200, "beyond 90 days — green"],
        [500, "far out"],
      ].map(([days, note]) => (
        <HStack key={String(days)} gap={3}>
          <ExpiryBadge expiry={isoInDays(days as number)} />
          <Text fontSize="xs" color="fg.muted">
            {note}
          </Text>
        </HStack>
      ))}
    </Stack>
  ),
};

/** Expired lots keep showing — a spent batch still has movements behind it. */
export const Expired: Story = {
  args: { expiry: isoInDays(-30) },
};

/** The boundary days, where an off-by-one in the threshold would show. */
export const Boundaries: Story = {
  render: () => (
    <HStack gap={3} flexWrap="wrap">
      {[29, 30, 31, 89, 90, 91].map((d) => (
        <Stack key={d} gap={1} align="center">
          <ExpiryBadge expiry={isoInDays(d)} />
          <Text fontSize="xs" color="fg.muted">
            +{d}d
          </Text>
        </Stack>
      ))}
    </HStack>
  ),
};
