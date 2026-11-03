import {
  Button,
  Drawer,
  Field,
  HStack,
  IconButton,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";

import DatePickerField from "../../components/DatePicker";
import { StockUnitOptions } from "../../components/StockUnitPopover";
import type { StockUnitGroup, StockUnitsByBase } from "../../lib/stockUnit";

type Props = {
  open: boolean;
  onClose: () => void;
  opnameBefore: string;
  onOpnameBeforeChange: (v: string) => void;
  stockUnitsByBase: StockUnitsByBase;
  onChangeStockUnit: (baseName: string, deriv: string) => void;
  stockUnitGroups: StockUnitGroup[];
};

/**
 * The Products list's filters on a phone: a bottom sheet (same shape as the
 * Menu drawer) instead of a toolbar that wrapped to three rows above the list.
 * Search stays on the page — it's the one control used on every visit.
 *
 * Changes apply live, like the md+ toolbar they mirror; "Done" only closes.
 * Satuan renders its radio groups inline (StockUnitOptions) rather than as its
 * usual popover — a popup inside a popup.
 *
 * Stays mounted and is driven by `open` (Dialog-mount HARD RULE).
 */
export default function ProductFilterSheet({
  open,
  onClose,
  opnameBefore,
  onOpnameBeforeChange,
  stockUnitsByBase,
  onChangeStockUnit,
  stockUnitGroups,
}: Props) {
  const { t } = useTranslation();

  const reset = () => {
    onOpnameBeforeChange("");
    for (const g of stockUnitGroups) {
      if (stockUnitsByBase[g.baseName]) onChangeStockUnit(g.baseName, "");
    }
  };

  return (
    <Drawer.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      placement="bottom"
    >
      <Portal>
        <Drawer.Backdrop />
        <Drawer.Positioner>
          <Drawer.Content
            maxH="85dvh"
            roundedTop="xl"
            pb="env(safe-area-inset-bottom)"
          >
            <Drawer.Header>
              <Drawer.Title>{t("filters.title")}</Drawer.Title>
            </Drawer.Header>
            <Drawer.CloseTrigger asChild top="3" insetEnd="2">
              <IconButton aria-label={t("common.close")} variant="ghost" size="sm">
                <X size={18} />
              </IconButton>
            </Drawer.CloseTrigger>
            <Drawer.Body>
              <Stack gap={6}>
                <Field.Root>
                  <Field.Label>{t("inventory.products.opnameBefore")}</Field.Label>
                  <DatePickerField
                    value={opnameBefore}
                    onChange={onOpnameBeforeChange}
                  />
                </Field.Root>
                <Stack gap={3}>
                  <Text fontSize="sm" fontWeight="medium">
                    {t("inventory.products.units")}
                  </Text>
                  <StockUnitOptions
                    byBase={stockUnitsByBase}
                    onChangeBase={onChangeStockUnit}
                    groups={stockUnitGroups}
                  />
                </Stack>
              </Stack>
            </Drawer.Body>
            <Drawer.Footer>
              <HStack w="full" justify="space-between">
                <Button variant="ghost" onClick={reset}>
                  {t("filters.reset")}
                </Button>
                <Button colorPalette="blue" onClick={onClose}>
                  {t("filters.done")}
                </Button>
              </HStack>
            </Drawer.Footer>
          </Drawer.Content>
        </Drawer.Positioner>
      </Portal>
    </Drawer.Root>
  );
}
