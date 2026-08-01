import { Table } from "@chakra-ui/react";
import type { ReactNode } from "react";

/**
 * Bounded scroll viewport for a data table.
 *
 * Wraps Chakra's `Table.ScrollArea`, whose base style is
 * `overflow: auto; max-width: 100%; white-space: nowrap` — so the table scrolls
 * inside its own box on BOTH axes and can never push the page sideways. We add
 * the one thing it doesn't ship: a `maxH`, so a 100-row page stops growing the
 * document and the toolbar above / pager below stay on screen.
 *
 * Pair it with `stickyHeader` on the `Table.Root` inside. Because the scroll
 * container is this box (not the window), the variant's
 * `--table-sticky-offset` default of 0 is already correct — no TopBar height to
 * hardcode.
 *
 *   <TableScroll>
 *     <Table.Root size="sm" stickyHeader>…</Table.Root>
 *   </TableScroll>
 *
 * `framed` (default) draws the border/radius/surface that list tables used to
 * put on `Table.Root` itself — move those off the Root or you get a double
 * border. Pass `framed={false}` when the table already sits inside a Card.
 */
export type TableScrollProps = {
  children: ReactNode;
  /** Viewport cap. Default leaves room for the page chrome of a plain list page. */
  maxH?: string;
  /** Draw the surrounding border + surface. Off when nested in a Card. */
  framed?: boolean;
};

/** Fits a list page: TopBar + PageHeader + toolbar + pager. */
export const TABLE_MAX_H = "calc(100vh - 300px)";
/** Fits a table nested in a detail-page card or tab panel. */
export const TABLE_MAX_H_NESTED = "420px";

export default function TableScroll({
  children,
  maxH = TABLE_MAX_H,
  framed = true,
}: TableScrollProps) {
  return (
    <Table.ScrollArea
      maxH={maxH}
      // A sticky header MUST be opaque or the body scrolls visibly through it.
      // Chakra's default `line` variant gives the header no background at all
      // (only `outline` does), so the invariant lives here rather than in 30+
      // call sites where one omission is an invisible-until-you-scroll bug.
      // Every header in the app already uses bg.muted, so this repaints nothing.
      // The shadow replaces the bottom border, which `borderCollapse: collapse`
      // drops once the row is stuck.
      css={{
        "& thead tr": {
          bg: "bg.muted",
          boxShadow: "0 1px 0 var(--chakra-colors-border)",
        },
      }}
      {...(framed
        ? { bg: "bg.subtle", borderWidth: "1px", borderRadius: "lg" }
        : {})}
    >
      {children}
    </Table.ScrollArea>
  );
}
