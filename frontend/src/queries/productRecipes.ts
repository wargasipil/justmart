import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { productRecipeClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";
import { productKeys } from "./products";
import type {
  CreateProductRecipeItemRequest,
  UpdateProductRecipeItemRequest,
} from "../gen/inventory_iface/v1/product_recipe_pb";

export const productRecipeKeys = {
  all: ["productRecipes"] as const,
  list: (productId: string, page: number, pageSize: number) =>
    [...productRecipeKeys.all, "list", productId, page, pageSize] as const,
};

// `buildable` = -1 means "no recipe defined", which the UI must render
// differently from 0 ("out of an ingredient"). Exported so call sites read the
// intent instead of testing a magic number.
export const NO_RECIPE = -1n;

// One page of a COMPOSITE product's recipe — the Product detail "Resep" card.
// Each line carries its component's name/sku/unit and current on-hand in the
// active warehouse; `buildable` is how many portions the whole recipe can make
// (computed server-side over ALL lines, not just this page).
export function useProductRecipeQuery(
  productId: string,
  opts: { page?: number; pageSize?: number; enabled?: boolean } = {},
) {
  const { page = 0, pageSize = DEFAULT_PAGE_SIZE, enabled = true } = opts;
  const q = useQuery({
    queryKey: productRecipeKeys.list(productId, page, pageSize),
    queryFn: async () => {
      const res = await productRecipeClient.listProductRecipeItems({
        productId,
        limit: pageSize,
        offset: page * pageSize,
      });
      return { rows: res.items, total: res.total, buildable: res.buildable };
    },
    enabled: enabled && !!productId,
  });
  return {
    ...q,
    rows: q.data?.rows ?? [],
    total: q.data?.total ?? 0,
    buildable: q.data?.buildable ?? NO_RECIPE,
  };
}

// Recipes are EMBEDDED on Product (Get/List/SearchProducts hydrate `recipe`, and
// a composite's ready_stock IS its buildable portions), so a recipe write also
// invalidates the product cache — otherwise the catalog and POS would keep
// showing the old ingredient list and the old availability.
function invalidate(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: productRecipeKeys.all });
  qc.invalidateQueries({ queryKey: productKeys.all });
}

export function useCreateProductRecipeItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateProductRecipeItemRequest>) =>
      productRecipeClient.createProductRecipeItem(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useUpdateProductRecipeItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateProductRecipeItemRequest>) =>
      productRecipeClient.updateProductRecipeItem(req),
    onSuccess: () => invalidate(qc),
    meta: { silentError: true },
  });
}

export function useDeleteProductRecipeItemMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => productRecipeClient.deleteProductRecipeItem({ id }),
    onSuccess: () => invalidate(qc),
  });
}
