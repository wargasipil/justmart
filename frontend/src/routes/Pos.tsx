import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import {
  Badge,
  Box,
  Button,
  Flex,
  HStack,
  IconButton,
  Input,
  Popover,
  Portal,
  RadioGroup,
  Stack,
  Text,
} from "@chakra-ui/react";
import { useQueryClient } from "@tanstack/react-query";
import { Lock, LogOut, Minus, Percent, Plus, Search, StickyNote, Trash2, Warehouse as WarehouseIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router-dom";

import DiscountField, { type DiscountType } from "../components/DiscountField";
import EnumSelect from "../components/EnumSelect";
import MoneyInput from "../components/MoneyInput";
import NumberInput from "../components/NumberInput";
import PrinterSelect from "../components/PrinterSelect";
import ProductImage from "../components/ProductImage";
import WarehouseSelect from "../components/WarehouseSelect";
import KitchenNoteDialog from "./pos/KitchenNoteDialog";
import { CustomerBar, PrescriptionBar, QuickAmountRow } from "./pos/posBars";
import { CustomerPickerDialog, PrescriptionPickerDialog, ReceiptDialog } from "./pos/posDialogs";
import RestaurantBar from "./pos/RestaurantBar";
import { Product, type ProductUnit } from "../gen/inventory_iface/v1/product_pb";
import type { ProductPriceTier } from "../gen/inventory_iface/v1/product_price_tier_pb";
import { PaymentSource, Sale, SaleStatus, type SaleItem } from "../gen/pos_iface/v1/sale_pb";
import { saleClient } from "../lib/clients";
import { formatMoney } from "../lib/format";
import { toast } from "../lib/toaster";
import { useAuth } from "../lib/auth";
import { canSell, unitAvailability } from "../lib/posAvailability";
import { POS_PRINTER_KEY, decodePrinter } from "../lib/printerTarget";
import { WAREHOUSE_KEY } from "../lib/transport";
import { useMyWarehousesQuery } from "../queries/warehouses";
import { useAllProductsQuery } from "../queries/products";
import { nextTier, tiersForUnit } from "../queries/productPriceTiers";
import { useStockLevelsQuery } from "../queries/stock";
import { useConnectorsQuery } from "../queries/connectors";
import { useBusinessMode } from "../queries/settings";
import {
  useAddItemMutation,
  useAttachPrescriptionMutation,
  useCompleteSaleMutation,
  useDetachPrescriptionMutation,
  useFireToKitchenMutation,
  useClearLineDiscountMutation,
  useRemoveItemMutation,
  useSetCartDiscountMutation,
  useSetItemNoteMutation,
  useSetItemQuantityMutation,
  useSetLineDiscountMutation,
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
// The receipt-printer target (POS_PRINTER_KEY / decodePrinter) is shared with
// order-history reprint — see lib/printerTarget.ts.

// Release the body lock Chakra/Ark leaves behind when a modal Dialog is
// unmounted via navigation instead of a normal close (it sets pointer-events:
// none + overflow:hidden on <body> and aria-hidden on #root, and doesn't
// restore them on abrupt unmount). Without this the destination page is frozen.
function releaseModalBodyLock() {
  document.body.style.removeProperty("pointer-events");
  document.body.style.removeProperty("overflow");
  document.getElementById("root")?.removeAttribute("aria-hidden");
}

// LineDiscountPopover is the per-cart-line discount affordance: a small button
// (highlighted when a discount is set) that opens a popover with the shared
// <DiscountField> + Apply/Clear. Draft state is local; Apply commits via the
// passed handler. Kept always-mounted, controlled by `open` (Ark body-lock rule).
function LineDiscountPopover({
  item,
  onApply,
  onClear,
}: {
  item: SaleItem;
  onApply: (type: DiscountType, human: number) => void | Promise<void>;
  onClear: () => void | Promise<void>;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const seed = (): { type: DiscountType; value: number } => {
    // Seed from a MANUAL discount only; an auto product discount starts blank so
    // the cashier types a fresh override.
    const type = (item.discountManual ? item.discountType : "FIXED") as DiscountType;
    const raw = item.discountManual ? Number(item.discountValue) : 0;
    return { type, value: type === "PERCENT" ? raw / 100 : raw };
  };
  const [draft, setDraft] = useState(seed);
  const hasDiscount = Number(item.lineDiscount) > 0;
  const isAuto = hasDiscount && !item.discountManual;

  return (
    <Popover.Root
      open={open}
      onOpenChange={(e) => {
        setOpen(e.open);
        if (e.open) setDraft(seed());
      }}
      positioning={{ placement: "bottom-end" }}
    >
      <Popover.Trigger asChild>
        <IconButton
          aria-label={t("pos.lineDiscount")}
          size="xs"
          variant={hasDiscount ? "subtle" : "ghost"}
          colorPalette={hasDiscount ? "blue" : undefined}
        >
          <Percent size={14} />
        </IconButton>
      </Popover.Trigger>
      <Portal>
        <Popover.Positioner>
          <Popover.Content width="auto">
            <Popover.Body>
              <Stack gap={2}>
                <Text fontSize="xs" color="fg.muted">
                  {t("pos.lineDiscount")}
                </Text>
                {isAuto && (
                  <Text fontSize="xs" color="green.fg">
                    {t("pos.autoDiscountHint")}
                  </Text>
                )}
                {item.tierMinQty > 0 && (
                  <Text fontSize="xs" color="purple.fg">
                    {t("pos.grosirDiscountHint")}
                  </Text>
                )}
                <DiscountField
                  type={draft.type}
                  value={draft.value}
                  onChange={(type, value) => setDraft({ type, value })}
                />
                <HStack justify="flex-end" gap={2}>
                  <Button
                    size="xs"
                    variant="ghost"
                    onClick={() => {
                      void onClear();
                      setOpen(false);
                    }}
                  >
                    {t("pos.discountClear")}
                  </Button>
                  <Button
                    size="xs"
                    colorPalette="blue"
                    onClick={() => {
                      void onApply(draft.type, draft.value);
                      setOpen(false);
                    }}
                  >
                    {t("pos.discountApply")}
                  </Button>
                </HStack>
              </Stack>
            </Popover.Body>
          </Popover.Content>
        </Popover.Positioner>
      </Portal>
    </Popover.Root>
  );
}

export default function Pos() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const { user } = useAuth();

  const { isPharmacy, isRestaurant } = useBusinessMode();

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
  const fireToKitchen = useFireToKitchenMutation();
  const setItemNote = useSetItemNoteMutation();

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

  // Restaurant: the order type a NEW counter cart is started with. A dine-in
  // order never comes through here — it arrives already bound to a table from
  // the floor plan — so this only ever holds "" | TAKEAWAY | DELIVERY.
  const [counterOrderType, setCounterOrderType] = useState("");
  // The cart line whose kitchen note is being edited (null = dialog closed).
  const [noteFor, setNoteFor] = useState<SaleItem | null>(null);

  const ensureSale = useCallback(async (): Promise<Sale | null> => {
    if (sale) return sale;
    try {
      const res = await startSale.mutateAsync({ orderType: counterOrderType });
      if (res.sale) setSale(res.sale);
      return res.sale ?? null;
    } catch {
      return null;
    }
  }, [sale, startSale, counterOrderType]);

  // Lines not yet sent to the kitchen. Drives the Fire button's count and its
  // disabled state; firing is incremental server-side, so this is a cue, not a
  // guard.
  const pendingFireCount = useMemo(
    () => (sale?.items ?? []).filter((it) => it.firedAt === 0n).length,
    [sale],
  );

  // Changing the order type of a counter cart. Before any line exists there is
  // no sale yet, so this just records the choice for the draft ensureSale will
  // create; once a cart exists the type is fixed for it (changing it would mean
  // re-keying an order the kitchen may already have) — the cashier clears the
  // cart to switch.
  const onChangeOrderType = useCallback(
    (orderType: string) => {
      if (sale) return;
      setCounterOrderType(orderType);
    },
    [sale],
  );

  const onSaveNote = useCallback(
    async (note: string) => {
      if (!sale || !noteFor) return;
      try {
        const res = await setItemNote.mutateAsync({
          saleId: sale.id,
          itemId: noteFor.id,
          note,
        });
        if (res.sale) setSale(res.sale);
        setNoteFor(null);
      } catch (err) {
        toast.fromError(err);
      }
    },
    [sale, noteFor, setItemNote],
  );

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
      // A TABLE-BOUND bill is never discarded on leave. An abandoned counter
      // cart is garbage nobody will miss, which is what makes deleting it safe;
      // an open bill on a table is a seated customer's order that the waiter is
      // expected to come back to — walking away from the screen must not throw
      // it away. (The server's stale-draft sweeper skips them for the same
      // reason.) Clearing a table is an explicit floor action.
      if (s && s.status === SaleStatus.DRAFT && !s.tableId) {
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

  // Fire the unfired lines to the kitchen. Declared here, after the printer
  // selection it reads: an explicit POS printer choice is passed through, and an
  // empty one lets the server resolve the saved KITCHEN target (falling back to
  // the receipt printer).
  const onFire = useCallback(async () => {
    if (!sale) return;
    try {
      const res = await fireToKitchen.mutateAsync({
        saleId: sale.id,
        ...decodePrinter(printerValue),
      });
      if (res.firedItems > 0) {
        toast.success(t("pos.firedToast", { count: res.firedItems }));
        // firedAt changed on every line just sent — refresh the cart so the
        // per-line cue and the Fire count are accurate.
        const fresh = await saleClient.getSale({ id: sale.id }).catch(() => null);
        if (fresh?.sale) setSale(fresh.sale);
      }
    } catch (err) {
      toast.fromError(err);
    }
  }, [sale, fireToKitchen, printerValue, t]);

  // Mount: restore a preserved DRAFT cart if we're returning from the
  // create-resep page (?attachRx=<id>), otherwise start a fresh draft. When an
  // Rx was just created we attach it and re-add the deferred Rx-required product
  // — a single round-trip lands the resep + the item that needed it.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;

    const attachRx = searchParams.get("attachRx") ?? "";
    // ?sale=<id> — arriving from the floor plan on a table's open bill. It wins
    // over any persisted counter draft: the waiter tapped a specific table, and
    // silently resuming an unrelated cart instead would put the next item on the
    // wrong bill.
    const tableSaleId = searchParams.get("sale") ?? "";
    const persistedSaleId = tableSaleId || localStorage.getItem(POS_DRAFT_KEY);
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
      } else if (tableSaleId) {
        // Strip ?sale too — a refresh should resume from state, not re-resolve
        // a table id that may have been settled in the meantime.
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
  // `tiers` is precomputed here (not per render) so the search list stays cheap.
  // It drives the DISPLAY hint only — a cart line's applied-grosir state always
  // comes from the server (SaleItem.tierMinQty), never from this.
  type UnitRow = {
    med: Product;
    unit: ProductUnit;
    available: number;
    unlimited: boolean;
    tiers: ProductPriceTier[];
  };
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
      for (const unit of med.units.filter((u) => u.sellable && u.active)) {
        const tiers = tiersForUnit(med.priceTiers, unit.id).filter((t) => t.price < unit.sellPrice);
        // Availability is kind-aware: a menu item sells its buildable portions
        // and a service never runs out — neither has batches for the ledger map
        // to report. See lib/posAvailability.ts.
        const { available, unlimited } = unitAvailability(med, unit, stockByProduct.get(med.id));
        out.push({ med, unit, available, unlimited, tiers });
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
      // The scan/SKU-exact path passes no `available`, so it asks the same
      // kind-aware helper rather than falling back to the raw ledger map (which
      // reports 0 for every menu item and every service).
      const enough =
        available !== undefined ? available >= 1 : canSell(product, stockByProduct.get(product.id));
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
      // An unlimited row (a SERVICE) reports available: 0 — passing that through
      // would read as out-of-stock. Passing undefined makes onAdd ask canSell,
      // which knows the kind.
      if (row) void onAdd(row.med, row.unit.id, row.unlimited ? undefined : row.available);
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
          <Stack gap={1}>
            {unitRows.map((row, i) => {
              const { med: m, unit, available, unlimited, tiers } = row;
              const out = !unlimited && available < 1;
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
                  onClick={() => !out && onAdd(m, unit.id, unlimited ? undefined : available)}
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
                      {/* A service has no quantity to report — printing "0" or a
                          fake large number would both be lies. */}
                      {m.sku}
                      {unlimited ? "" : ` · ${available} ${unit.name}`}
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
            {isRestaurant && (
              <RestaurantBar
                sale={sale}
                pendingCount={pendingFireCount}
                isFiring={fireToKitchen.isPending}
                onFire={onFire}
                onOrderTypeChange={onChangeOrderType}
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
                      {/* Restaurant: the cook-facing note, and whether this line
                          has already gone to the kitchen. Both belong on the
                          line itself — a waiter asked "did the satay go?" is
                          looking at the cart, not at a separate log. */}
                      {isRestaurant && (it.kitchenNote || it.firedAt > 0n) && (
                        <HStack gap={1.5}>
                          {it.kitchenNote && (
                            <Text fontSize="2xs" color="orange.fg" truncate title={it.kitchenNote}>
                              * {it.kitchenNote}
                            </Text>
                          )}
                          {it.firedAt > 0n && (
                            <Text fontSize="2xs" color="fg.muted">
                              {t("pos.lineFired")}
                            </Text>
                          )}
                        </HStack>
                      )}
                    </Stack>
                    {isRestaurant && (
                      <IconButton
                        aria-label={t("pos.noteTitle")}
                        size="xs"
                        variant={it.kitchenNote ? "solid" : "ghost"}
                        colorPalette={it.kitchenNote ? "orange" : undefined}
                        onClick={() => setNoteFor(it)}
                      >
                        <StickyNote size={14} />
                      </IconButton>
                    )}
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

      <KitchenNoteDialog
        item={noteFor}
        open={noteFor != null}
        isPending={setItemNote.isPending}
        onSave={onSaveNote}
        onCancel={() => setNoteFor(null)}
      />

      <ReceiptDialog
        sale={completedSale}
        onClose={onCloseReceipt}
        printerTarget={decodePrinter(printerValue)}
      />
    </Flex>
  );
}

