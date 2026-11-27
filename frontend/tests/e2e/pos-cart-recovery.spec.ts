import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// The POS cart is a server-side DRAFT sale owned by routes/pos/usePosCart.ts.
// These pin the three ways a click/scan used to miss the cart while the
// product showed (not greyed) in the search list:
//   1. the draft vanished server-side (24h sweeper / completed elsewhere):
//      every add failed with a raw "sale not found" until POS was reopened;
//   2. two quick adds whose replies came back out of order: the older reply
//      overwrote the newer cart, hiding a line the server still charged;
//   3. two adds while no draft existed yet each started their own draft.
//
// A deliberately failed AddItem logs "Failed to load resource ... 404 (Not
// Found)", which the shared fixture's allowlist already tolerates.

type Seed = { productId: string; sku: string; name: string };

async function api<T = unknown>(page: Page, path: string, body: unknown): Promise<T> {
  return (await page.evaluate(
    async ([p, b]: readonly [string, unknown]) => {
      const token = localStorage.getItem("justmart_access_token");
      if (!token) throw new Error("no access token");
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      };
      // Seed stock in the warehouse POS sells from (the active one).
      const wh = localStorage.getItem("justmart_warehouse_id");
      if (wh) headers["X-Warehouse-Id"] = wh;
      const res = await fetch(`/api/${p}`, { method: "POST", headers, body: JSON.stringify(b) });
      if (!res.ok) throw new Error(`${p}: ${res.status} ${await res.text()}`);
      return (await res.json()) as unknown;
    },
    [path, body] as const,
  )) as T;
}

async function seedOne(page: Page, marker: string, tag: string): Promise<Seed> {
  const sku = `POS-RC-${marker}-${tag}`;
  const name = `POS Recovery ${tag} ${marker}`;
  const product = (
    await api<{ product: { id: string } }>(page, "inventory_iface.v1.ProductService/CreateProduct", {
      sku,
      name,
      unit: "tab",
      unitPrice: "3000",
    })
  ).product;
  await api(page, "inventory_iface.v1.BatchService/CreateBatch", {
    productId: product.id,
    batchNumber: `${sku}-B`,
    expiryDate: "2099-12-31",
    costPrice: "1000",
    initialQuantity: "20",
  });
  return { productId: product.id, sku, name };
}

async function seed(page: Page, marker: string): Promise<[Seed, Seed]> {
  await page.goto("/");
  return [await seedOne(page, marker, "A"), await seedOne(page, marker, "B")];
}

async function cleanup(page: Page, seeds: Seed[]): Promise<void> {
  // Leaving /pos discards the open draft (unmount cleanup).
  await page.goto("/");
  for (const s of seeds) {
    try {
      await api(page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: s.productId });
    } catch {
      /* */
    }
  }
}

type DraftItem = { productId: string; qty?: number };
async function draftItems(page: Page, saleId: string): Promise<DraftItem[]> {
  const res = await api<{ sale?: { items?: DraftItem[] } }>(
    page,
    "pos_iface.v1.SaleService/GetSale",
    { id: saleId },
  );
  return res.sale?.items ?? [];
}

// Record every draft id the till receives (StartSale / successful AddItem).
function trackDrafts(page: Page): string[] {
  const ids: string[] = [];
  page.on("response", async (res) => {
    if (!/SaleService\/(StartSale|AddItem)$/.test(res.url()) || !res.ok()) return;
    try {
      const j = (await res.json()) as { sale?: { id?: string } };
      if (j.sale?.id) ids.push(j.sale.id);
    } catch {
      /* body gone after navigation */
    }
  });
  return ids;
}

const searchBox = (page: Page) => page.getByPlaceholder(/search (medicine|product)|cari/i);
const cartLines = (page: Page) => page.getByRole("textbox", { name: "line quantity" });

async function search(page: Page, q: string): Promise<void> {
  await searchBox(page).fill(q);
  await page.waitForTimeout(300); // client-side filter
}

// Empty the search list so a product name on screen can only be a cart line.
// Call it only once the last add landed: a successful add clears the search
// box (resetAfterAdd), which would bring the catalog rows back.
const hideSearchRows = (page: Page, marker: string) => search(page, `no-such-${marker}`);

test.describe("POS cart recovery", () => {
  test("a draft deleted server-side is replaced and the next item still lands", async ({ page }) => {
    const marker = String(Date.now());
    const [a, b] = await seed(page, marker);
    const drafts = trackDrafts(page);
    try {
      await page.goto("/pos");
      await search(page, a.sku);
      await page.getByText(a.name).first().click();
      await expect(cartLines(page)).toHaveCount(1);
      await expect.poll(() => drafts.length).toBeGreaterThan(0);
      const dead = drafts[drafts.length - 1];

      // What the 24h sweeper does to an idle till's draft.
      await api(page, "pos_iface.v1.SaleService/DiscardSale", { saleId: dead });

      await search(page, b.sku);
      await page.getByText(b.name).first().click();

      await expect(page.getByText(/previous cart is no longer valid/i)).toBeVisible();
      await expect(searchBox(page)).toHaveValue(""); // the retried add landed
      await hideSearchRows(page, marker);
      await expect(cartLines(page)).toHaveCount(1);
      await expect(page.getByText(b.name)).toBeVisible();
      await expect(page.getByText(a.name)).toHaveCount(0); // new cart starts empty
      await expect(page.getByText(/sale not found/i)).toHaveCount(0);

      // The item is on a NEW draft, and that draft holds exactly what's on screen.
      await expect.poll(() => drafts[drafts.length - 1]).not.toBe(dead);
      const fresh = drafts[drafts.length - 1];
      const items = await draftItems(page, fresh);
      expect(items.map((i) => i.productId)).toEqual([b.productId]);
    } finally {
      await cleanup(page, [a, b]);
    }
  });

  test("two quick adds with out-of-order replies: the screen matches the server", async ({
    page,
  }) => {
    const marker = String(Date.now());
    const [a, b] = await seed(page, marker);
    const drafts = trackDrafts(page);
    try {
      await page.goto("/pos");
      await search(page, `POS-RC-${marker}-`);
      await expect(page.getByText(a.name).first()).toBeVisible();
      await expect(page.getByText(b.name).first()).toBeVisible();

      // Hold the FIRST AddItem reply so an unserialized second add would win.
      let n = 0;
      await page.route("**/pos_iface.v1.SaleService/AddItem", async (route) => {
        const mine = ++n;
        const resp = await route.fetch();
        if (mine === 1) await new Promise((r) => setTimeout(r, 1200));
        await route.fulfill({ response: resp });
      });
      await page.getByText(a.name).first().click();
      await page.getByText(b.name).first().click();

      await expect(cartLines(page)).toHaveCount(2, { timeout: 10_000 });
      await hideSearchRows(page, marker);
      await expect(cartLines(page)).toHaveCount(2);
      await expect(page.getByText(a.name)).toBeVisible();
      await expect(page.getByText(b.name)).toBeVisible();

      await expect.poll(() => drafts.length).toBeGreaterThan(0);
      const items = await draftItems(page, drafts[drafts.length - 1]);
      expect(items.map((i) => i.productId).sort()).toEqual([a.productId, b.productId].sort());
    } finally {
      // A held reply must not fail the test once the page navigates away.
      await page.unrouteAll({ behavior: "ignoreErrors" });
      await cleanup(page, [a, b]);
    }
  });

  test("a double-click while the first draft is still starting starts exactly one draft", async ({
    page,
  }) => {
    const marker = String(Date.now());
    const [a, b] = await seed(page, marker);
    const drafts = trackDrafts(page);
    // Hold the StartSale reply until after the double-click, so both clicks
    // happen while the till has no draft yet.
    let starts = 0;
    let release!: () => void;
    const held = new Promise<void>((r) => (release = r));
    await page.route("**/pos_iface.v1.SaleService/StartSale", async (route) => {
      starts++;
      const resp = await route.fetch();
      await held;
      await route.fulfill({ response: resp });
    });
    try {
      await page.goto("/pos");
      await search(page, a.sku);
      await page.getByText(a.name).first().dblclick();
      await page.waitForTimeout(300);
      release();

      await expect(cartLines(page)).toHaveValue("2"); // both clicks, one merged line
      await hideSearchRows(page, marker);
      await expect(cartLines(page)).toHaveCount(1);
      await expect(page.getByText(a.name)).toBeVisible();
      expect(starts).toBe(1);
      await expect.poll(() => drafts.length).toBeGreaterThanOrEqual(3); // StartSale + 2 AddItem
      expect(new Set(drafts).size).toBe(1);
      const items = await draftItems(page, drafts[0]);
      expect(items).toHaveLength(1);
      expect(Number(items[0].qty ?? 0)).toBe(2);
    } finally {
      release();
      // A held reply must not fail the test once the page navigates away.
      await page.unrouteAll({ behavior: "ignoreErrors" });
      await cleanup(page, [a, b]);
    }
  });
});
