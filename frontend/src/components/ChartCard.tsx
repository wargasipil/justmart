import { Box, HStack, Spinner, Stack, Text } from "@chakra-ui/react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

// The framed surface every chart sits in — same card vocabulary as
// DashboardTile (bg.subtle + 1px border + radius lg), plus the loading and
// empty states so no page hand-rolls a spinner around a Recharts container.
// Pair it with <TrendChart> for the plot itself.
type Props = {
  /** Localized chart title (names the single series, so no legend is needed). */
  title: string;
  /** Optional muted sub-line under the title. */
  description?: string;
  /** Right-aligned slot in the header (a filter, a toggle). */
  actions?: ReactNode;
  /** Plot height. Default 240px. */
  height?: string;
  isLoading?: boolean;
  /** Renders the "no results" placeholder instead of the children. */
  isEmpty?: boolean;
  children: ReactNode;
};

export default function ChartCard({
  title,
  description,
  actions,
  height = "240px",
  isLoading,
  isEmpty,
  children,
}: Props) {
  const { t } = useTranslation();
  return (
    <Stack gap={3} bg="bg.subtle" borderWidth="1px" borderColor="border" borderRadius="lg" p={4}>
      <HStack justify="space-between" align="flex-start" gap={3}>
        <Stack gap={0.5}>
          <Text fontSize="sm" fontWeight="medium" color="fg">
            {title}
          </Text>
          {description && (
            <Text fontSize="xs" color="fg.muted">
              {description}
            </Text>
          )}
        </Stack>
        {actions}
      </HStack>
      <Box h={height} position="relative">
        {isLoading ? (
          <Center>
            <Spinner size="sm" colorPalette="blue" />
          </Center>
        ) : isEmpty ? (
          <Center>
            <Text fontSize="sm" color="fg.muted">
              {t("common.noResults")}
            </Text>
          </Center>
        ) : (
          children
        )}
      </Box>
    </Stack>
  );
}

function Center({ children }: { children: ReactNode }) {
  return (
    <Box display="flex" alignItems="center" justifyContent="center" h="100%" w="100%">
      {children}
    </Box>
  );
}
