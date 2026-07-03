import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { productDiscountClient } from "../lib/clients";
import type {
  CreateProductDiscountRequest,
  UpdateProductDiscountRequest,
} from "../gen/inventory_iface/v1/product_discount_pb";

export const productDiscountKeys = {
  all: ["productDiscounts"] as const,
  list: (productId: string) => [...productDiscountKeys.all, "list", productId] as const,
};

// All discounts for one product (the Product detail "Discount" tab).
export function useProductDiscountsQuery(productId: string, enabled = true) {
  return useQuery({
    queryKey: productDiscountKeys.list(productId),
    queryFn: async () => (await productDiscountClient.listProductDiscounts({ productId })).discounts,
    enabled: enabled && !!productId,
  });
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
