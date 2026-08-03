import { useEffect, useRef, useState } from "react";
import {
  Button,
  Dialog,
  Flex,
  IconButton,
  Input,
  Portal,
  Spinner,
  Stack,
  Text,
} from "@chakra-ui/react";
import { Boxes, Check, ChevronDown, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Batch } from "../gen/inventory_iface/v1/batch_pb";
import { searchBatches } from "../queries/batches";
import ProductImage from "./ProductImage";

type Props = {
  // Scope availability + the in-stock filter to this warehouse (e.g. the
  // transfer source). Empty falls back to the caller's active warehouse.
  warehouseId?: string;
  value?: string;
  onChange?: (id: string) => void;
  // Fires with the full picked batch — lets callers capture the product name +
  // available qty without a second lookup.
  onSelectItem?: (batch: Batch) => void;
  // Hide already-picked batch ids from the result list.
  excludeIds?: readonly string[];
  // Only show batches with stock in the scoped warehouse (default true).
  // NOTE searchBatches defaults this to FALSE — a ledger surface (movements
  // filter, a positive ADJUSTMENT) must pass false to keep spent lots reachable.
  onlyInStock?: boolean;
  // Scope results to one product's lots (the stocktake add-batch dialog pairs
  // this with its own product filter).
  productId?: string;
  // Render an inline × that clears the selection. Off by default: a required
  // line field (Transfers) must not offer "none", but a filter has to.
  clearable?: boolean;
  placeholder?: string;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  disabled?: boolean;
  // Chip label when the selected value isn't in the currently-loaded list.
  selectedLabel?: string;
};

const DEBOUNCE_MS = 250;

/** The lot's own identity: its batch number, or a short id when it has none. */
function lotOf(b: Batch): string {
  return b.batchNumber || b.id.slice(0, 8);
}

/**
 * Trigger/row label. A missing product name falls back to the lot rather than a
 * dash — a placeholder-looking "—" reads as "nothing selected" on a control that
 * has, in fact, a selection.
 */
function batchLabel(b: Batch): string {
  return b.productName ? `${b.productName} · ${lotOf(b)}` : lotOf(b);
}

// Reusable batch picker rendered as a searchable modal popup (mirrors
// WarehouseSelect). Result rows are two-line and human-readable — product name
// on top, batch no · expiry · available qty below — so staff never has to
// memorize batch numbers. Backend-searched via searchBatches, scoped to
// `warehouseId` so availability reflects the right location.
export default function BatchSelect({
  warehouseId,
  value = "",
  onChange,
  onSelectItem,
  excludeIds,
  onlyInStock = true,
  productId,
  clearable = false,
  placeholder,
  size = "sm",
  width = "100%",
  disabled,
  selectedLabel,
}: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const [items, setItems] = useState<Batch[]>([]);
  const [isFetching, setIsFetching] = useState(false);
  const latestQueryRef = useRef("");
  // The picked row is remembered separately from `items`, which close() clears:
  // the trigger has to keep showing the thumbnail + label after the dialog is
  // gone, and re-deriving it from `items` would blank out on close.
  const [picked, setPicked] = useState<Batch | null>(null);

  // Debounced backend search; a ref guard discards stale responses if a newer
  // query is in flight. Empty query fires immediately (top in-stock lots).
  useEffect(() => {
    if (!open) return;
    const trimmed = q.trim();
    latestQueryRef.current = trimmed;
    const handle = setTimeout(
      async () => {
        setIsFetching(true);
        try {
          const res = await searchBatches(trimmed, {
            warehouseId,
            onlyInStock,
            productId,
          });
          if (latestQueryRef.current === trimmed) setItems(res);
        } finally {
          if (latestQueryRef.current === trimmed) setIsFetching(false);
        }
      },
      q === "" ? 0 : DEBOUNCE_MS,
    );
    return () => clearTimeout(handle);
  }, [q, open, warehouseId, onlyInStock, productId]);

  const exclude = new Set(excludeIds ?? []);
  const options = items.filter((b) => b.id === value || !exclude.has(b.id));

  // Prefer the row that's actually loaded; fall back to the remembered pick so
  // the trigger survives close(). A `value` set from outside (edit drawer, URL)
  // that was never picked in-session has no row to preview — that's what the
  // `selectedLabel` escape hatch is for.
  const selectedRow =
    items.find((b) => b.id === value) ?? (picked?.id === value ? picked : null);
  const title = placeholder ?? t("transfers.pickBatch");
  const label = selectedRow
    ? batchLabel(selectedRow)
    : (selectedLabel ?? title);

  const close = () => {
    setOpen(false);
    setQ("");
    setItems([]);
  };

  return (
    <>
      {/* The × is a SIBLING of the trigger, not nested inside it: a button
          inside a button is invalid markup and swallows the outer click. */}
      <Flex align="center" gap={1} width={width}>
        <Button
          type="button"
          variant="outline"
          size={size}
          flex="1"
          minW={0}
          disabled={disabled}
          justifyContent="space-between"
          fontWeight="normal"
          onClick={() => setOpen(true)}
        >
          <Flex align="center" gap={2} minW={0}>
            {/* Preview the picked lot's product; the Boxes glyph stands in for
              "a batch goes here" while empty, matching SupplierSelect's slot. */}
            {selectedRow ? (
              <ProductImage
                productId={selectedRow.productId}
                name={selectedRow.productName}
                version={Number(selectedRow.productImageUpdatedAt)}
                size={20}
              />
            ) : (
              <Boxes size={14} />
            )}
            <Text
              truncate
              color={selectedRow || selectedLabel ? "fg" : "fg.muted"}
            >
              {label}
            </Text>
          </Flex>
          <ChevronDown size={16} />
        </Button>
        {clearable && !!value && !disabled && (
          <IconButton
            aria-label={t("common.clear")}
            variant="ghost"
            size={size}
            onClick={() => {
              setPicked(null);
              onChange?.("");
            }}
          >
            <X size={14} />
          </IconButton>
        )}
      </Flex>

      <Dialog.Root open={open} onOpenChange={(d) => !d.open && close()}>
        <Portal>
          <Dialog.Backdrop />
          <Dialog.Positioner>
            <Dialog.Content>
              <Dialog.Header>
                <Dialog.Title>{title}</Dialog.Title>
                <Dialog.CloseTrigger asChild>
                  <IconButton aria-label="close" variant="ghost" size="sm">
                    <X size={16} />
                  </IconButton>
                </Dialog.CloseTrigger>
              </Dialog.Header>
              <Dialog.Body>
                <Stack gap={3}>
                  <Flex align="center" gap={2}>
                    <Input
                      placeholder={t("common.search")}
                      value={q}
                      onChange={(e) => setQ(e.target.value)}
                      autoFocus
                    />
                    {isFetching && <Spinner size="xs" />}
                  </Flex>
                  <Stack gap={1} maxH="360px" overflowY="auto">
                    {options.map((b) => (
                      <Flex
                        key={b.id}
                        px={3}
                        py={2}
                        borderRadius="md"
                        _hover={{ bg: "bg.muted" }}
                        cursor="pointer"
                        justify="space-between"
                        align="center"
                        gap={3}
                        bg={b.id === value ? "bg.muted" : undefined}
                        onClick={() => {
                          setPicked(b);
                          onChange?.(b.id);
                          onSelectItem?.(b);
                          close();
                        }}
                      >
                        <Flex align="center" gap={3} minW={0}>
                          <ProductImage
                            productId={b.productId}
                            name={b.productName}
                            version={Number(b.productImageUpdatedAt)}
                            size={32}
                          />
                          <Stack gap={0} minW={0}>
                            <Text fontSize="sm" fontWeight="medium" truncate>
                              {b.productName || lotOf(b)}
                            </Text>
                            <Text fontSize="xs" color="fg.muted" truncate>
                              {lotOf(b)}
                              {b.expiryDate
                                ? ` · ${t("transfers.expShort")} ${b.expiryDate}`
                                : ""}
                            </Text>
                          </Stack>
                        </Flex>
                        <Flex align="center" gap={2} flexShrink={0}>
                          <Text
                            fontSize="xs"
                            color="fg.muted"
                            whiteSpace="nowrap"
                          >
                            {String(b.currentQuantity)}{" "}
                            {t("transfers.available")}
                          </Text>
                          {b.id === value && <Check size={14} />}
                        </Flex>
                      </Flex>
                    ))}
                    {options.length === 0 && !isFetching && (
                      <Text
                        color="fg.muted"
                        fontSize="sm"
                        textAlign="center"
                        py={4}
                      >
                        {t("common.noResults")}
                      </Text>
                    )}
                  </Stack>
                </Stack>
              </Dialog.Body>
            </Dialog.Content>
          </Dialog.Positioner>
        </Portal>
      </Dialog.Root>
    </>
  );
}
