// The POS cart: the server-side DRAFT sale and every mutation on it. Lifted
// out of routes/Pos.tsx, which now only renders and wires.
//
// Three guarantees the till relies on:
//  1. SERIAL. Every request that carries the draft id runs through one promise
//     chain (tailRef): an op starts only after the previous reply was applied.
//     Two quick clicks used to race, and the older reply could overwrite the
//     newer cart, hiding a line the server would still charge.
//  2. ONE DRAFT. ensureSale() shares an in-flight StartSale instead of starting
//     a second draft.
//  3. RECOVER. A draft that vanished while the till stayed open (the 24h
//     sweeper, completed or voided elsewhere) answers sale.not_found /
//     sale.not_draft. The hook drops it and starts a fresh draft; an ADD is
//     retried once on it so the scanned item lands. When the dead cart had
//     lines on screen the cashier gets pos.cartExpired: the new cart starts
//     EMPTY, the old lines are not re-added. Other ops are not retried (their
//     line ids belong to the dead cart). A COMPLETE answering sale.not_draft
//     first reads the sale back: if it did complete (lost reply), that is shown
//     as a normal completion, so the same sale is never rung twice.
//
// The cart mutation hooks are meta.silentError; every error that is not a dead
// draft is toasted here with toast.fromError, the same message as before.
import { useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import type { DiscountType } from "../../components/DiscountField";
import { PaymentSource, Sale, SaleStatus, type SaleItem } from "../../gen/pos_iface/v1/sale_pb";
import { saleClient } from "../../lib/clients";
import type { DeferredAdd } from "../../lib/posStorage";
import { connectMessage } from "../../lib/serverErrors";
import { toast } from "../../lib/toaster";
import {
  invalidateAfterCompletedSale,
  useAddItemMutation,
  useAttachPrescriptionMutation,
  useClearLineDiscountMutation,
  useCompleteSaleMutation,
  useDetachPrescriptionMutation,
  useRemoveItemMutation,
  useSetCartDiscountMutation,
  useSetItemQuantityMutation,
  useSetLineDiscountMutation,
  useSetSaleCustomerMutation,
  useSetServiceFeeMutation,
  useStartSaleMutation,
} from "../../queries/sales";

const NOT_FOUND = "sale.not_found";
const NOT_DRAFT = "sale.not_draft";

type Reply = { sale?: Sale };
type RunOpts = {
  /** Start a draft first when there is none (add item, attach Rx). */
  ensure?: boolean;
  /** After recovering a dead draft, retry once on the fresh one (add item). */
  retry?: boolean;
  /** Checked against the live draft before each attempt; false skips the op. */
  gate?: (sale: Sale) => boolean;
};
type AddLine = { productId: string; productUnitId: string; qty: number };

/** A discount as the backend takes it: PERCENT in basis points, FIXED in minor units. */
export function discountToWire(type: DiscountType, human: number): number {
  return type === "PERCENT" ? Math.round(human * 100) : Math.round(human);
}

export function usePosCart() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  // mutateAsync is stable across renders, so the ops below keep their identity.
  const { mutateAsync: startSale } = useStartSaleMutation();
  const { mutateAsync: addItem } = useAddItemMutation();
  const { mutateAsync: setQty } = useSetItemQuantityMutation();
  const { mutateAsync: removeItem } = useRemoveItemMutation();
  const { mutateAsync: setCustomer } = useSetSaleCustomerMutation();
  const { mutateAsync: attachRx } = useAttachPrescriptionMutation();
  const { mutateAsync: detachRx } = useDetachPrescriptionMutation();
  const { mutateAsync: setFee } = useSetServiceFeeMutation();
  const { mutateAsync: setLineDisc } = useSetLineDiscountMutation();
  const { mutateAsync: clearLineDisc } = useClearLineDiscountMutation();
  const { mutateAsync: setCartDisc } = useSetCartDiscountMutation();
  const { mutateAsync: completeSale, isPending: completing } = useCompleteSaleMutation();

  const [sale, setSale] = useState<Sale | null>(null);
  // saleRef is the live draft: queued ops and the unmount cleanup read it, so
  // they never act on the draft a stale render captured.
  const saleRef = useRef<Sale | null>(null);
  const tailRef = useRef<Promise<unknown>>(Promise.resolve());
  const startingRef = useRef<Promise<Sale | null> | null>(null);
  const aliveRef = useRef(true);
  // Set before an intentional create-resep navigation so the unmount cleanup
  // keeps the persisted DRAFT instead of discarding it.
  const keepDraftRef = useRef(false);

  const apply = useCallback((next: Sale | null) => {
    saleRef.current = next;
    setSale(next);
  }, []);

  // Run task after every op queued before it has finished.
  const enqueue = useCallback(<T>(task: () => Promise<T>): Promise<T> => {
    const next = tailRef.current.then(task);
    tailRef.current = next.catch(() => undefined);
    return next;
  }, []);

  const ensureSale = useCallback((): Promise<Sale | null> => {
    if (saleRef.current) return Promise.resolve(saleRef.current);
    if (!startingRef.current) {
      startingRef.current = startSale()
        .then((res) => {
          const s = res.sale ?? null;
          if (s && !aliveRef.current) {
            // POS was left while StartSale was in flight: don't orphan the draft.
            void saleClient.discardSale({ saleId: s.id }).catch(() => {});
            return null;
          }
          if (s) apply(s);
          return s;
        })
        .catch(() => null) // StartSale's own error still toasts globally
        .finally(() => {
          startingRef.current = null;
        });
    }
    return startingRef.current;
  }, [startSale, apply]);

  // Replace a dead draft with a fresh one.
  const recover = useCallback(async (): Promise<Sale | null> => {
    const hadItems = (saleRef.current?.items.length ?? 0) > 0;
    apply(null);
    if (hadItems) toast.info(t("pos.cartExpired"));
    return ensureSale();
  }, [apply, ensureSale, t]);

  const run = useCallback(
    (op: (s: Sale) => Promise<Reply>, opts: RunOpts = {}): Promise<boolean> =>
      enqueue(async () => {
        if (!aliveRef.current) return false;
        let s = opts.ensure ? await ensureSale() : saleRef.current;
        for (let attempt = 0; s; attempt++) {
          if (opts.gate && !opts.gate(s)) return false;
          try {
            const res = await op(s);
            if (res.sale) apply(res.sale);
            return true;
          } catch (err) {
            const token = connectMessage(err);
            if (token !== NOT_FOUND && token !== NOT_DRAFT) {
              toast.fromError(err);
              return false;
            }
            const fresh = await recover();
            s = opts.retry && attempt === 0 ? fresh : null;
          }
        }
        return false;
      }),
    [enqueue, ensureSale, apply, recover],
  );

  // Cart ops ----
  const addLine = useCallback(
    (line: AddLine, gate?: (s: Sale) => boolean) =>
      run((s) => addItem({ saleId: s.id, ...line }), { ensure: true, retry: true, gate }),
    [run, addItem],
  );

  const removeLine = useCallback(
    (itemId: string) => run((s) => removeItem({ saleId: s.id, itemId })),
    [run, removeItem],
  );

  const changeQty = useCallback(
    (itemId: string, qty: number) =>
      qty <= 0 ? removeLine(itemId) : run((s) => setQty({ saleId: s.id, itemId, qty })),
    [run, removeLine, setQty],
  );

  // Switch a line's selling unit (box/strip/tablet): remove + re-add at the new
  // unit, keeping the same numeric qty. One queued op, so nothing interleaves.
  const changeUnit = useCallback(
    (item: SaleItem, unitId: string) => {
      if (unitId === item.productUnitId) return Promise.resolve(false);
      return run(async (s) => {
        const removed = await removeItem({ saleId: s.id, itemId: item.id });
        try {
          return await addItem({
            saleId: s.id,
            productId: item.productId,
            productUnitId: unitId,
            qty: item.qty,
          });
        } catch (err) {
          if (removed.sale) apply(removed.sale); // show what the server holds
          throw err;
        }
      });
    },
    [run, removeItem, addItem, apply],
  );

  const setSaleCustomer = useCallback(
    (customerId: string) => run((s) => setCustomer({ saleId: s.id, customerId })),
    [run, setCustomer],
  );

  const attachPrescription = useCallback(
    (prescriptionId: string) =>
      run((s) => attachRx({ saleId: s.id, prescriptionId }), { ensure: true }),
    [run, attachRx],
  );

  const detachPrescription = useCallback(
    () => run((s) => detachRx({ saleId: s.id })),
    [run, detachRx],
  );

  const setServiceFee = useCallback(
    (biayaJasa: bigint) => run((s) => setFee({ saleId: s.id, biayaJasa })),
    [run, setFee],
  );

  const setCartDiscount = useCallback(
    (type: DiscountType, human: number) =>
      run((s) =>
        setCartDisc({
          saleId: s.id,
          discountType: type,
          discountValue: BigInt(Math.max(0, discountToWire(type, human))),
        }),
      ),
    [run, setCartDisc],
  );

  const setLineDiscount = useCallback(
    (itemId: string, type: DiscountType, human: number) =>
      run((s) =>
        setLineDisc({
          saleId: s.id,
          itemId,
          discountType: type,
          discountValue: BigInt(Math.max(0, discountToWire(type, human))),
        }),
      ),
    [run, setLineDisc],
  );

  // Clear a manual line discount, reverting to the auto product discount (if any).
  const clearLineDiscount = useCallback(
    (itemId: string) => run((s) => clearLineDisc({ saleId: s.id, itemId })),
    [run, clearLineDisc],
  );

  // Resolves to the completed sale (the receipt) or null when nothing completed.
  const complete = useCallback(
    (paymentSource: PaymentSource, paidAmount: bigint): Promise<{ sale?: Sale } | null> =>
      enqueue(async () => {
        const s = saleRef.current;
        if (!s || !aliveRef.current) return null;
        try {
          const res = await completeSale({ saleId: s.id, paymentSource, paidAmount });
          apply(null);
          return { sale: res.sale };
        } catch (err) {
          const token = connectMessage(err);
          if (token === NOT_DRAFT) {
            // The reply may have been lost after the sale did complete.
            const got = await saleClient.getSale({ id: s.id }).catch(() => null);
            if (got?.sale?.status === SaleStatus.COMPLETED) {
              apply(null);
              invalidateAfterCompletedSale(qc);
              return { sale: got.sale };
            }
          }
          if (token === NOT_FOUND || token === NOT_DRAFT) await recover();
          else toast.fromError(err);
          return null;
        }
      }),
    [enqueue, completeSale, apply, qc, recover],
  );

  // Drop the in-progress DRAFT. Stock is per-warehouse, so the cart can't
  // survive a warehouse switch; the next add lazily starts a fresh draft
  // stamped with the new warehouse. Best-effort: raw client, errors swallowed.
  const discard = useCallback(
    () =>
      enqueue(async () => {
        if (startingRef.current) await startingRef.current;
        const s = saleRef.current;
        if (!s) return;
        await saleClient.discardSale({ saleId: s.id }).catch(() => {});
        apply(null);
      }),
    [enqueue, apply],
  );

  // Mount: restore the DRAFT preserved across the create-resep round-trip, else
  // start a fresh one. With a just-created resep (attachRx), attach it and
  // re-add the deferred Rx-required product in one op. Resolves true when the
  // attach was attempted (the caller then strips ?attachRx).
  const restore = useCallback(
    async (persistedSaleId: string, attachRxId: string, deferred: DeferredAdd | null) => {
      const restored = await enqueue(async () => {
        if (persistedSaleId) {
          const res = await saleClient.getSale({ id: persistedSaleId }).catch(() => null);
          if (res?.sale && res.sale.status === SaleStatus.DRAFT) {
            apply(res.sale);
            return true;
          }
        }
        await ensureSale(); // draft gone (or none kept): fresh start
        return false;
      });
      if (!restored || !attachRxId) return false;
      await run(async (s) => {
        let cur = (await attachRx({ saleId: s.id, prescriptionId: attachRxId })).sale ?? s;
        if (deferred?.productId) {
          try {
            const res = await addItem({
              saleId: cur.id,
              productId: deferred.productId,
              productUnitId: deferred.unitId ?? "",
              qty: 1,
            });
            cur = res.sale ?? cur;
          } catch (err) {
            apply(cur); // the resep did attach
            throw err;
          }
        }
        return { sale: cur };
      });
      return true;
    },
    [enqueue, apply, ensureSale, run, attachRx, addItem],
  );

  // Leaving POS deletes an abandoned DRAFT: it never completed, so it vanishes
  // entirely (no VOIDED trace, never reaches order history). Best-effort.
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
      if (keepDraftRef.current) return; // intentional create-resep nav: keep the cart
      const s = saleRef.current;
      if (s && s.status === SaleStatus.DRAFT) {
        void saleClient.discardSale({ saleId: s.id }).catch(() => {});
      }
    };
  }, []);

  const currentSale = useCallback(() => saleRef.current, []);
  const keepDraftOnLeave = useCallback(() => {
    keepDraftRef.current = true;
  }, []);

  return {
    /** The active DRAFT (null before the first StartSale / right after completion). */
    sale,
    /** The live draft, for callbacks that must not read a stale render. */
    currentSale,
    ensureSale,
    addLine,
    changeQty,
    changeUnit,
    removeLine,
    setSaleCustomer,
    attachPrescription,
    detachPrescription,
    setServiceFee,
    setCartDiscount,
    setLineDiscount,
    clearLineDiscount,
    complete,
    completing,
    discard,
    restore,
    keepDraftOnLeave,
  };
}

export type PosCart = ReturnType<typeof usePosCart>;
