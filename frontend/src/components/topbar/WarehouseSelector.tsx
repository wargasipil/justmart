import { HStack, Text } from "@chakra-ui/react";
import { useQueryClient } from "@tanstack/react-query";
import { Warehouse as WarehouseIcon } from "lucide-react";
import { useEffect, useState } from "react";

import WarehouseSelect from "../WarehouseSelect";
import { WAREHOUSE_KEY } from "../../lib/transport";
import { searchMyWarehouses, useMyWarehousesQuery } from "../../queries/warehouses";

// The TopBar's active-warehouse chip. Owns the persisted `justmart_warehouse_id`
// choice that the transport reads per request, so switching refetches every
// warehouse-scoped query in place — no full page reload.
export default function WarehouseSelector() {
  const queryClient = useQueryClient();
  const myWarehousesQ = useMyWarehousesQuery();
  const [current, setCurrent] = useState<string>(() => localStorage.getItem(WAREHOUSE_KEY) || "");

  // Once memberships load, default to the persisted choice or the user's
  // default warehouse.
  useEffect(() => {
    const data = myWarehousesQ.data;
    if (!data || data.warehouses.length === 0) return;
    const persisted = localStorage.getItem(WAREHOUSE_KEY);
    if (persisted && data.warehouses.some((w) => w.id === persisted)) {
      setCurrent(persisted);
      return;
    }
    const def = data.memberships.find((m) => m.isDefault);
    const fallback = def?.warehouseId ?? data.warehouses[0].id;
    setCurrent(fallback);
    localStorage.setItem(WAREHOUSE_KEY, fallback);
  }, [myWarehousesQ.data]);

  if (!myWarehousesQ.data) return null;
  const list = myWarehousesQ.data.warehouses;
  // 0 accessible warehouses: nothing to show; downstream calls already surface
  // a "no warehouse configured" error if they actually need one.
  if (list.length === 0) return null;

  // Chip label uses the cached full list — survives a typed query that filters
  // the selected warehouse out of the popover (async search source).
  const selectedFromFull = list.find((w) => w.id === current);
  const selectedLabel = selectedFromFull
    ? `${selectedFromFull.code} · ${selectedFromFull.name}`
    : undefined;

  // Single-warehouse user: render an informational read-only label so the
  // cashier always knows where they are. No popover, no chevron.
  if (list.length === 1) {
    const only = list[0];
    return (
      <HStack
        gap={2}
        px={3}
        py={1}
        borderWidth="1px"
        borderRadius="md"
        bg="bg.subtle"
        color="fg.muted"
      >
        <WarehouseIcon size={14} />
        <Text fontSize="sm" maxW="180px" truncate>
          {`${only.code} · ${only.name}`}
        </Text>
      </HStack>
    );
  }

  return (
    <WarehouseSelect
      size="sm"
      width="180px"
      value={current}
      onChange={(v) => {
        setCurrent(v);
        localStorage.setItem(WAREHOUSE_KEY, v);
        // Refetch all warehouse-scoped data with the new X-Warehouse-Id header
        // (the transport reads localStorage per request) — no full page reload.
        void queryClient.invalidateQueries();
      }}
      loadOptions={searchMyWarehouses}
      selectedLabel={selectedLabel}
    />
  );
}
