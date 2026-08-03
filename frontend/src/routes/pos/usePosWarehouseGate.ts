// The POS selling-warehouse gate + in-place switcher. Split out of
// routes/Pos.tsx; behaviour unchanged.
//
// POS is full-screen (no TopBar selector), so the cashier chooses the active
// warehouse here. Auto-skipped when they have <=1 warehouse or one is already
// chosen. The choice drives the X-Warehouse-Id header (FEFO sells from this
// warehouse only). "Change warehouse" clears the choice to re-open the gate.
import { useCallback, useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { WAREHOUSE_KEY } from "../../lib/transport";
import { useMyWarehousesQuery } from "../../queries/warehouses";

export function usePosWarehouseGate({
  discardActiveSale,
}: {
  /**
   * Drop the in-progress DRAFT cart before switching warehouse. Stock is
   * per-warehouse, so the cart cannot survive the switch. Best-effort — the
   * caller swallows errors.
   */
  discardActiveSale: () => Promise<void>;
}) {
  const queryClient = useQueryClient();
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

  const confirmWarehouse = useCallback(
    (id: string) => {
      const prev = localStorage.getItem(WAREHOUSE_KEY);
      localStorage.setItem(WAREHOUSE_KEY, id);
      setCurrentWarehouse(id);
      // Refetch warehouse-scoped data with the new header — no full reload.
      if (prev !== id) void queryClient.invalidateQueries();
      setGateDone(true);
    },
    [queryClient],
  );

  // Switch the selling warehouse in place from the header picker. The
  // in-progress DRAFT cart is discarded (deleted, not voided); the next add
  // lazily starts a fresh draft stamped with the new warehouse.
  const switchWarehouse = useCallback(
    async (id: string) => {
      if (id === currentWarehouse) return;
      await discardActiveSale();
      localStorage.setItem(WAREHOUSE_KEY, id);
      setCurrentWarehouse(id);
      void queryClient.invalidateQueries();
    },
    [currentWarehouse, discardActiveSale, queryClient],
  );

  return {
    gateDone,
    warehouses,
    currentWarehouse,
    activeWarehouseName:
      warehouses.find((w) => w.id === currentWarehouse)?.name ?? "",
    confirmWarehouse,
    switchWarehouse,
  };
}
