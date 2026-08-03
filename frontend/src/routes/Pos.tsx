import { useCallback, useEffect, useRef, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  IconButton,
  Input,
  RadioGroup,
  Stack,
  Switch,
  Text,
} from "@chakra-ui/react";
import { Lock, LogOut, Minus, Plus, Search, Trash2, Warehouse as WarehouseIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import DiscountField, { type DiscountType } from "../components/DiscountField";
import EnumSelect from "../components/EnumSelect";
import MoneyInput from "../components/MoneyInput";
import NumberInput from "../components/NumberInput";
import PrinterSelect from "../components/PrinterSelect";
import ProductImage from "../components/ProductImage";
import WarehouseSelect from "../components/WarehouseSelect";
import type { Product } from "../gen/inventory_iface/v1/product_pb";
import { PaymentSource, Sale, SaleStatus, type SaleItem } from "../gen/pos_iface/v1/sale_pb";
import { saleClient } from "../lib/clients";
import { formatMoney } from "../lib/format";
import { releaseModalBodyLock } from "../lib/modalLock";
import { toast } from "../lib/toaster";
import { useAuth } from "../lib/auth";
import { saveResepRoundTrip, takeResepRoundTrip } from "../lib/posStorage";
import { nextTier } from "../queries/productPriceTiers";
import { useBusinessMode } from "../queries/settings";
import {
  CustomerPickerDialog,
  PrescriptionPickerDialog,
  ReceiptDialog,
} from "./pos/posDialogs";
import {
  CustomerBar,
  LineDiscountPopover,
  PrescriptionBar,
  QuickAmountRow,
} from "./pos/posControls";
import { usePosPrinter } from "./pos/usePosPrinter";
import { usePosSearch } from "./pos/usePosSearch";
import { usePosWarehouseGate } from "./pos/usePosWarehouseGate";
import {
  useAddItemMutation,
  useAttachPrescriptionMutation,
  useCompleteSaleMutation,
  useDetachPrescriptionMutation,
  useClearLineDiscountMutation,
  useRemoveItemMutation,
  useSetCartDiscountMutation,
  useSetItemQuantityMutation,
  useSetLineDiscountMutation,
  useSetSaleCustomerMutation,
  useSetServiceFeeMutation,
  useStartSaleMutation,
} from "../queries/sales";

// Cart/preference persistence lives in lib/posStorage.ts; the receipt-printer
// target (POS_PRINTER_KEY / decodePrinter) is shared with order-history reprint
// — see lib/printerTarget.ts.

export default function Pos() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { user } = useAuth();

  const { isPharmacy } = useBusinessMode();

  // Sale lifecycle ----
  const startSale = useStartSaleMutation();
  const addItem = useAddItemMutation();
  const setQty = useSetItemQuantityMutation();
  const removeItem = useRemoveItemMutation();
  const setSaleCustomer = useSetSaleCustomerMutation();
  const attachPrescription = useAttachPrescriptionMutation();
  const detachPrescription = useDetachPrescriptionMutation();
  const setServiceFee = useSetServiceFeeMutation();
  const setLineDiscount = useSetLineDiscountMutation();
  const clearLineDiscount = useClearLineDiscountMutation();
  const setCartDiscount = useSetCartDiscountMutation();
  const completeSale = useCompleteSaleMutation();

  const [sale, setSale] = useState<Sale | null>(null);
  const [completedSale, setCompletedSale] = useState<Sale | null>(null);
  const [customerOpen, setCustomerOpen] = useState(false);
  // Pharmacy resep flow: the picker, plus a product whose add was deferred until
  // a covering prescription is attached (the "click Rx-required product" path).
  const [prescriptionOpen, setPrescriptionOpen] = useState(false);
  const [deferred, setDeferred] = useState<{ product: Product; unitId: string } | null>(null);
  const [paymentSource, setPaymentSource] = useState<PaymentSource>(
    PaymentSource.CASH,
  );
  const [paidAmount, setPaidAmount] = useState("0");
  // Biaya jasa (service fee) — defaults from the attached resep, editable here.
  // Local input state committed on blur via SetServiceFee (avoids an RPC per
  // keystroke); kept in sync whenever the sale's fee changes (e.g. attach).
  const [feeInput, setFeeInput] = useState("0");
  // Cart-level discount — Rp/% toggle + value, committed on blur / type change
  // via SetCartDiscount. value is human (minor units for FIXED, plain % for
  // PERCENT); kept in sync with the sale (the backend re-resolves PERCENT).
  const [cartDisc, setCartDisc] = useState<{ type: DiscountType; value: number }>({
    type: "FIXED",
    value: 0,
  });

  const ensureSale = useCallback(async (): Promise<Sale | null> => {
    if (sale) return sale;
    try {
      const res = await startSale.mutateAsync();
      if (res.sale) setSale(res.sale);
      return res.sale ?? null;
    } catch {
      return null;
    }
  }, [sale, startSale]);

  // Drop the in-progress DRAFT cart. Stock is per-warehouse, so the cart can't
  // survive a warehouse switch; the next add lazily starts a fresh draft stamped
  // with the new warehouse. Best-effort: raw client call, errors swallowed.
  const discardActiveSale = useCallback(async () => {
    if (!sale) return;
    await saleClient.discardSale({ saleId: sale.id }).catch(() => {});
    setSale(null);
  }, [sale]);

  // Discard an abandoned cart when leaving POS. The active `sale` is always a
  // DRAFT (doComplete nulls it on completion), so deleting it on unmount cleans
  // up in-progress carts that never completed — they vanish entirely (no VOIDED
  // trace, never reach order history). Best-effort: raw client call with errors
  // swallowed (no global error toast). saleRef mirrors `sale` so the mount-once
  // cleanup reads the latest value.
  const saleRef = useRef<Sale | null>(null);
  // Set true before an intentional create-resep navigation so the unmount
  // cleanup keeps the persisted DRAFT instead of discarding it.
  const keepDraftRef = useRef(false);
  useEffect(() => {
    saleRef.current = sale;
  }, [sale]);
  useEffect(() => {
    return () => {
      if (keepDraftRef.current) return; // intentional create-resep nav — keep the cart
      const s = saleRef.current;
      if (s && s.status === SaleStatus.DRAFT) {
        void saleClient.discardSale({ saleId: s.id }).catch(() => {});
      }
    };
  }, []);

  // Persist the cart + the pending Rx-required product, then route to the
  // full create-resep page. On return (?attachRx=<id>) the mount effect below
  // restores the draft and attaches the new resep. Navigation is DEFERRED until
  // after the picker dialog closes (see the effect below): navigating while a
  // Chakra Dialog is open leaves the body scroll-lock / inert state applied,
  // which freezes the whole destination page (nothing clickable/typable).
  const [pendingResepNav, setPendingResepNav] = useState<string | null>(null);
  const goCreateResep = useCallback(() => {
    const s = saleRef.current;
    saveResepRoundTrip(
      s?.id ?? null,
      deferred ? { productId: deferred.product.id, unitId: deferred.unitId } : null,
    );
    keepDraftRef.current = true;
    const patient = s?.customerId ?? "";
    setPrescriptionOpen(false); // close the picker first so it unmounts + releases the body lock
    setPendingResepNav(
      `/prescriptions/new?returnTo=pos${patient ? `&patient=${patient}` : ""}`,
    );
  }, [deferred]);

  // Once the picker dialog has closed (committed), navigate on the next frame.
  // Chakra/Ark applies a body lock (pointer-events:none + overflow:hidden) and
  // sets aria-hidden on #root while a modal Dialog is open, and only restores
  // them on a normal close transition — NOT when the Dialog unmounts because we
  // navigated away. That leaves the destination page completely frozen. We've
  // closed the picker (so it's unmounted by now) but must release the residual
  // body lock ourselves before routing to the create-resep page.
  useEffect(() => {
    if (pendingResepNav == null) return;
    const id = requestAnimationFrame(() => {
      releaseModalBodyLock();
      navigate(pendingResepNav);
    });
    return () => cancelAnimationFrame(id);
  }, [pendingResepNav, navigate]);

  // Warehouse gate + in-place switcher, and the receipt-printer picker. Both
  // are self-contained slices — see routes/pos/.
  const {
    gateDone,
    warehouses,
    currentWarehouse,
    activeWarehouseName,
    confirmWarehouse,
    switchWarehouse,
  } = usePosWarehouseGate({ discardActiveSale });
  const { connectors, hasPrinters, printerValue, onPickPrinter, printerTarget } =
    usePosPrinter();

  // Mount: restore a preserved DRAFT cart if we're returning from the
  // create-resep page (?attachRx=<id>), otherwise start a fresh draft. When an
  // Rx was just created we attach it and re-add the deferred Rx-required product
  // — a single round-trip lands the resep + the item that needed it.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;

    const attachRx = searchParams.get("attachRx") ?? "";
    const { saleId: persistedSaleId, deferred: deferredInfo } = takeResepRoundTrip();

    void (async () => {
      let restored: Sale | null = null;
      if (persistedSaleId) {
        try {
          const res = await saleClient.getSale({ id: persistedSaleId });
          if (res.sale && res.sale.status === SaleStatus.DRAFT) {
            restored = res.sale;
            setSale(res.sale);
          }
        } catch {
          /* draft gone — fall back to a fresh start */
        }
      }
      if (!restored) {
        await ensureSale();
        return;
      }
      if (attachRx) {
        try {
          const attRes = await attachPrescription.mutateAsync({
            saleId: restored.id,
            prescriptionId: attachRx,
          });
          let cur = attRes.sale ?? restored;
          if (deferredInfo?.productId) {
            const addRes = await addItem.mutateAsync({
              saleId: cur.id,
              productId: deferredInfo.productId,
              productUnitId: deferredInfo.unitId ?? "",
              qty: 1,
            });
            cur = addRes.sale ?? cur;
          }
          setSale(cur);
        } catch {
          /* toast handled globally */
        }
        // Strip ?attachRx so a manual refresh doesn't re-trigger the attach.
        setSearchParams({}, { replace: true });
      }
    })();
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

  const onAdd = useCallback(
    async (product: Product, unitId: string, available?: number) => {
      const s = await ensureSale();
      if (!s) return;
      // Pharmacy gate: an Rx-required product can't be added until a covering
      // prescription is attached. Defer the add and open the picker instead of
      // letting the backend reject it.
      if (isPharmacy && product.prescriptionRequired && !s.prescriptionId) {
        setDeferred({ product, unitId });
        setPrescriptionOpen(true);
        return;
      }
      const enough =
        available !== undefined
          ? available >= 1
          : Number(stockByProduct.get(product.id) ?? 0n) > 0;
      if (!enough) {
        toast.error(t("pos.outOfStock"));
        return;
      }
      try {
        const res = await addItem.mutateAsync({
          saleId: s.id,
          productId: product.id,
          productUnitId: unitId,
          qty: 1,
        });
        if (res.sale) setSale(res.sale);
        resetAfterAdd();
      } catch {
        /* toast handled globally */
      }
    },
    [ensureSale, addItem, stockByProduct, isPharmacy, t],
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
        setDeferred(null);
        setPrescriptionOpen(true);
      } else if (e.key === "F8") {
        e.preventDefault();
        void doComplete();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sale, paymentSource, paidAmount, isPharmacy]);

  // Cart ops ----
  const onChangeQty = async (itemId: string, qty: number) => {
    if (!sale) return;
    if (qty <= 0) {
      try {
        const res = await removeItem.mutateAsync({ saleId: sale.id, itemId });
        if (res.sale) setSale(res.sale);
      } catch {
        /* toast handled globally */
      }
      return;
    }
    try {
      const res = await setQty.mutateAsync({ saleId: sale.id, itemId, qty });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const onRemove = async (itemId: string) => {
    if (!sale) return;
    try {
      const res = await removeItem.mutateAsync({ saleId: sale.id, itemId });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  // Switch a cart line's selling unit (box/strip/tablet): remove + re-add at the
  // new unit, keeping the same numeric qty.
  const onChangeUnit = async (item: SaleItem, unitId: string) => {
    if (!sale || unitId === item.productUnitId) return;
    try {
      await removeItem.mutateAsync({ saleId: sale.id, itemId: item.id });
      const res = await addItem.mutateAsync({
        saleId: sale.id,
        productId: item.productId,
        productUnitId: unitId,
        qty: item.qty,
      });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const onAttachCustomer = async (customerId: string) => {
    if (!sale) return;
    try {
      const res = await setSaleCustomer.mutateAsync({
        saleId: sale.id,
        customerId,
      });
      if (res.sale) setSale(res.sale);
      setCustomerOpen(false);
    } catch {
      /* toast handled globally */
    }
  };

  const onClearCustomer = async () => {
    if (!sale) return;
    try {
      const res = await setSaleCustomer.mutateAsync({
        saleId: sale.id,
        customerId: "",
      });
      if (res.sale) setSale(res.sale);
    } catch {
      /* */
    }
  };

  // Attach a prescription, then (if a product add was deferred pending the Rx)
  // add that product immediately — a single user gesture lands both.
  const onAttachPrescription = async (prescriptionId: string) => {
    const s = await ensureSale();
    if (!s) return;
    try {
      const res = await attachPrescription.mutateAsync({
        saleId: s.id,
        prescriptionId,
      });
      if (res.sale) setSale(res.sale);
      setPrescriptionOpen(false);
      if (deferred && res.sale) {
        const d = deferred;
        setDeferred(null);
        try {
          const addRes = await addItem.mutateAsync({
            saleId: res.sale.id,
            productId: d.product.id,
            productUnitId: d.unitId,
            qty: 1,
          });
          if (addRes.sale) setSale(addRes.sale);
          resetAfterAdd();
        } catch {
          /* backend coverage toast */
        }
      }
    } catch {
      /* toast handled globally */
    }
  };

  const onDetachPrescription = async () => {
    if (!sale) return;
    try {
      const res = await detachPrescription.mutateAsync({ saleId: sale.id });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally (blocked if Rx items still in cart) */
    }
  };

  // Keep the fee input mirrored to the sale (attach snapshots the resep's fee).
  useEffect(() => {
    setFeeInput(String(Number(sale?.biayaJasa ?? 0n)));
  }, [sale?.biayaJasa]);

  const commitFee = async () => {
    if (!sale) return;
    const v = Math.max(0, Number(feeInput || "0") || 0);
    if (v === Number(sale.biayaJasa)) return;
    try {
      const res = await setServiceFee.mutateAsync({ saleId: sale.id, biayaJasa: BigInt(v) });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  // Keep the cart-discount inputs mirrored to the sale. The sale stores the raw
  // value (basis points when PERCENT); convert back to human percent here.
  useEffect(() => {
    const type = (sale?.cartDiscountType || "FIXED") as DiscountType;
    const raw = Number(sale?.cartDiscountValue ?? 0n);
    setCartDisc({ type, value: type === "PERCENT" ? raw / 100 : raw });
  }, [sale?.id, sale?.cartDiscountType, sale?.cartDiscountValue]);

  const commitCartDiscount = async (type: DiscountType, human: number) => {
    if (!sale) return;
    // PERCENT goes to the backend as basis points (×100); FIXED as minor units.
    const wire = type === "PERCENT" ? Math.round(human * 100) : Math.round(human);
    if (type === (sale.cartDiscountType || "FIXED") && wire === Number(sale.cartDiscountValue)) {
      return;
    }
    try {
      const res = await setCartDiscount.mutateAsync({
        saleId: sale.id,
        discountType: type,
        discountValue: BigInt(Math.max(0, wire)),
      });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const commitLineDiscount = async (itemId: string, type: DiscountType, human: number) => {
    if (!sale) return;
    const wire = type === "PERCENT" ? Math.round(human * 100) : Math.round(human);
    try {
      const res = await setLineDiscount.mutateAsync({
        saleId: sale.id,
        itemId,
        discountType: type,
        discountValue: BigInt(Math.max(0, wire)),
      });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  // Clear a manual line discount → revert to the auto product discount (if any).
  const commitClearLineDiscount = async (itemId: string) => {
    if (!sale) return;
    try {
      const res = await clearLineDiscount.mutateAsync({ saleId: sale.id, itemId });
      if (res.sale) setSale(res.sale);
    } catch {
      /* toast handled globally */
    }
  };

  const total = Number(sale?.total ?? 0n);
  const paidNum = Number(paidAmount || "0") || 0;
  const change = paidNum - total;
  const canComplete =
    !!sale &&
    sale.items.length > 0 &&
    (paymentSource !== PaymentSource.CASH || paidNum >= total);

  const doComplete = useCallback(async () => {
    if (!canComplete || !sale) return;
    try {
      const res = await completeSale.mutateAsync({
        saleId: sale.id,
        paymentSource,
        paidAmount: BigInt(paidNum),
      });
      if (res.sale) setCompletedSale(res.sale);
      setSale(null);
      setQuery("");
      setPaidAmount("0");
      setPaymentSource(PaymentSource.CASH);
    } catch {
      /* toast handled globally */
    }
  }, [canComplete, sale, completeSale, paymentSource, paidNum]);

  const onCloseReceipt = async () => {
    setCompletedSale(null);
    await ensureSale();
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
        <Box flex="3" borderRightWidth="1px" overflowY="auto" p={4}>
          <Box position="relative" mb={3}>
            <Box position="absolute" left={3} top="50%" transform="translateY(-50%)" color="fg.muted">
              <Search size={16} />
            </Box>
            <Input
              ref={searchRef}
              pl={10}
              placeholder={t("pos.searchPlaceholder")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={onSearchKeyDown}
              autoFocus
            />
          </Box>
          <Switch.Root
            size="sm"
            mb={3}
            checked={showOutOfStock}
            onCheckedChange={(d) => onToggleOutOfStock(d.checked)}
          >
            <Switch.HiddenInput />
            <Switch.Control />
            <Switch.Label fontSize="sm" color="fg.muted">
              {t("pos.showOutOfStock")}
            </Switch.Label>
          </Switch.Root>
          <Stack gap={1}>
            {unitRows.map((row, i) => {
              const { med: m, unit, available, tiers } = row;
              const out = available < 1;
              const active = i === highlight;
              // Pharmacy cue: Rx-required product with no covering Rx yet — dimmed
              // + lock, but still clickable (the click opens the Rx picker).
              const needsRx = isPharmacy && m.prescriptionRequired && !sale?.prescriptionId;
              return (
                <Flex
                  key={`${m.id}:${unit.id}`}
                  px={3}
                  py={2}
                  borderRadius="md"
                  bg={active ? "bg.muted" : "transparent"}
                  borderWidth="1px"
                  borderColor={active ? "border" : "transparent"}
                  align="center"
                  justify="space-between"
                  cursor={out ? "not-allowed" : "pointer"}
                  opacity={out ? 0.5 : needsRx ? 0.7 : 1}
                  onMouseEnter={() => setHighlight(i)}
                  onClick={() => !out && onAdd(m, unit.id, available)}
                >
                  {/* Thumbnail first: a cashier scans this list by sight, and
                      the picture is the fastest thing to match against. THUMB
                      rendition; products without one cost zero requests. */}
                  <ProductImage
                    productId={m.id}
                    name={m.name}
                    version={Number(m.imageUpdatedAt)}
                    size={36}
                  />
                  <Stack gap={0} flex="1" ml={3}>
                    <HStack gap={2}>
                      {needsRx && <Lock size={12} />}
                      <Text fontSize="sm" fontWeight="medium">
                        {m.name}
                      </Text>
                      <Text fontSize="xs" color="fg.muted">
                        · {unit.name}
                      </Text>
                      {needsRx && (
                        <Text fontSize="xs" color="orange.fg">
                          {t("pos.needsRx")}
                        </Text>
                      )}
                    </HStack>
                    <Text fontSize="xs" color="fg.muted">
                      {m.sku} · {available} {unit.name}
                    </Text>
                  </Stack>
                  <Stack gap={0} align="flex-end">
                    <Text fontSize="sm" fontFamily="mono">
                      {formatMoney(unit.sellPrice)}
                    </Text>
                    {/* Grosir hint: only the FIRST rung — the one crossed most
                        often — plus a +N counter. The full ladder is back-office
                        detail and would be noise in a 40-row scan list. */}
                    {tiers.length > 0 && (
                      <HStack gap={1}>
                        <Badge size="xs" variant="subtle" colorPalette="purple">
                          {t("pos.grosir")}
                        </Badge>
                        <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
                          {tiers.length > 1
                            ? t("pos.grosirTierHintMore", {
                                qty: tiers[0].minQty,
                                price: formatMoney(tiers[0].price),
                                n: tiers.length - 1,
                              })
                            : t("pos.grosirTierHint", {
                                qty: tiers[0].minQty,
                                price: formatMoney(tiers[0].price),
                              })}
                        </Text>
                      </HStack>
                    )}
                  </Stack>
                </Flex>
              );
            })}
            {unitRows.length === 0 && (
              <Text color="fg.muted" fontSize="sm" textAlign="center" py={6}>
                {/* With the filter on, an empty list means "nothing in stock
                    matches", not "no such product" — say so, or the cashier
                    concludes the product isn't in the catalog at all. */}
                {showOutOfStock
                  ? t("common.noResults")
                  : t("pos.noResultsHiddenOutOfStock")}
              </Text>
            )}
          </Stack>
        </Box>

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
              onClear={onClearCustomer}
            />
            {isPharmacy && (
              <PrescriptionBar
                sale={sale}
                onAttach={() => {
                  setDeferred(null);
                  setPrescriptionOpen(true);
                }}
                onDetach={onDetachPrescription}
              />
            )}
          </Box>

          <Box flex="1" overflowY="auto" px={4} py={2}>
            {(sale?.items.length ?? 0) === 0 && (
              <Text color="fg.muted" fontSize="sm" textAlign="center" py={8}>
                {t("pos.empty")}
              </Text>
            )}
            <Stack gap={2}>
              {sale?.items.map((it) => {
                const med = products.find((m) => m.id === it.productId);
                return (
                  <Flex key={it.id} align="center" gap={2}>
                    <Stack gap={0} flex="1">
                      <Text fontSize="sm" fontWeight="medium">
                        {med?.name ?? it.productId.slice(0, 8)}
                      </Text>
                      {/* Grosir: struck-through normal price + the rung earned,
                          right beside the qty input the cashier is looking at.
                          All values are server fields — no client price math. */}
                      <HStack gap={1.5}>
                        {it.tierMinQty > 0 && it.listPriceSnapshot > it.unitPriceSnapshot && (
                          <Text
                            fontSize="2xs"
                            color="fg.muted"
                            fontFamily="mono"
                            textDecoration="line-through"
                          >
                            {formatMoney(it.listPriceSnapshot)}
                          </Text>
                        )}
                        <Text fontSize="xs" color="fg.muted" fontFamily="mono">
                          {formatMoney(it.unitPriceSnapshot)}
                        </Text>
                        {it.tierMinQty > 0 && (
                          <Text fontSize="2xs" color="purple.fg" fontFamily="mono">
                            {t("pos.grosirThreshold", { qty: it.tierMinQty })}
                          </Text>
                        )}
                      </HStack>
                      {/* "Buy N more" nudge, deliberately narrow: only when the
                          gap is a plausible ask, and never a click target — it's
                          for the cashier to relay, not to bump the qty. */}
                      {(() => {
                        const unit = med?.units.find((u) => u.id === it.productUnitId);
                        if (!unit) return null;
                        const nt = nextTier(med?.priceTiers, it.productUnitId, it.qty, unit.sellPrice);
                        if (!nt) return null;
                        const remaining = nt.minQty - it.qty;
                        if (remaining <= 0 || remaining > Math.max(2, Math.ceil(nt.minQty * 0.2))) {
                          return null;
                        }
                        return (
                          <Text fontSize="2xs" color="orange.fg">
                            {t("pos.grosirNudge", { n: remaining, price: formatMoney(nt.price) })}
                          </Text>
                        );
                      })()}
                    </Stack>
                    <IconButton
                      aria-label="decrease quantity"
                      size="xs"
                      variant="outline"
                      onClick={() => onChangeQty(it.id, it.qty - 1)}
                    >
                      <Minus size={14} />
                    </IconButton>
                    <NumberInput
                      size="sm"
                      width="48px"
                      aria-label="line quantity"
                      value={it.qty}
                      onChange={(raw) => onChangeQty(it.id, Number(raw || 0))}
                    />
                    <IconButton
                      aria-label="increase quantity"
                      size="xs"
                      variant="outline"
                      onClick={() => onChangeQty(it.id, it.qty + 1)}
                    >
                      <Plus size={14} />
                    </IconButton>
                    {med && med.units.filter((u) => u.sellable && u.active).length > 1 ? (
                      <EnumSelect
                        size="sm"
                        width="84px"
                        value={it.productUnitId}
                        onChange={(v) => onChangeUnit(it, v)}
                        items={med.units.filter((u) => u.sellable && u.active)}
                        itemToString={(u) => u.name}
                        itemToValue={(u) => u.id}
                      />
                    ) : (
                      <Text fontSize="xs" color="fg.muted" w="84px">
                        {it.unitName || med?.unit}
                      </Text>
                    )}
                    <LineDiscountPopover
                      item={it}
                      onApply={(type, human) => commitLineDiscount(it.id, type, human)}
                      onClear={() => commitClearLineDiscount(it.id)}
                    />
                    <Stack gap={0} w="80px" align="flex-end">
                      <Text fontSize="sm" fontFamily="mono">
                        {formatMoney(it.lineTotal)}
                      </Text>
                      {it.tierMinQty > 0 && (
                        <Badge size="xs" colorPalette="purple">
                          {t("pos.grosir")}
                        </Badge>
                      )}
                      {Number(it.lineDiscount) > 0 && (
                        <HStack gap={1}>
                          {/* Grosir and Promo are mutually exclusive: a tiered
                              line suppresses the auto discount, so any discount
                              shown here is the cashier's manual one. Guarded
                              explicitly rather than trusting lineDiscount === 0. */}
                          {!it.discountManual && it.tierMinQty === 0 && (
                            <Badge size="xs" colorPalette="green">
                              {t("pos.promo")}
                            </Badge>
                          )}
                          <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
                            -{formatMoney(Number(it.lineDiscount))}
                            {it.discountType === "PERCENT"
                              ? ` (${Number(it.discountValue) / 100}%)`
                              : ""}
                          </Text>
                        </HStack>
                      )}
                    </Stack>
                    <IconButton
                      aria-label="remove"
                      size="xs"
                      variant="ghost"
                      onClick={() => onRemove(it.id)}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Flex>
                );
              })}
            </Stack>
          </Box>

          {/* Totals + payment */}
          <Box borderTopWidth="1px" px={4} py={3}>
            <Stack gap={2}>
              <Flex justify="space-between">
                <Text fontSize="sm" color="fg.muted">{t("pos.subtotal")}</Text>
                <Text fontSize="sm" fontFamily="mono">
                  {formatMoney(Number(sale?.subtotal ?? 0n))}
                </Text>
              </Flex>
              <Flex justify="space-between" align="center">
                <Text fontSize="sm" color="fg.muted">{t("pos.discount")}</Text>
                <DiscountField
                  type={cartDisc.type}
                  value={cartDisc.value}
                  onChange={(type, value) => {
                    const typeChanged = type !== cartDisc.type;
                    setCartDisc({ type, value });
                    if (typeChanged) void commitCartDiscount(type, value);
                  }}
                  onBlur={() => void commitCartDiscount(cartDisc.type, cartDisc.value)}
                  valueWidth="120px"
                />
              </Flex>
              {Number(sale?.cartDiscount ?? 0n) > 0 && (
                <Flex justify="flex-end">
                  <Text fontSize="2xs" color="fg.muted" fontFamily="mono">
                    -{formatMoney(Number(sale?.cartDiscount ?? 0n))}
                  </Text>
                </Flex>
              )}
              {isPharmacy && (
                <Flex justify="space-between" align="center">
                  <Text fontSize="sm" color="fg.muted">{t("prescriptions.biayaJasa")}</Text>
                  <MoneyInput
                    size="sm"
                    width="120px"
                    value={feeInput}
                    onChange={setFeeInput}
                    onBlur={commitFee}
                    aria-label={t("prescriptions.biayaJasa")}
                  />
                </Flex>
              )}
              <Flex justify="space-between">
                <Text fontWeight="semibold">{t("pos.total")}</Text>
                <Text fontWeight="semibold" fontFamily="mono">
                  {formatMoney(total)}
                </Text>
              </Flex>

              <Box pt={2}>
                <Text fontSize="xs" color="fg.muted" mb={1}>
                  {t("pos.payment")}
                </Text>
                <RadioGroup.Root
                  value={String(paymentSource)}
                  onValueChange={(d) => setPaymentSource(Number(d.value) as PaymentSource)}
                >
                  <HStack gap={3}>
                    <RadioGroup.Item value={String(PaymentSource.CASH)}>
                      <RadioGroup.ItemHiddenInput />
                      <RadioGroup.ItemIndicator />
                      <RadioGroup.ItemText>{t("pos.paymentCash")}</RadioGroup.ItemText>
                    </RadioGroup.Item>
                    <RadioGroup.Item value={String(PaymentSource.NON_CASH)}>
                      <RadioGroup.ItemHiddenInput />
                      <RadioGroup.ItemIndicator />
                      <RadioGroup.ItemText>{t("pos.paymentNonCash")}</RadioGroup.ItemText>
                    </RadioGroup.Item>
                  </HStack>
                </RadioGroup.Root>
              </Box>

              {paymentSource === PaymentSource.CASH && (
                <Stack gap={1}>
                  <HStack>
                    <Text fontSize="xs" color="fg.muted" minW="56px">{t("pos.paid")}</Text>
                    <MoneyInput
                      size="sm"
                      value={paidAmount}
                      onChange={setPaidAmount}
                    />
                  </HStack>
                  <QuickAmountRow
                    total={total}
                    onPick={(n) => setPaidAmount(String(n))}
                  />
                  <Flex justify="space-between">
                    <Text fontSize="xs" color="fg.muted">{t("pos.change")}</Text>
                    <Text fontSize="sm" fontFamily="mono" color={change < 0 ? "fg.error" : "fg"}>
                      {formatMoney(Math.max(0, change))}
                    </Text>
                  </Flex>
                </Stack>
              )}

              <HStack gap={2} pt={2}>
                <Button
                  colorPalette="blue"
                  flex="1"
                  onClick={doComplete}
                  disabled={!canComplete}
                  loading={completeSale.isPending}
                >
                  {t("pos.complete")}
                </Button>
              </HStack>
            </Stack>
          </Box>

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
        open={prescriptionOpen}
        customerId={sale?.customerId ?? ""}
        deferredName={deferred?.product.name ?? ""}
        onClose={() => {
          setPrescriptionOpen(false);
          setDeferred(null);
        }}
        onPick={onAttachPrescription}
        onCreateNew={goCreateResep}
      />

      <ReceiptDialog
        sale={completedSale}
        onClose={onCloseReceipt}
        printerTarget={printerTarget}
      />
    </Flex>
  );
}
