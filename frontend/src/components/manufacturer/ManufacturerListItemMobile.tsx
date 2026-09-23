import {
  Badge,
  Box,
  Flex,
  HStack,
  Skeleton,
  SkeletonText,
  Stack,
  Text,
  chakra,
} from "@chakra-ui/react";
import { ChevronRight, Factory } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Manufacturer } from "../../gen/inventory_iface/v1/manufacturer_pb";

export type ManufacturerListItemMobileData = Pick<
  Manufacturer,
  "id" | "code" | "name" | "phone" | "contactEmail" | "address" | "active"
>;

type Props = {
  manufacturer: ManufacturerListItemMobileData;
  /** Tapping the row. Omit for a read-only row (no chevron, not a button). */
  onClick?: () => void;
};

// A pabrik as one phone-width row: the admin table's eight columns don't fit at
// 390px, so the row keeps the three things that identify a manufacturer — name,
// code, and one way to reach it — and drops the rest to the detail page.
//
// Edit / Archive are deliberately NOT here. They are the table's last column on
// a desk, and a row with two buttons on it is a row you archive by mistake with
// a thumb; tapping opens the detail page, which carries both actions already.
export default function ManufacturerListItemMobile({ manufacturer, onClick }: Props) {
  const { t } = useTranslation();
  // One contact line, in the order a shop actually reaches a factory. Address
  // is the last resort rather than a fourth line: at this width a second wrap
  // pushes the code off the row it belongs to.
  const contact = manufacturer.phone || manufacturer.contactEmail || manufacturer.address;

  const body = (
    <HStack gap={3} align="center" w="full" minW={0} py={2} opacity={manufacturer.active ? 1 : 0.6}>
      {/* Same glyph + muted square as <ManufacturerOption> in ManufacturerSelect,
          so a pabrik looks like a pabrik wherever it is listed. A 40px box, not
          the product row's 48px thumbnail: that slot holds a photo, this one a
          mark, and a mark scaled to photo size reads as a missing image. */}
      <Flex
        align="center"
        justify="center"
        boxSize={10}
        flexShrink={0}
        borderRadius="md"
        bg="bg.muted"
        color="fg.muted"
      >
        <Factory size={18} />
      </Flex>
      <Stack gap={0.5} flex="1" minW={0}>
        <HStack gap={2} minW={0}>
          <Text fontWeight="medium" lineClamp={2}>
            {manufacturer.name}
          </Text>
          {!manufacturer.active && (
            <Badge size="sm" colorPalette="gray" flexShrink={0}>
              {t("common.archived")}
            </Badge>
          )}
        </HStack>
        <Text fontSize="xs" color="fg.muted" fontFamily="mono" truncate>
          {manufacturer.code}
        </Text>
        {contact && (
          <Text fontSize="sm" color="fg.muted" truncate>
            {contact}
          </Text>
        )}
      </Stack>
      {onClick && (
        <Box color="fg.muted" flexShrink={0}>
          <ChevronRight size={16} />
        </Box>
      )}
    </HStack>
  );

  if (!onClick) return body;
  return (
    <chakra.button
      type="button"
      onClick={onClick}
      w="full"
      textAlign="start"
      px={3}
      borderRadius="md"
      _hover={{ bg: "bg.muted" }}
      _focusVisible={{ outline: "2px solid", outlineColor: "colorPalette.focusRing" }}
    >
      {body}
    </chakra.button>
  );
}

// The loading placeholder for one ManufacturerListItemMobile row: same 40px
// mark slot and the same three text lines, so rows swap in without the list
// jumping. Pass `chevron` when the real rows will be tappable.
export function ManufacturerListItemMobileSkeleton({ chevron }: { chevron?: boolean }) {
  return (
    <HStack gap={3} align="center" w="full" minW={0} py={2} px={chevron ? 3 : 0}>
      <Skeleton boxSize="40px" borderRadius="md" flexShrink={0} />
      <Stack gap={1.5} flex="1" minW={0}>
        <SkeletonText noOfLines={1} w="65%" />
        <Skeleton h="3" w="25%" />
        <Skeleton h="3.5" w="45%" />
      </Stack>
      {chevron && <Box w="16px" flexShrink={0} />}
    </HStack>
  );
}
