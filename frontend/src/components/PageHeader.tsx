import { Box, Flex, HStack, Heading, Stack, Text } from "@chakra-ui/react";
import type { ReactNode } from "react";

type Props = {
  title: string;
  /** Inline slot right after the title — a status Badge belongs here, not in the page body. */
  titleBadge?: ReactNode;
  description?: string;
  actions?: ReactNode;
};

/**
 * Page chrome: title + optional badge/description + right-aligned actions.
 *
 * There is deliberately no `breadcrumbs` prop — the trail lives once in the
 * TopBar (<Breadcrumbs/>) and is derived from the URL. Pages used to hand-pass
 * it here, which reprinted the title on 14 of 21 pages and drifted from the
 * router. A detail page contributes only its entity name, via useCrumbLabel().
 */
export default function PageHeader({ title, titleBadge, description, actions }: Props) {
  return (
    <Stack gap={3} pb={4} mb={4} borderBottomWidth="1px">
      {/* Wraps on a phone: the actions drop under the title instead of pushing
          the page sideways — but only when they genuinely don't fit. The title
          block's basis is 12rem rather than its content width, so a long
          description wraps inside it instead of shoving a single Add button
          onto its own line; the button stays top-right. */}
      <Flex align="center" justify="space-between" gap={4} wrap="wrap">
        <Box minW={0} flex="1 1 12rem">
          <HStack gap={2}>
            <Heading size="xl">{title}</Heading>
            {titleBadge}
          </HStack>
          {description && (
            <Text fontSize="sm" color="fg.muted" mt={1}>
              {description}
            </Text>
          )}
        </Box>
        {actions && (
          <HStack gap={2} wrap="wrap">
            {actions}
          </HStack>
        )}
      </Flex>
    </Stack>
  );
}
