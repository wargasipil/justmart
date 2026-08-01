import { Box, HStack, Heading } from "@chakra-ui/react";
import type { ReactNode } from "react";

/**
 * The app's panel surface: a `bg.subtle` body under a `bg.muted` title bar.
 *
 * This was a page-local helper in ProductDetail while it was the only user; it
 * moved here once the batch list wanted the same frame, so the three places
 * that draw this surface (here, ChartCard, TableScroll's `framed`) agree by
 * construction instead of by three copies of the same three props.
 *
 * `overflow: hidden` is what clips the children to the rounded corners — so a
 * <TableScroll> inside should be `framed={false}` (this draws the border) and
 * its own scrolling still works, since the scroll container is a descendant.
 */
export type CardProps = {
  title: string;
  /** Right-aligned slot in the title bar — page actions for this panel. */
  actions?: ReactNode;
  /** Rendered flush; pad it yourself so a table can sit edge-to-edge. */
  children: ReactNode;
};

export default function Card({ title, actions, children }: CardProps) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" overflow="hidden">
      <HStack bg="bg.muted" px={4} py={2} justify="space-between" gap={3}>
        <Heading size="sm">{title}</Heading>
        {actions}
      </HStack>
      {children}
    </Box>
  );
}
