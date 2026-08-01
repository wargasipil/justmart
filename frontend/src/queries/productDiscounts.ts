import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { productDiscountClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";
import type {
  CreateProductDiscountRequest,
  UpdateProductDiscountRequest,
} from "../gen/inventory_iface/v1/product_discount_pb";

export const productDiscountKeys = {
  all: ["productDiscounts"] as const,
  list: (productId: string, page: number, pageSize: number) =>
    [...productDiscountKeys.all, "list", productId, page, pageSize] as const,
};

// One page of a product's discounts (the Product detail "Discount" tab), newest
// first. Server-paginated; returns { rows, total }.
export function useProductDiscountsQuery(
  productId: string,
  opts: { page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE, enabled = true } = opts;
  const q = useQuery({
    queryKey: productDiscountKeys.list(productId, page, pageSize),
    queryFn: async () => {
      const res = await productDiscountClient.listProductDiscounts({
        productId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.discounts, total: res.total };
    },
    enabled: enabled && !!productId,
  });
  return { ...q, rows: q.data?.rows ?? [], total: q.data?.total ?? 0 };
}

export function useCreateProductDiscountMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateProductDiscountRequest>) => productDiscountClient.createProductDiscount(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productDiscountKeys.all }),
    meta: { silentError: true },
  });
}

export function useUpdateProductDiscountMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateProductDiscountRequest>) => productDiscountClient.updateProductDiscount(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: productDiscountKeys.all }),
    meta: { silentError: true },
  });
}

export function useDeleteProductDiscountMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => productDiscountClient.deleteProductDiscount({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: productDiscountKeys.all }),
  });
}

// The 4 modes = (discount_type FIXED|PERCENT) × per_item, surfaced as one select.
export type DiscountModeValue = "FIXED" | "PERCENT" | "FIXED_ITEM" | "PERCENT_ITEM";

export function modeValue(discountType: string, perItem: boolean): DiscountModeValue {
  if (perItem) return discountType === "PERCENT" ? "PERCENT_ITEM" : "FIXED_ITEM";
  return discountType === "PERCENT" ? "PERCENT" : "FIXED";
}

export function modeParts(m: DiscountModeValue): { discountType: "FIXED" | "PERCENT"; perItem: boolean } {
  return {
    discountType: m === "PERCENT" || m === "PERCENT_ITEM" ? "PERCENT" : "FIXED",
    perItem: m === "FIXED_ITEM" || m === "PERCENT_ITEM",
  };
}

// Display a discount's value: PERCENT as "10%", FIXED via formatMoney; "/item" suffix when per-item.
export function formatDiscountValue(
  discountType: string,
  perItem: boolean,
  value: bigint,
  formatMoney: (n: number | bigint) => string,
  perItemSuffix: string,
): string {
  const base = discountType === "PERCENT" ? `${Number(value) / 100}%` : formatMoney(value);
  return perItem ? `${base} ${perItemSuffix}` : base;
}
