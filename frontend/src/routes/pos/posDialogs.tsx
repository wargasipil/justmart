// Dialogs opened by the POS page (routes/Pos.tsx) and nowhere else. Split out of
// Pos.tsx to keep the page to its own shell + cart logic; behaviour unchanged.
//
// HARD RULE these three all follow: never `if (!open) return null`. Returning
// null unmounts an open Dialog.Root abruptly, which leaks Chakra/Ark's body lock
// (pointer-events:none + overflow:hidden on <body>, aria-hidden on #root) and
// freezes the whole page. Always render Dialog.Root and guard the CONTENT.
import { useState } from "react";
import {
  Box,
  Button,
  Dialog,
  Flex,
  IconButton,
  Input,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";
import { Plus, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { Customer } from "../../gen/customer_iface/v1/customer_pb";
import type { Prescription } from "../../gen/prescription_iface/v1/prescription_pb";
import type { Sale } from "../../gen/pos_iface/v1/sale_pb";
import { formatMoney } from "../../lib/format";
import { toast } from "../../lib/toaster";
import { useCustomerSearchQuery } from "../../queries/customers";
import { usePrescriptionsQuery } from "../../queries/prescriptions";
import { useAllProductsQuery } from "../../queries/products";
import { usePrintReceiptMutation } from "../../queries/sales";

// PrescriptionPickerDialog — lists ACTIVE prescriptions (scoped to the sale's
// patient when one is set) for the cashier/apoteker to attach. The backend
// enforces per-product coverage on the subsequent AddItem.
export function PrescriptionPickerDialog({
  open,
  customerId,
  deferredName,
  onClose,
  onPick,
  onCreateNew,
}: {
  open: boolean;
  customerId: string;
  deferredName: string;
  onClose: () => void;
  onPick: (prescriptionId: string) => void;
  onCreateNew: () => void;
}) {
  const { t } = useTranslation();
  const rxQ = usePrescriptionsQuery({ status: "ACTIVE", customerId, limit: 1000, enabled: open });
  const rows: Prescription[] = open ? rxQ.rows : [];

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("prescriptions.attach")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                {deferredName && (
                  <Text fontSize="sm" color="orange.fg">
                    {t("prescriptions.needForProduct", { product: deferredName })}
                  </Text>
                )}
                <Stack gap={1} maxH="320px" overflowY="auto">
                  {rows.map((rx) => (
                    <Flex
                      key={rx.id}
                      px={3}
                      py={2}
                      borderRadius="md"
                      _hover={{ bg: "bg.muted" }}
                      cursor="pointer"
                      justify="space-between"
                      onClick={() => onPick(rx.id)}
                    >
                      <Stack gap={0}>
                        <Text fontSize="sm" fontWeight="medium" fontFamily="mono">
                          {rx.rxNo}
                        </Text>
                        <Text fontSize="xs" color="fg.muted">
                          {rx.issuerName} · {rx.items.length} {t("prescriptions.items")}
                        </Text>
                      </Stack>
                      <Plus size={14} />
                    </Flex>
                  ))}
                  {rows.length === 0 && (
                    <Stack gap={2} py={4} align="center">
                      <Text color="fg.muted" fontSize="sm">
                        {t("prescriptions.noCoveringRx")}
                      </Text>
                    </Stack>
                  )}
                </Stack>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Button colorPalette="blue" variant="outline" onClick={onCreateNew}>
                <Plus size={14} />
                {t("prescriptions.createNew")}
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

export function CustomerPickerDialog({
  open,
  onClose,
  onPick,
}: {
  open: boolean;
  onClose: () => void;
  onPick: (customerId: string) => void;
}) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const searchQ = useCustomerSearchQuery(q, open);

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{t("pos.attachCustomer")}</Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                <Input
                  placeholder={t("customers.searchPlaceholder")}
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                  autoFocus
                />
                <Stack gap={1} maxH="320px" overflowY="auto">
                  {(searchQ.data ?? []).map((c: Customer) => (
                    <Flex
                      key={c.id}
                      px={3}
                      py={2}
                      borderRadius="md"
                      _hover={{ bg: "bg.muted" }}
                      cursor="pointer"
                      justify="space-between"
                      onClick={() => onPick(c.id)}
                    >
                      <Stack gap={0}>
                        <Text fontSize="sm" fontWeight="medium">{c.name}</Text>
                        <Text fontSize="xs" color="fg.muted">
                          {c.phone || "—"}
                        </Text>
                      </Stack>
                      <Plus size={14} />
                    </Flex>
                  ))}
                  {(searchQ.data?.length ?? 0) === 0 && (
                    <Text color="fg.muted" fontSize="sm" textAlign="center" py={4}>
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
  );
}

export function ReceiptDialog({
  sale,
  onClose,
  printerTarget,
}: {
  sale: Sale | null;
  onClose: () => void;
  // The print device chosen in the POS header (decoded). Empty → the server
  // resolves the saved default / sole connector.
  printerTarget: { deviceId: string; printerName: string };
}) {
  const { t } = useTranslation();
  const productsQ = useAllProductsQuery();
  const printMut = usePrintReceiptMutation();

  const onPrint = async () => {
    if (!sale) return;
    try {
      await printMut.mutateAsync({
        saleId: sale.id,
        connectorDeviceId: printerTarget.deviceId,
        printerName: printerTarget.printerName,
      });
      toast.success(t("pos.printSent"));
    } catch {
      /* toast handled globally */
    }
  };
  const medById = new Map(productsQ.rows.map((m) => [m.id, m]));
  return (
    <Dialog.Root open={!!sale} onOpenChange={(d) => !d.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            {sale && (
              <>
            <Dialog.Header>
              <Dialog.Title>
                {t("pos.receiptTitle")} · {sale.saleNo || sale.id.slice(0, 8)}
              </Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3} fontFamily="mono">
                <Stack gap={1}>
                  {sale.items.map((it) => (
                    <Flex key={it.id} justify="space-between" gap={2}>
                      <Text fontSize="sm" flex="1">
                        {it.qty}
                        {it.unitName ? ` ${it.unitName}` : ""}×{" "}
                        {medById.get(it.productId)?.name ?? it.productId.slice(0, 8)}
                      </Text>
                      <Text fontSize="sm">{formatMoney(it.lineTotal)}</Text>
                    </Flex>
                  ))}
                </Stack>
                <Box borderTopWidth="1px" pt={2}>
                  <Flex justify="space-between">
                    <Text fontSize="sm">{t("pos.subtotal")}</Text>
                    <Text fontSize="sm">{formatMoney(Number(sale.subtotal))}</Text>
                  </Flex>
                  {Number(sale.cartDiscount) > 0 && (
                    <Flex justify="space-between">
                      <Text fontSize="sm">{t("pos.discount")}</Text>
                      <Text fontSize="sm">-{formatMoney(Number(sale.cartDiscount))}</Text>
                    </Flex>
                  )}
                  {Number(sale.biayaJasa) > 0 && (
                    <Flex justify="space-between">
                      <Text fontSize="sm">{t("prescriptions.biayaJasa")}</Text>
                      <Text fontSize="sm">{formatMoney(Number(sale.biayaJasa))}</Text>
                    </Flex>
                  )}
                  <Flex justify="space-between">
                    <Text fontWeight="semibold">{t("pos.total")}</Text>
                    <Text fontWeight="semibold">{formatMoney(Number(sale.total))}</Text>
                  </Flex>
                  <Flex justify="space-between">
                    <Text fontSize="sm" color="fg.muted">{t("pos.paid")}</Text>
                    <Text fontSize="sm">{formatMoney(Number(sale.paidAmount))}</Text>
                  </Flex>
                  <Flex justify="space-between">
                    <Text fontSize="sm" color="fg.muted">{t("pos.change")}</Text>
                    <Text fontSize="sm">
                      {formatMoney(Math.max(0, Number(sale.paidAmount) - Number(sale.total)))}
                    </Text>
                  </Flex>
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Button variant="outline" onClick={onPrint} loading={printMut.isPending}>
                {t("pos.print")}
              </Button>
              <Button colorPalette="blue" onClick={onClose}>
                {t("pos.newSale")}
              </Button>
            </Dialog.Footer>
              </>
            )}
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
