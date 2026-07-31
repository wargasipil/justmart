import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { productPriceTierClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";
import { productKeys } from "./products";
import type {
  CreateProductPriceTierRequest,
  ProductPriceTier,
  UpdateProductPriceTierRequest,
} from "../gen/inventory_iface/v1/product_price_tier_pb";
import type { ProductUnit } from "../gen/inventory_iface/v1/product_pb";

export const productPriceTierKeys = {
  all: ["productPriceTiers"] as const,
  list: (productId: string, page: number, pageSize: number) =>
    [...productPriceTierKeys.all, "list", productId, page, pageSize] as const,
};

// One page of grosir (wholesale) tiers for a product — the Product detail
// "Grosir" card. Server-ordered base unit first, then by factor, then ascending
// threshold; returns { rows, total }.
//
// NOTE: this is the ADMIN read. POS resolves tiers from the `priceTiers` array
// embedded on Product, which is never paginated — a partial ladder would
// mis-price a sale. Don't route POS through this hook.
export function useProductPriceTiersQuery(
  productId: string,
  opts: { page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE, enabled = true } = opts;
  const q = useQuery({
    queryKey: productPriceTierKeys.list(productId, page, pageSize),
    queryFn: async () => {
      const res = await productPriceTierClient.listProductPriceTiers({
        productId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.tiers, total: res.total };
    },
    enabled: enabled && !!productId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

// Unlike productDiscounts, tiers are EMBEDDED on Product (Get/List/SearchProducts
// hydrate `price_tiers`), so a tier write also staleness-invalidates the product
// cache — otherwise the POS search-row hint and the detail-page unit chips would
// keep rendering the old ladder.
function invalidate(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: productPriceTierKeys.all });
  qc.invalidateQueries({ queryKey: productKeys.all });
}

export function useCreateProductPriceTierMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateProductPriceTierRequest>) =>
      productPriceTierClient.createProductPriceTier(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useUpdateProductPriceTierMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateProductPriceTierRequest>) =>
      productPriceTierClient.updateProductPriceTier(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useDeleteProductPriceTierMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => productPriceTierClient.deleteProductPriceTier({ id }),
    onSuccess: () => invalidate(qc),
  });
}

// ---------------------------------------------------------------------------
// Pure helpers. `activeTier` / `nextTier` are DISPLAY ONLY, for the POS search
// rows where no sale line exists yet. A cart line's applied-grosir state always
// comes from the server (SaleItem.tierMinQty / listPriceSnapshot) — never
// recompute it client-side, or the screen and the receipt can disagree.
// ---------------------------------------------------------------------------

// Tiers for one unit, ascending by threshold.
export function tiersForUnit(tiers: ProductPriceTier[] | undefined, unitId: string): ProductPriceTier[] {
  if (!tiers?.length || !unitId) return [];
  return tiers.filter((t) => t.productUnitId === unitId).sort((a, b) => a.minQty - b.minQty);
}

// The tier a line of `qty` would get: cheapest qualifying rung, ties to the
// lowest threshold. Mirrors the backend's resolveTierPrice, including the strict
// "must beat the normal price" rule — pass the unit's sellPrice as listPrice.
export function activeTier(
  tiers: ProductPriceTier[] | undefined,
  unitId: string,
  qty: number,
  listPrice: bigint,
): ProductPriceTier | undefined {
  let best: ProductPriceTier | undefined;
  for (const t of tiersForUnit(tiers, unitId)) {
    if (t.minQty <= 0 || qty < t.minQty) continue;
    if (t.price >= listPrice) continue;
    if (!best || t.price < best.price) best = t;
  }
  return best;
}

// The next rung a line of `qty` has not reached yet — powers the POS nudge.
export function nextTier(
  tiers: ProductPriceTier[] | undefined,
  unitId: string,
  qty: number,
  listPrice: bigint,
): ProductPriceTier | undefined {
  return tiersForUnit(tiers, unitId).find((t) => t.minQty > qty && t.price < listPrice);
}

// [{ unit, tiers }] in the product's own unit order (base first, then ascending
// factor); units with no ladder are omitted.
export function groupTiersByUnit(
  tiers: ProductPriceTier[] | undefined,
  units: ProductUnit[],
): { unit: ProductUnit; tiers: ProductPriceTier[] }[] {
  const ordered = [...units]
    .filter((u) => u.active)
    .sort((a, b) => (a.isBase === b.isBase ? Number(a.factor) - Number(b.factor) : a.isBase ? -1 : 1));
  return ordered
    .map((unit) => ({ unit, tiers: tiersForUnit(tiers, unit.id) }))
    .filter((g) => g.tiers.length > 0);
}
