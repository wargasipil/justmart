import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Coverage for test-specification.md:
//   Pos    — grosir price applies and reverts as cart qty crosses a threshold
//   Pos    — grosir suppresses the automatic product discount but not a manual one
//   Pos    — an order from a grosir cart keeps the price through completion,
//            receipt and order history
//   Grosir — authoring round-trip on the Product detail "Grosir" tab
//
// The pricing arithmetic itself is pinned at the Go layer (price_tier_test.go,
// complete_sale_grosir_test.go); these specs prove the wiring the browser owns:
// the MoneyInput contract, the badge exclusivity, and the DRAFT → history path.

type Seed = { productId: string; baseUnitId: string; sku: string; name: string };

async function api<T = unknown>(page: Page, path: string, body: unknown): Promise<T> {
  return (await page.evaluate(
    async ([p, b]: [string, unknown]) => {
      const token = localStorage.getItem("justmart_access_token");
      if (!token) throw new Error("no access token");
      const res = await fetch(`/api/${p}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(b),
      });
      if (!res.ok) throw new Error(`${p}: ${res.status} ${await res.text()}`);
      return (await res.json()) as unknown;
    },
    [path, body] as const,
  )) as Promise<T>;
}

// A product at 10.000/pcs with stock, plus its base unit id (a tier targets one
// specific unit).
async function seed(page: Page, marker: string, qty: number): Promise<Seed> {
  await page.goto("/");
  const sku = `GROSIR-${marker}`;
  const name = `Grosir Item ${marker}`;
  const product = (
    await api<{ product: { id: string; units: Array<{ id: string; isBase?: boolean }> } }>(
      page,
      "inventory_iface.v1.ProductService/CreateProduct",
      { sku, name, unit: "pcs", unitPrice: "10000" },
    )
  ).product;
  const baseUnitId = (product.units ?? []).find((u) => u.isBase)?.id ?? "";
  await api(page, "inventory_iface.v1.BatchService/CreateBatch", {
    productId: product.id,
    batchNumber: `GROSIR-B-${marker}`,
    expiryDate: "2099-12-31",
    costPrice: "4000",
    initialQuantity: String(qty),
  });
  return { productId: product.id, baseUnitId, sku, name };
}

async function addTier(page: Page, s: Seed, minQty: number, price: number): Promise<void> {
  await api(page, "inventory_iface.v1.ProductPriceTierService/CreateProductPriceTier", {
    productId: s.productId,
    productUnitId: s.baseUnitId,
    minQty,
    price: String(price),
  });
}

async function archive(page: Page, s: Seed): Promise<void> {
  try {
    await api(page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: s.productId });
  } catch {
    /* */
  }
}

// Set the cart line's qty and wait for SetItemQuantity to land (every keystroke
// is an RPC round-trip).
async function setCartQty(page: Page, qty: number): Promise<void> {
  const qtyInput = page.getByRole("textbox", { name: "line quantity" }).first();
  await qtyInput.fill(String(qty));
  await qtyInput.blur();
  await page.waitForTimeout(800);
}

// The POS cart LINE row, reached from its qty input. Badge assertions must be
// scoped here: the search list shows its own "Wholesale" hint badge for the same
// product, so an unscoped getByText would match that instead.
function cart(page: Page) {
  return page.getByRole("textbox", { name: "line quantity" }).first().locator("..");
}

test.describe("Grosir (wholesale tiers)", () => {
  test("authoring: add a tier on the Grosir card, see the ladder, delete via ConfirmDialog", async ({
    page,
  }) => {
    const marker = String(Date.now());
    const s = await seed(page, marker, 500);
    try {
      await page.goto(`/products/${s.productId}`);
      await page.waitForLoadState("networkidle");
      // The grosir ladder is a CARD in the top grid (under Satuan), not a tab —
      // a tier prices ONE unit, so it only reads correctly beside the unit list.
      await expect(page.getByRole("heading", { name: /^(Grosir|Wholesale)$/ })).toBeVisible();

      // Empty state before any rung exists.
      await expect(page.getByText(/Belum ada harga grosir|No wholesale prices/i)).toBeVisible();

      // Add 12 -> 8.500 through the drawer.
      await page.getByRole("button", { name: /Tambah tingkat|Add tier/i }).click();
      const drawer = page.getByRole("dialog");
      await expect(drawer).toBeVisible();
      await drawer.getByLabel(/Jumlah Beli Minimum|Min\. Quantity to Buy/i).fill("12");
      await drawer.getByLabel(/Harga Grosir|Wholesale Price/i).fill("8500");
      await drawer.getByRole("button", { name: /Simpan|Save/i }).click();
      await expect(drawer).toBeHidden();

      // MoneyInput contract: typing 8500 must land as 8.500, not 85 or 8.
      await expect(page.getByText(/Beli ≥ 12|Buy ≥ 12/)).toBeVisible();
      await expect(page.getByText(/8[.,]500/).first()).toBeVisible();

      // Two more rungs, seeded via RPC, must render ascending under one group.
      await addTier(page, s, 60, 8000);
      await addTier(page, s, 144, 7200);
      await page.reload();
      const rungs = page.getByText(/Beli ≥ \d+|Buy ≥ \d+/);
      await expect(rungs).toHaveCount(3);
      await expect(rungs.nth(0)).toHaveText(/12/);
      await expect(rungs.nth(1)).toHaveText(/60/);
      await expect(rungs.nth(2)).toHaveText(/144/);
      // Saving column is populated (10.000 → 8.500 = 15%).
      await expect(page.getByText(/1[.,]500 \(15%\)/)).toBeVisible();

      // Delete the last rung — a real Chakra dialog (role=alertdialog), never
      // window.confirm.
      await page.getByRole("row").filter({ hasText: /144/ }).getByRole("button").last().click();
      const confirm = page.getByRole("alertdialog");
      await expect(confirm).toBeVisible();
      await confirm.getByRole("button", { name: /^(Hapus|Delete)$/i }).click();
      await expect(confirm).toBeHidden();
      await expect(page.getByText(/Beli ≥ \d+|Buy ≥ \d+/)).toHaveCount(2);

      // The page must still be interactive — guards the Ark body-lock rule.
      await page.getByRole("tab", { name: /Diskon|Discount/i }).click();
      await expect(page.getByRole("button", { name: /Tambah diskon|Add discount/i })).toBeVisible();
    } finally {
      await archive(page, s);
    }
  });

  test("POS: crossing the threshold applies grosir, suppresses Promo, and reverts on qty drop", async ({
    page,
  }) => {
    const marker = String(Date.now());
    const s = await seed(page, marker, 500);
    try {
      await addTier(page, s, 12, 8500);
      // A live 10% auto discount so the green Promo badge is showing at qty 1.
      await api(page, "inventory_iface.v1.ProductDiscountService/CreateProductDiscount", {
        productId: s.productId,
        discountType: "PERCENT",
        value: "1000",
        minQty: 0,
      });

      await page.goto("/pos");
      await page.waitForLoadState("networkidle");
      await page.getByPlaceholder(/search (medicine|product)|cari/i).fill(s.sku);
      await page.waitForTimeout(400);

      // The search row hints the ladder before anything is in the cart.
      await expect(page.getByText(/≥\s*12/).first()).toBeVisible();
      await page.getByText(s.name).first().click();
      await page.waitForTimeout(500);

      // qty 1: normal price, Promo visible, no Grosir badge on the cart line.
      const c = cart(page);
      await expect(c.getByText(/^Promo$/)).toBeVisible();
      await expect(c.getByText(/^Grosir$|^Wholesale$/)).toHaveCount(0);

      // qty 12 crosses the rung — all four assertions at once.
      await setCartQty(page, 12);
      await expect(c.getByText(/^Grosir$|^Wholesale$/).first()).toBeVisible();
      await expect(c.getByText(/^Promo$/)).toHaveCount(0); // grosir suppresses it
      await expect(c.getByText(/10[.,]000/).first()).toBeVisible(); // struck-through list price
      await expect(c.getByText(/8[.,]500/).first()).toBeVisible();
      await expect(c.getByText(/102[.,]000/).first()).toBeVisible(); // 12 x 8.500

      // Dropping back below the rung restores the normal price (anti-ratchet).
      await setCartQty(page, 5);
      await expect(c.getByText(/^Grosir$|^Wholesale$/)).toHaveCount(0);
      await expect(c.getByText(/^Promo$/)).toBeVisible();
    } finally {
      await archive(page, s);
    }
  });

  test("POS: an order from a grosir cart keeps the price through receipt and order history", async ({
    page,
  }) => {
    const marker = String(Date.now());
    const s = await seed(page, marker, 500);
    try {
      await addTier(page, s, 12, 8500);

      await page.goto("/pos");
      await page.waitForLoadState("networkidle");
      await page.getByPlaceholder(/search (medicine|product)|cari/i).fill(s.sku);
      await page.waitForTimeout(400);
      await page.getByText(s.name).first().click();
      await page.waitForTimeout(500);
      await setCartQty(page, 12);

      const paidInput = page.getByText(/^Paid$|^Bayar$/).locator("..").getByRole("textbox");
      await paidInput.fill("102000");
      await page.keyboard.press("F8");

      const receipt = page.getByRole("dialog");
      await expect(receipt).toBeVisible({ timeout: 5000 });
      await expect(receipt.getByText(/102[.,]000/).first()).toBeVisible();

      // The persisted line carries the full pricing provenance.
      type SaleItem = {
        productId: string;
        unitPriceSnapshot?: string;
        listPriceSnapshot?: string;
        tierMinQty?: number;
      };
      const sales = await api<{ sales?: Array<{ id: string; items?: SaleItem[] }> }>(
        page,
        "pos_iface.v1.SaleService/ListSales",
        { query: s.name, limit: 5 },
      );
      const line = (sales.sales ?? [])
        .flatMap((sale) => sale.items ?? [])
        .find((it) => it.productId === s.productId);
      expect(line).toBeDefined();
      expect(Number(line!.unitPriceSnapshot ?? 0)).toBe(8500);
      expect(Number(line!.listPriceSnapshot ?? 0)).toBe(10000);
      expect(Number(line!.tierMinQty ?? 0)).toBe(12);

      // And order history shows the grosir price, not the list price.
      await page.goto("/orders");
      await page.getByPlaceholder(/Search|Cari/i).first().fill(s.name);
      await page.waitForTimeout(500);
      const row = page.getByRole("row").filter({ hasText: s.name });
      await expect(row).toBeVisible();
      await row.click();
      await page.waitForURL(/\/orders\/[0-9a-f-]{36}$/);
      await expect(page.getByText(/8[.,]500/).first()).toBeVisible();
      await expect(page.getByText(/102[.,]000/).first()).toBeVisible();
      await expect(page.getByText(/^Grosir$|^Wholesale$/).first()).toBeVisible();
      await expect(page.getByText(/≥\s*12/).first()).toBeVisible();
    } finally {
      await archive(page, s);
    }
  });

  test("POS: the 'buy N more' nudge only shows inside its window", async ({ page }) => {
    const marker = String(Date.now());
    const s = await seed(page, marker, 500);
    try {
      await addTier(page, s, 12, 8500);

      await page.goto("/pos");
      await page.waitForLoadState("networkidle");
      await page.getByPlaceholder(/search (medicine|product)|cari/i).fill(s.sku);
      await page.waitForTimeout(400);
      await page.getByText(s.name).first().click();
      await page.waitForTimeout(500);

      const c = cart(page);
      // 1 short of the rung → nudge shows.
      await setCartQty(page, 11);
      await expect(c.getByText(/Kurang 1 lagi|1 more/i)).toBeVisible();

      // At the rung → grosir is active, nudge is gone.
      await setCartQty(page, 12);
      await expect(c.getByText(/Kurang \d+ lagi|\d+ more/i)).toHaveCount(0);
      await expect(c.getByText(/^Grosir$|^Wholesale$/).first()).toBeVisible();

      // Far below (outside the max(2, 20%) window) → no nudge either.
      await setCartQty(page, 5);
      await expect(c.getByText(/Kurang \d+ lagi|\d+ more/i)).toHaveCount(0);
    } finally {
      await archive(page, s);
    }
  });
});
