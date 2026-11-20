import { Box, chakra, Table } from "@chakra-ui/react";
import { ArrowDown, ArrowUp } from "lucide-react";

/** The active sort of a table: a column key + direction, or null = the list's default order. */
export type ColumnSort = { by: string; desc: boolean } | null;

/**
 * The three-step header cycle: a new column sorts ascending, a second click on
 * it flips to descending, a third clears back to the default order (null).
 */
export function cycleColumnSort(current: ColumnSort, by: string): ColumnSort {
  if (current?.by !== by) return { by, desc: false };
  if (!current.desc) return { by, desc: true };
  return null;
}

export type SortableColumnHeaderProps = {
  label: string;
  /** This column is the one the table is sorted by. */
  active: boolean;
  /** Direction of the active sort; ignored while `active` is false. */
  desc: boolean;
  onClick: () => void;
  /** "end" for a numeric column, so the header sits over its right-aligned cells. */
  align?: "start" | "end";
};

// A clickable column header. The whole cell is the hit area (onClick sits on
// the <th>); the label is wrapped in a real <button> so the header is reachable
// by Tab and activates on Enter/Space. That button has no handler of its own:
// its click — mouse or keyboard-generated — bubbles to the header, so there is
// exactly one call per activation. The arrow sits in a fixed 12px slot so the
// label does not jump sideways when the indicator appears or clears (same
// design as MetricTable's header).
export default function SortableColumnHeader({
  label,
  active,
  desc,
  onClick,
  align = "start",
}: SortableColumnHeaderProps) {
  const direction = active ? (desc ? "descending" : "ascending") : "none";
  return (
    <Table.ColumnHeader
      textAlign={align}
      whiteSpace="nowrap"
      cursor="pointer"
      userSelect="none"
      aria-sort={direction}
      onClick={onClick}
    >
      <chakra.button
        type="button"
        display="inline-flex"
        alignItems="center"
        gap={1}
        font="inherit"
        color="inherit"
        cursor="inherit"
        borderRadius="xs"
        colorPalette="blue"
        _focusVisible={{
          outline: "2px solid",
          outlineColor: "colorPalette.focusRing",
          outlineOffset: "2px",
        }}
      >
        <span>{label}</span>
        <Box as="span" display="inline-block" w="12px" flexShrink={0} lineHeight={0} aria-hidden>
          {active && (desc ? <ArrowDown size={12} /> : <ArrowUp size={12} />)}
        </Box>
      </chakra.button>
    </Table.ColumnHeader>
  );
}
