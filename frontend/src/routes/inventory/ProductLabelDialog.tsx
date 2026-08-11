import {
  Alert,
  Box,
  Button,
  Field,
  HStack,
  Input,
  SegmentGroup,
  SimpleGrid,
  Stack,
  Text,
} from "@chakra-ui/react";
import { Printer } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import Barcode from "../../components/Barcode";
import EnumSelect from "../../components/EnumSelect";
import EntityDialog from "../../components/EntityDialog";
import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { isCode128Encodable } from "../../lib/barcode";
import { formatMoney } from "../../lib/format";
import { printLabelSheet } from "../../lib/labelSheet";
import { savedPrinterTarget } from "../../lib/printerTarget";
import { toast } from "../../lib/toaster";
import { usePrintProductLabelMutation } from "../../queries/products";

// Where the labels come out. "thermal" goes through PrintProductLabel on the
// backend (the shop's receipt/label printer, same dispatch as receipts);
// "sheet" never touches the server and lays labels out for an ordinary printer.
type Destination = "thermal" | "sheet";

// Matches printer.MaxLabelCopies on the backend, which clamps anything larger.
const MAX_COPIES = 100;
const MAX_COLUMNS = 6;

/** A unit the label can be priced in. `id` "" = the product's own base fields. */
type LabelUnit = { id: string; name: string; price: bigint };

/** Sellable, non-archived units — the only ones a shelf price makes sense for. */
function labelUnitsOf(product: Product): LabelUnit[] {
  const units = product.units
    .filter((u) => u.active && u.sellable)
    .map((u) => ({ id: u.id, name: u.name, price: u.sellPrice }));
  // A catalog row predating the units table still has to be labelable.
  return units.length
    ? units
    : [{ id: "", name: product.unit, price: product.unitPrice }];
}

/** Clamps a typed number into [min, max]; empty/NaN falls back to `min`. */
function clampInput(raw: string, min: number, max: number): number {
  const n = Number.parseInt(raw, 10);
  if (Number.isNaN(n)) return min;
  return Math.min(max, Math.max(min, n));
}

type Props = {
  /** The product to label, or null when the dialog is closed. */
  product: Product | null;
  onClose: () => void;
};

/**
 * Print barcode labels for a product: pick the unit whose price goes on the
 * label, how many copies, and whether they come out of the thermal printer or
 * as a sheet on an ordinary printer.
 *
 * The SKU *is* the barcode — POS scans by exact-SKU match — so a label printed
 * here is guaranteed to resolve at the till.
 */
export default function ProductLabelDialog({ product, onClose }: Props) {
  const { t } = useTranslation();
  const printMut = usePrintProductLabelMutation();

  const [destination, setDestination] = useState<Destination>("thermal");
  const [unitId, setUnitId] = useState("");
  const [copies, setCopies] = useState(1);
  const [columns, setColumns] = useState(3);

  const units = useMemo(
    () => (product ? labelUnitsOf(product) : []),
    [product],
  );

  // Re-seed on open (and when switching to a different product): the dialog
  // stays mounted while closed, so without this the previous product's unit
  // choice would carry over — the same trap useResetOnOpen exists for on forms.
  useEffect(() => {
    if (!product) return;
    const base = product.units.find((u) => u.isBase && u.active && u.sellable);
    setUnitId(base?.id ?? units[0]?.id ?? "");
    setCopies(1);
  }, [product?.id]);

  const selected = units.find((u) => u.id === unitId) ?? units[0];
  // A SKU with a non-ASCII or control byte cannot be carried by CODE128. The
  // backend refuses it too (product.sku_not_printable) — blocking here means
  // the user gets an explanation instead of a failed print.
  const printable = !!product && isCode128Encodable(product.sku);

  const onPrint = async () => {
    if (!product || !selected) return;

    if (destination === "sheet") {
      const ok = printLabelSheet(
        {
          name: product.name,
          sku: product.sku,
          unitName: selected.name,
          price: formatMoney(selected.price),
        },
        { columns, copies },
        `${product.name} — ${product.sku}`,
      );
      if (!ok) {
        toast.error(t("barcode.notEncodable"));
        return;
      }
      onClose();
      return;
    }

    // Thermal: reuse the printer POS already targets (empty target → the server
    // resolves the saved default / sole connector / TCP address).
    const target = savedPrinterTarget();
    try {
      const res = await printMut.mutateAsync({
        productId: product.id,
        productUnitId: selected.id,
        copies,
        connectorDeviceId: target.deviceId,
        printerName: target.printerName,
      });
      toast.success(t("inventory.products.label.sent", { count: res.copies }));
      onClose();
    } catch {
      /* surfaced by the global mutation toast (printing disabled / Unavailable) */
    }
  };

  return (
    <EntityDialog
      open={!!product}
      onClose={onClose}
      title={t("inventory.products.label.title")}
      size="lg"
      footer={
        <HStack justify="flex-end" gap={3}>
          <Button variant="ghost" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            colorPalette="blue"
            onClick={onPrint}
            disabled={!printable}
            loading={printMut.isPending}
          >
            <Printer size={16} />
            {t("inventory.products.label.print")}
          </Button>
        </HStack>
      }
    >
      {product && (
        <Stack gap={5}>
          {!printable && (
            <Alert.Root status="warning">
              <Alert.Indicator />
              <Alert.Content>
                <Alert.Description>
                  {t("inventory.products.label.skuNotPrintable", {
                    sku: product.sku,
                  })}
                </Alert.Description>
              </Alert.Content>
            </Alert.Root>
          )}

          <Field.Root>
            <Field.Label>{t("inventory.products.label.destination")}</Field.Label>
            {/* Two mutually exclusive targets, both always visible: a select
                would hide the choice behind a popover for no gain. */}
            <SegmentGroup.Root
              size="sm"
              value={destination}
              onValueChange={(e) =>
                setDestination((e.value as Destination) ?? "thermal")
              }
            >
              <SegmentGroup.Indicator />
              <SegmentGroup.Item value="thermal">
                <SegmentGroup.ItemText>
                  {t("inventory.products.label.thermal")}
                </SegmentGroup.ItemText>
                <SegmentGroup.ItemHiddenInput />
              </SegmentGroup.Item>
              <SegmentGroup.Item value="sheet">
                <SegmentGroup.ItemText>
                  {t("inventory.products.label.sheet")}
                </SegmentGroup.ItemText>
                <SegmentGroup.ItemHiddenInput />
              </SegmentGroup.Item>
            </SegmentGroup.Root>
            <Field.HelperText>
              {destination === "thermal"
                ? t("inventory.products.label.thermalHint")
                : t("inventory.products.label.sheetHint")}
            </Field.HelperText>
          </Field.Root>

          <SimpleGrid columns={{ base: 1, sm: destination === "sheet" ? 3 : 2 }} gap={4}>
            <Field.Root>
              <Field.Label>{t("inventory.products.label.unit")}</Field.Label>
              <EnumSelect
                value={selected?.id ?? ""}
                onChange={setUnitId}
                items={units}
                itemToValue={(u) => u.id}
                itemToString={(u) => `${u.name} · ${formatMoney(u.price)}`}
                size="sm"
              />
            </Field.Root>

            <Field.Root>
              <Field.Label>{t("inventory.products.label.copies")}</Field.Label>
              {/* Clamped on change, so an out-of-range value is unrepresentable
                  and there is no error message to translate. */}
              <Input
                type="number"
                size="sm"
                min={1}
                max={MAX_COPIES}
                value={copies}
                onChange={(e) => setCopies(clampInput(e.target.value, 1, MAX_COPIES))}
              />
            </Field.Root>

            {destination === "sheet" && (
              <Field.Root>
                <Field.Label>{t("inventory.products.label.columns")}</Field.Label>
                <Input
                  type="number"
                  size="sm"
                  min={1}
                  max={MAX_COLUMNS}
                  value={columns}
                  onChange={(e) =>
                    setColumns(clampInput(e.target.value, 1, MAX_COLUMNS))
                  }
                />
              </Field.Root>
            )}
          </SimpleGrid>

          <Box borderWidth="1px" borderRadius="md" p={4} bg="bg.subtle">
            <Text fontSize="xs" color="fg.muted" mb={3}>
              {t("inventory.products.label.preview")}
            </Text>
            <Stack align="center" gap={1}>
              <Text fontWeight="semibold" textAlign="center">
                {product.name}
              </Text>
              <Barcode value={product.sku} height={50} fontSize={13} />
              {selected && (
                <Text fontWeight="bold" fontSize="lg">
                  {formatMoney(selected.price)}
                  {selected.name ? ` / ${selected.name}` : ""}
                </Text>
              )}
            </Stack>
          </Box>
        </Stack>
      )}
    </EntityDialog>
  );
}
