import { Box, Stat } from "@chakra-ui/react";

// Shared tile for a list page's summary bar — the server-side aggregate over
// ALL rows matching the active filters, shown above the table (Orders, Products,
// Restock).
//
// Built on Chakra's Stat primitive (ChakraUI-first rule), so the label/value/
// help text carry Stat's semantics — it renders a <dl>/<dt>/<dd>, which is what
// a metric actually is — instead of three hand-styled <Text>s. The surrounding
// card chrome is ours: Stat ships no container, and this strip needs one.
//
// Distinct from <DashboardTile>: that one is a landing-page card (bigger, often
// clickable, tone-colored). This is the flatter strip that sits between a
// toolbar and a table, so it stays compact and never navigates.
//
// One figure per tile. A count and its valuation are two cells side by side, not
// one cell with a subtitle — as peers they stay comparable, and the caller owns
// the grid that pairs them (see the 4-up row on /products).
type Props = {
  label: string;
  value: string;
};

export default function SummaryTile({ label, value }: Props) {
  return (
    <Box flex="1" minW="160px" bg="bg.subtle" borderWidth="1px" borderRadius="lg" px={4} py={3}>
      <Stat.Root size="sm">
        <Stat.Label fontSize="xs" color="fg.muted">
          {label}
        </Stat.Label>
        <Stat.ValueText fontSize="xl" fontWeight="semibold">
          {value}
        </Stat.ValueText>
      </Stat.Root>
    </Box>
  );
}
