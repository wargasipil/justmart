import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import {
  Box,
  Button,
  Dialog,
  Flex,
  HStack,
  IconButton,
  Input,
  Portal,
  RadioGroup,
  Stack,
  Text,
} from "@chakra-ui/react";
import { useQueryClient } from "@tanstack/react-query";
import { FileText, Lock, LogOut, Minus, Plus, Search, Trash2, UserRound, Warehouse as WarehouseIcon, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import EnumSelect from "../components/EnumSelect";
import MoneyInput from "../components/MoneyInput";
import NumberInput from "../components/NumberInput";
import PrinterSelect from "../components/PrinterSelect";
import WarehouseSelect from "../components/WarehouseSelect";
import { Product, type ProductUnit } from "../gen/inventory_iface/v1/product_pb";
import { PaymentSource, Sale, SaleStatus, type SaleItem } from "../gen/pos_iface/v1/sale_pb";
import { Customer } from "../gen/customer_iface/v1/customer_pb";
import type { Prescription } from "../gen/prescription_iface/v1/prescription_pb";
import { saleClient } from "../lib/clients";
import { formatMoney } from "../lib/format";
import { toast } from "../lib/toaster";
import { useAuth } from "../lib/auth";
import { WAREHOUSE_KEY } from "../lib/transport";
import { useMyWarehousesQuery } from "../queries/warehouses";
import { useAllProductsQuery } from "../queries/products";
import { useStockLevelsQuery } from "../queries/stock";
import { useCustomerSearchQuery } from "../queries/customers";
import { useCustomerRefs } from "../queries/refs";
import { useConnectorsQuery } from "../queries/connectors";
import { useBusinessMode } from "../queries/settings";
import { usePrescriptionsQuery } from "../queries/prescriptions";
import {
  useAddItemMutation,
  useAttachPrescriptionMutation,
  useCompleteSaleMutation,
  useDetachPrescriptionMutation,
  usePrintReceiptMutation,
  useRemoveItemMutation,
  useSetItemQuantityMutation,
  useSetSaleCustomerMutation,
  useSetServiceFeeMutation,
  useStartSaleMutation,
} from "../queries/sales";

// localStorage keys for preserving the in-progress DRAFT cart across the
// create-resep round-trip (POS → /prescriptions/new → POS). Without this the
// unmount cleanup would hard-delete the draft. POS_DEFERRED carries the
// Rx-required product whose add was pending a covering prescription so it can
// be re-added once the new resep is attached on return.
const POS_DRAFT_KEY = "justmart_pos_draft";
const POS_DEFERRED_KEY = "justmart_pos_deferred";
// The cashier's chosen receipt printer (connector mode), persisted per device so
// it sticks across sales. Value is "<deviceId>|<printerName>", or "" for Auto.
const POS_PRINTER_KEY = "justmart_pos_printer";

// decodePrinter splits the persisted "<deviceId>|<printerName>" value. Split on
// the FIRST "|" — deviceId is a uuid (no "|"); a printer name may contain one.
function decodePrinter(v: string): { deviceId: string; printerName: string } {
  if (!v) return { deviceId: "", printerName: "" };
  const i = v.indexOf("|");
  return i < 0
    ? { deviceId: v, printerName: "" }
    : { deviceId: v.slice(0, i), printerName: v.slice(i + 1) };
}

// Release the body lock Chakra/Ark leaves behind when a modal Dialog is
// unmounted via navigation instead of a normal close (it sets pointer-events:
// none + overflow:hidden on <body> and aria-hidden on #root, and doesn't
// restore them on abrupt unmount). Without this the destination page is frozen.
function releaseModalBodyLock() {
  document.body.style.removeProperty("pointer-events");
  document.body.style.removeProperty("overflow");
  document.getElementById("root")?.removeAttribute("aria-hidden");
}

export default function Pos() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
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
    if (s) localStorage.setItem(POS_DRAFT_KEY, s.id);
    if (deferred) {
      localStorage.setItem(
        POS_DEFERRED_KEY,
        JSON.stringify({ productId: deferred.product.id, unitId: deferred.unitId }),
      );
    } else {
      localStorage.removeItem(POS_DEFERRED_KEY);
    }
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

  // --- Warehouse gate: pick the selling warehouse before POS opens ----
  // POS is full-screen (no TopBar selector), so the cashier chooses the active
  // warehouse here. Auto-skipped when they have <=1 warehouse or one is already
  // chosen. The choice drives the X-Warehouse-Id header (FEFO sells from this
  // warehouse only). "Change warehouse" clears the choice to re-open the gate.
  const myWarehousesQ = useMyWarehousesQuery();
  const [gateDone, setGateDone] = useState(false);
  const [currentWarehouse, setCurrentWarehouse] = useState<string>(
    () => localStorage.getItem(WAREHOUSE_KEY) ?? "",
  );
  const warehouses = myWarehousesQ.data?.warehouses ?? [];

  useEffect(() => {
    const data = myWarehousesQ.data;
    if (!data) return;
    if (data.warehouses.length === 0) {
      // No membership — proceed; the backend resolves the default warehouse.
      setGateDone(true);
      return;
    }
    const persisted = localStorage.getItem(WAREHOUSE_KEY);
    if (persisted && data.warehouses.some((w) => w.id === persisted)) {
      setCurrentWarehouse(persisted);
      setGateDone(true);
      return;
    }
    if (data.warehouses.length === 1) {
      localStorage.setItem(WAREHOUSE_KEY, data.warehouses[0].id);
      setCurrentWarehouse(data.warehouses[0].id);
      setGateDone(true);
    }
    // else: multiple warehouses + nothing chosen yet -> show the gate.
  }, [myWarehousesQ.data]);

  const confirmWarehouse = useCallback((id: string) => {
    const prev = localStorage.getItem(WAREHOUSE_KEY);
    localStorage.setItem(WAREHOUSE_KEY, id);
    setCurrentWarehouse(id);
    // Refetch warehouse-scoped data with the new header — no full reload.
    if (prev !== id) void queryClient.invalidateQueries();
    setGateDone(true);
  }, [queryClient]);

  const activeWarehouseName =
    warehouses.find((w) => w.id === currentWarehouse)?.name ?? "";

  // Switch the selling warehouse in place from the header picker. Stock is
  // per-warehouse, so the in-progress DRAFT cart is discarded (deleted, not
  // voided); the next add lazily starts a fresh draft stamped with the new
  // warehouse. Best-effort discard: raw client call, errors swallowed.
  const switchWarehouse = useCallback(
    async (id: string) => {
      if (id === currentWarehouse) return;
      if (sale) {
        await saleClient.discardSale({ saleId: sale.id }).catch(() => {});
        setSale(null);
      }
      localStorage.setItem(WAREHOUSE_KEY, id);
      setCurrentWarehouse(id);
      void queryClient.invalidateQueries();
    },
    [currentWarehouse, sale, queryClient],
  );

  // Receipt printer selection (connector mode). Live list of connectors+printers
  // (polls 5s); the cashier picks the print device from the header. The choice
  // persists per device and drives the receipt Print. The header picker is shown
  // only when a connector printer is available — TCP/no-connector shops never
  // see it. "" = Auto (server resolves the saved default / sole connector).
  const connectorsQ = useConnectorsQuery();
  const connectors = useMemo(() => connectorsQ.data ?? [], [connectorsQ.data]);
  const hasPrinters = connectors.some((c) => c.printerNames.length > 0);
  const [printerValue, setPrinterValue] = useState<string>(
    () => localStorage.getItem(POS_PRINTER_KEY) ?? "",
  );
  // Drop the persisted choice if that device/printer is no longer connected.
  useEffect(() => {
    if (!printerValue) return;
    const { deviceId, printerName } = decodePrinter(printerValue);
    const stillThere = connectors.some(
      (c) => c.deviceId === deviceId && c.printerNames.includes(printerName),
    );
    if (!stillThere) {
      setPrinterValue("");
      localStorage.removeItem(POS_PRINTER_KEY);
    }
  }, [connectors, printerValue]);
  const onPickPrinter = (v: string) => {
    setPrinterValue(v);
    if (v) localStorage.setItem(POS_PRINTER_KEY, v);
    else localStorage.removeItem(POS_PRINTER_KEY);
  };

  // Mount: restore a preserved DRAFT cart if we're returning from the
  // create-resep page (?attachRx=<id>), otherwise start a fresh draft. When an
  // Rx was just created we attach it and re-add the deferred Rx-required product
  // — a single round-trip lands the resep + the item that needed it.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;

    const attachRx = searchParams.get("attachRx") ?? "";
    const persistedSaleId = localStorage.getItem(POS_DRAFT_KEY);
    const deferredRaw = localStorage.getItem(POS_DEFERRED_KEY);
    localStorage.removeItem(POS_DRAFT_KEY);
    localStorage.removeItem(POS_DEFERRED_KEY);

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
        const deferredInfo = deferredRaw
          ? (JSON.parse(deferredRaw) as { productId: string; unitId: string })
          : null;
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

  // Search ----
  const productsQ = useAllProductsQuery();
  const stockQ = useStockLevelsQuery();
  const stockByProduct = useMemo(() => {
    const out = new Map<string, bigint>();
    for (const l of stockQ.data ?? []) {
      out.set(l.productId, (out.get(l.productId) ?? 0n) + l.currentQuantity);
    }
    return out;
  }, [stockQ.data]);
  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const searchRef = useRef<HTMLInputElement | null>(null);

  // Each sellable unit of each matching product is its own search row, so one
  // click adds that exact unit. `available` = how many of that unit the current
  // base stock can make (base ÷ factor).
  type UnitRow = { med: Product; unit: ProductUnit; available: number };
  const MAX_ROWS = 40;
  const unitRows = useMemo<UnitRow[]>(() => {
    const q = query.trim().toLowerCase();
    const meds = q
      ? productsQ.rows.filter((m) =>
          [m.sku, m.name].some((s) => s.toLowerCase().includes(q)),
        )
      : productsQ.rows;
    const out: UnitRow[] = [];
    for (const med of meds) {
      const base = Number(stockByProduct.get(med.id) ?? 0n);
      for (const unit of med.units.filter((u) => u.sellable && u.active)) {
        const factor = Number(unit.factor) || 1;
        out.push({ med, unit, available: Math.floor(base / factor) });
        if (out.length >= MAX_ROWS) return out;
      }
    }
    return out;
  }, [query, productsQ.rows, stockByProduct]);

  useEffect(() => {
    setHighlight(0);
  }, [query]);

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
        setQuery("");
        searchRef.current?.focus();
      } catch {
        /* toast handled globally */
      }
    },
    [ensureSale, addItem, stockByProduct, isPharmacy, t],
  );

  const onSearchKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => Math.min(unitRows.length - 1, h + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(0, h - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      // Barcode scanner: exact SKU match adds the product at its base unit.
      const skuExact = productsQ.rows.find(
        (m) => m.sku.toLowerCase() === query.trim().toLowerCase(),
      );
      if (skuExact) {
        const baseId = skuExact.units.find((u) => u.isBase)?.id ?? "";
        void onAdd(skuExact, baseId);
        return;
      }
      const row = unitRows[highlight];
      if (row) void onAdd(row.med, row.unit.id, row.available);
    } else if (e.key === "Escape") {
      setQuery("");
    }
  };

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
          setQuery("");
          searchRef.current?.focus();
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
          <Stack gap={1}>
            {unitRows.map((row, i) => {
              const { med: m, unit, available } = row;
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
                  <Stack gap={0} flex="1">
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
                  <Text fontSize="sm" fontFamily="mono">
                    {formatMoney(unit.sellPrice)}
                  </Text>
                </Flex>
              );
            })}
            {unitRows.length === 0 && (
              <Text color="fg.muted" fontSize="sm" textAlign="center" py={6}>
                {t("common.noResults")}
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
                const med = productsQ.rows.find((m) => m.id === it.productId);
                return (
                  <Flex key={it.id} align="center" gap={2}>
                    <Stack gap={0} flex="1">
                      <Text fontSize="sm" fontWeight="medium">
                        {med?.name ?? it.productId.slice(0, 8)}
                      </Text>
                      <Text fontSize="xs" color="fg.muted" fontFamily="mono">
                        {formatMoney(it.unitPriceSnapshot)}
                      </Text>
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
                    <Text fontSize="sm" fontFamily="mono" w="80px" textAlign="right">
                      {formatMoney(it.lineTotal)}
                    </Text>
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
        printerTarget={decodePrinter(printerValue)}
      />
    </Flex>
  );
}

// QuickAmountRow: one-tap fill of the paid input. Renders below the Dibayar
// field for Cash payments. Includes an "Exact" chip (paid = total), an
// optional round-up-to-next-10k chip, and standard IDR banknote denominations
// (5k/10k/20k/50k/100k) filtered to amounts >= total.
function QuickAmountRow({
  total,
  onPick,
}: {
  total: number;
  onPick: (n: number) => void;
}) {
  const { t } = useTranslation();
  if (total <= 0) return null;
  const DENOMS = [5_000, 10_000, 20_000, 50_000, 100_000];
  const above = DENOMS.filter((d) => d >= total);
  const roundedUp = Math.ceil(total / 10_000) * 10_000;
  const showRoundUp = roundedUp !== total && !above.includes(roundedUp);
  return (
    <Flex wrap="wrap" gap={1} mt={1}>
      <Button
        size="xs"
        variant="outline"
        colorPalette="blue"
        onClick={() => onPick(total)}
      >
        {t("pos.exactAmount")}
      </Button>
      {showRoundUp && (
        <Button
          size="xs"
          variant="outline"
          colorPalette="blue"
          onClick={() => onPick(roundedUp)}
        >
          {formatMoney(roundedUp)}
        </Button>
      )}
      {above.map((d) => (
        <Button
          key={d}
          size="xs"
          variant="outline"
          colorPalette="blue"
          onClick={() => onPick(d)}
        >
          {formatMoney(d)}
        </Button>
      ))}
    </Flex>
  );
}

function CustomerBar({
  sale,
  onAttach,
  onClear,
}: {
  sale: Sale | null;
  onAttach: () => void;
  onClear: () => void;
}) {
  const { t } = useTranslation();
  const customerId = sale?.customerId ?? "";
  const hasCustomer = !!customerId;
  // Resolve the attached customer's name (manual pick OR auto-filled from an
  // attached resep) so the bar shows the name, not the raw UUID.
  const refs = useCustomerRefs(useMemo(() => (customerId ? [customerId] : []), [customerId]));
  return (
    <Flex mt={2} align="center" gap={2}>
      <UserRound size={14} />
      <Text fontSize="xs" color="fg.muted" flex="1">
        {hasCustomer
          ? (refs.get(customerId)?.name ?? customerId.slice(0, 8))
          : t("pos.customer")}
      </Text>
      {hasCustomer ? (
        <Button size="xs" variant="ghost" onClick={onClear}>
          {t("pos.clearCustomer")}
        </Button>
      ) : (
        <Button size="xs" variant="ghost" onClick={onAttach}>
          {t("pos.attachCustomer")}
        </Button>
      )}
    </Flex>
  );
}

// PrescriptionBar — pharmacy mode only. Shows the attached resep (Rx number) or
// an "attach" affordance (F5). Sits under the CustomerBar in the cart panel.
function PrescriptionBar({
  sale,
  onAttach,
  onDetach,
}: {
  sale: Sale | null;
  onAttach: () => void;
  onDetach: () => void;
}) {
  const { t } = useTranslation();
  const attached = !!sale?.prescriptionId;
  return (
    <Flex mt={2} align="center" gap={2}>
      <FileText size={14} />
      <Text fontSize="xs" color="fg.muted" flex="1">
        {attached ? t("prescriptions.attached") : t("prescriptions.attach")}
      </Text>
      {attached ? (
        <Button size="xs" variant="ghost" onClick={onDetach}>
          {t("prescriptions.detach")}
        </Button>
      ) : (
        <Button size="xs" variant="ghost" onClick={onAttach}>
          {t("prescriptions.attach")}
        </Button>
      )}
    </Flex>
  );
}

// PrescriptionPickerDialog — lists ACTIVE prescriptions (scoped to the sale's
// patient when one is set) for the cashier/apoteker to attach. The backend
// enforces per-product coverage on the subsequent AddItem.
function PrescriptionPickerDialog({
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

  // NOTE: always render Dialog.Root (never `if (!open) return null`). Returning
  // null on close unmounts the dialog abruptly, which leaks Chakra/Ark's body
  // lock (pointer-events:none on <body> + aria-hidden on #root) and freezes POS.
  // Letting Dialog.Root see open=false runs Ark's proper close + restore.
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

function CustomerPickerDialog({
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

  // Always render Dialog.Root (see PrescriptionPickerDialog note) — returning
  // null on close leaks the body lock and freezes POS.
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

function ReceiptDialog({
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

  // Always render Dialog.Root (never `if (!sale) return null`) so Ark runs its
  // close + body-lock restore; content is guarded on `sale` below.
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
