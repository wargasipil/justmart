import { useCallback, useEffect, useRef, useState } from "react";
import {
  Box,
  Button,
  Flex,
  HStack,
  IconButton,
  Text,
} from "@chakra-ui/react";
import { LogOut, Warehouse as WarehouseIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import PrinterSelect from "../components/PrinterSelect";
import WarehouseSelect from "../components/WarehouseSelect";
import type { Product } from "../gen/inventory_iface/v1/product_pb";
import type { Sale } from "../gen/pos_iface/v1/sale_pb";
import { toast } from "../lib/toaster";
import { useAuth } from "../lib/auth";
import { takeResepRoundTrip } from "../lib/posStorage";
import { useBusinessMode } from "../queries/settings";
import {
  CustomerPickerDialog,
  PrescriptionPickerDialog,
  ReceiptDialog,
} from "./pos/posDialogs";
import {
  CustomerBar,
  PrescriptionBar,
} from "./pos/posControls";
import { usePosPrinter } from "./pos/usePosPrinter";
import PosCartLines from "./pos/PosCartLines";
import PosPaymentPanel from "./pos/PosPaymentPanel";
import PosSearchPanel from "./pos/PosSearchPanel";
import { usePosCart } from "./pos/usePosCart";
import { usePosPayment } from "./pos/usePosPayment";
import { usePosRxFlow } from "./pos/usePosRxFlow";
import { usePosSearch } from "./pos/usePosSearch";
import { usePosWarehouseGate } from "./pos/usePosWarehouseGate";

// Cart/preference persistence lives in lib/posStorage.ts; the receipt-printer
// target (POS_PRINTER_KEY / decodePrinter) is shared with order-history reprint
// — see lib/printerTarget.ts. The DRAFT sale and every cart mutation live in
// routes/pos/usePosCart.ts (serialized, with dead-draft recovery).

export default function Pos() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { user } = useAuth();

  const { isPharmacy } = useBusinessMode();

  // Sale lifecycle: the DRAFT cart + its mutations (one request at a time).
  const cart = usePosCart();
  const { sale } = cart;
  const [completedSale, setCompletedSale] = useState<Sale | null>(null);
  const [customerOpen, setCustomerOpen] = useState(false);

  // Warehouse gate + in-place switcher, and the receipt-printer picker. Both
  // are self-contained slices — see routes/pos/. A warehouse switch discards
  // the cart (stock is per-warehouse).
  const {
    gateDone,
    warehouses,
    currentWarehouse,
    activeWarehouseName,
    confirmWarehouse,
    switchWarehouse,
  } = usePosWarehouseGate({ discardActiveSale: cart.discard });
  const { connectors, hasPrinters, printerValue, onPickPrinter, printerTarget } =
    usePosPrinter();

  // Mount: restore a preserved DRAFT cart if we're returning from the
  // create-resep page (?attachRx=<id>), otherwise start a fresh draft. When an
  // Rx was just created the cart attaches it and re-adds the deferred
  // Rx-required product — a single round-trip lands the resep + the item.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;

    const attachRx = searchParams.get("attachRx") ?? "";
    const { saleId: persistedSaleId, deferred: deferredInfo } = takeResepRoundTrip();
    void cart.restore(persistedSaleId, attachRx, deferredInfo).then((attempted) => {
      // Strip ?attachRx so a manual refresh doesn't re-trigger the attach.
      if (attempted) setSearchParams({}, { replace: true });
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Product search: query text, keyboard highlight, out-of-stock filter and the
  // derived per-unit rows. `onAdd` is passed in — the hook derives, the page adds.
  const {
    products,
    query,
    setQuery,
    highlight,
    setHighlight,
    searchRef,
    showOutOfStock,
    onToggleOutOfStock,
    unitRows,
    stockByProduct,
    onSearchKeyDown,
    resetAfterAdd,
  } = usePosSearch({ onAdd: (p, u, a) => onAdd(p, u, a) });

  // Pharmacy resep flow: the picker, plus a product whose add was deferred
  // until a covering prescription is attached (the "click Rx-required product" path).
  const rx = usePosRxFlow({ cart, onAdded: resetAfterAdd });
  const pay = usePosPayment(sale, cart);

  const { addLine } = cart;
  const { openPicker: openRxPicker } = rx;
  const onAdd = useCallback(
    async (product: Product, unitId: string, available?: number) => {
      const added = await addLine(
        { productId: product.id, productUnitId: unitId, qty: 1 },
        // Checked against the live draft (again after a dead-draft retry).
        (s) => {
          // Pharmacy gate: an Rx-required product can't be added until a
          // covering prescription is attached. Defer the add and open the
          // picker instead of letting the backend reject it.
          if (isPharmacy && product.prescriptionRequired && !s.prescriptionId) {
            openRxPicker({ product, unitId });
            return false;
          }
          const enough =
            available !== undefined
              ? available >= 1
              : Number(stockByProduct.get(product.id)?.sellable ?? 0n) > 0;
          if (!enough) {
            toast.error(t("pos.outOfStock"));
            return false;
          }
          return true;
        },
      );
      if (added) resetAfterAdd();
    },
    [addLine, openRxPicker, stockByProduct, isPharmacy, t, resetAfterAdd],
  );

  // Global keyboard shortcuts ----
  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "F2") {
        e.preventDefault();
        searchRef.current?.focus();
      } else if (e.key === "F4") {
        e.preventDefault();
        setCustomerOpen(true);
      } else if (e.key === "F5" && isPharmacy) {
        e.preventDefault();
        rx.openPicker(null);
      } else if (e.key === "F8") {
        e.preventDefault();
        void doComplete();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sale, pay.paymentSource, pay.paidAmount, isPharmacy]);

  const onAttachCustomer = async (customerId: string) => {
    if (await cart.setSaleCustomer(customerId)) setCustomerOpen(false);
  };

  const doComplete = async () => {
    if (!pay.canComplete || !sale) return;
    const done = await cart.complete(pay.paymentSource, BigInt(pay.paidNum));
    if (!done) return;
    if (done.sale) setCompletedSale(done.sale);
    setQuery("");
    pay.resetPayment();
  };

  const onCloseReceipt = async () => {
    setCompletedSale(null);
    await cart.ensureSale();
    searchRef.current?.focus();
  };

  // Warehouse gate: block POS until a selling warehouse is chosen.
  if (!gateDone) {
    return (
      <Flex direction="column" align="center" justify="center" h="100vh" bg="bg" gap={5} p={6}>
        <HStack gap={2} color="fg.muted">
          <WarehouseIcon size={20} />
          <Text fontSize="lg" fontWeight="semibold">
            {t("pos.selectWarehouse")}
          </Text>
        </HStack>
        <Text fontSize="sm" color="fg.muted">
          {t("pos.selectWarehouseHint")}
        </Text>
        <WarehouseSelect
          value=""
          onChange={confirmWarehouse}
          warehouses={warehouses}
          width="320px"
        />
        <Button variant="ghost" size="sm" onClick={() => navigate("/")}>
          <LogOut size={16} />
          {t("pos.exit")}
        </Button>
      </Flex>
    );
  }

  return (
    <Flex direction="column" h="100vh" bg="bg">
      {/* Header strip */}
      <Flex
        align="center"
        justify="space-between"
        px={4}
        h="48px"
        borderBottomWidth="1px"
      >
        <Text fontWeight="semibold">{t("pos.title")}</Text>
        <HStack gap={2}>
          {warehouses.length > 1 ? (
            <WarehouseSelect
              value={currentWarehouse}
              onChange={switchWarehouse}
              warehouses={warehouses}
              size="xs"
              width="190px"
            />
          ) : activeWarehouseName ? (
            <HStack gap={1} color="fg.muted" px={2}>
              <WarehouseIcon size={14} />
              <Text fontSize="xs">{activeWarehouseName}</Text>
            </HStack>
          ) : null}
          {hasPrinters && (
            <PrinterSelect
              connectors={connectors}
              value={printerValue}
              onChange={onPickPrinter}
              size="xs"
              width="190px"
            />
          )}
          {user && (
            <Text fontSize="sm" color="fg.muted">
              {user.name || user.email}
            </Text>
          )}
          <IconButton
            aria-label="exit"
            variant="ghost"
            size="sm"
            onClick={() => navigate("/")}
          >
            <LogOut size={16} />
          </IconButton>
        </HStack>
      </Flex>

      {/* Body */}
      <Flex flex="1" minH={0}>
        {/* Search panel */}
        <PosSearchPanel
          searchRef={searchRef}
          query={query}
          setQuery={setQuery}
          onSearchKeyDown={onSearchKeyDown}
          showOutOfStock={showOutOfStock}
          onToggleOutOfStock={onToggleOutOfStock}
          unitRows={unitRows}
          highlight={highlight}
          setHighlight={setHighlight}
          isPharmacy={isPharmacy}
          hasPrescription={!!sale?.prescriptionId}
          onAdd={onAdd}
        />

        {/* Cart panel */}
        <Flex flex="2" direction="column" minW="360px">
          <Box px={4} py={3} borderBottomWidth="1px">
            <Flex justify="space-between" align="center">
              <Text fontWeight="semibold">{t("pos.cart")}</Text>
              <Text fontSize="sm" color="fg.muted">
                {sale?.items.length ?? 0} {t("pos.items")}
              </Text>
            </Flex>
            <CustomerBar
              sale={sale}
              onAttach={() => setCustomerOpen(true)}
              onClear={() => void cart.setSaleCustomer("")}
            />
            {isPharmacy && (
              <PrescriptionBar
                sale={sale}
                onAttach={() => rx.openPicker(null)}
                onDetach={() => void cart.detachPrescription()}
              />
            )}
          </Box>

          <PosCartLines
            items={sale?.items ?? []}
            products={products}
            onChangeQty={cart.changeQty}
            onChangeUnit={cart.changeUnit}
            commitLineDiscount={(itemId, type, human) => void cart.setLineDiscount(itemId, type, human)}
            commitClearLineDiscount={(itemId) => void cart.clearLineDiscount(itemId)}
            onRemove={cart.removeLine}
          />

          {/* Totals + payment */}
          <PosPaymentPanel
            sale={sale}
            isPharmacy={isPharmacy}
            cartDisc={pay.cartDisc}
            setCartDisc={pay.setCartDisc}
            commitCartDiscount={pay.commitCartDiscount}
            feeInput={pay.feeInput}
            setFeeInput={pay.setFeeInput}
            commitFee={pay.commitFee}
            total={pay.total}
            paymentSource={pay.paymentSource}
            setPaymentSource={pay.setPaymentSource}
            paidAmount={pay.paidAmount}
            setPaidAmount={pay.setPaidAmount}
            change={pay.change}
            doComplete={doComplete}
            canComplete={pay.canComplete}
            completing={cart.completing}
          />

          <Box bg="bg.muted" px={4} py={2} borderTopWidth="1px">
            <Text fontSize="xs" color="fg.muted">
              {t("pos.shortcutHints")}
            </Text>
          </Box>
        </Flex>
      </Flex>

      <CustomerPickerDialog
        open={customerOpen}
        onClose={() => setCustomerOpen(false)}
        onPick={onAttachCustomer}
      />

      <PrescriptionPickerDialog
        open={rx.open}
        customerId={sale?.customerId ?? ""}
        deferredName={rx.deferredName}
        onClose={rx.closePicker}
        onPick={rx.onPick}
        onCreateNew={rx.goCreateResep}
      />

      <ReceiptDialog
        sale={completedSale}
        onClose={onCloseReceipt}
        printerTarget={printerTarget}
      />
    </Flex>
  );
}
