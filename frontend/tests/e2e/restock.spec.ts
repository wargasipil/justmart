import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// End-to-end restock (purchase order) flow:
//   1. Seed a product with strip + box units, and a supplier (via the Connect
//      JSON API — fast + deterministic, same approach as pos-rx.spec.ts).
//   2. Create a restock through the UI: pick the supplier, pick the product,
//      switch the line to the "box" unit, set qty + line total, click Create.
//   3. On the restock DETAIL page, assert the supplier name and the product
//      info (name + unit-aware ordered qty "5 box") render correctly.
//
// Supplier/product names on the detail page are resolved client-side
// (GetPurchaseOrder returns only IDs), so this must be a browser test.

type Seed = { productId: string; supplierId: string };

// Products are added to the restock form through <ProductPickerDialog> — a
// searchable, server-paginated multi-select. One line per checked product, so
// this is also how you'd REMOVE one (re-open, untick, Done).
async function addProductLine(page: Page, name: string) {
  // Glossary-aware: "Add product" in retail, "Add medicine" in pharmacy mode.
  await page.getByRole("button", { name: /^Add (product|medicine)$/ }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByPlaceholder(/Search by name or SKU/i).fill(name);
  await page.waitForTimeout(700); // debounced ListProducts
  await dialog.getByRole("row", { name: new RegExp(name) }).click();
  await dialog.getByRole("button", { name: "Done" }).click();
  await expect(dialog).toBeHidden();
}

// Fill the first line's qty + cost-per-item. Both are <MoneyInput>/<NumberInput>
// text inputs (role "textbox", NOT "spinbutton"), and the row has three of them
// (qty, cost, discount value), so address them by CELL position:
// [0] product [1] unit [2] qty [3] cost [4] discount [5] derived [6] subtotal.
async function fillFirstLine(page: Page, qty: string, cost: string) {
  const cells = page.getByRole("table").getByRole("row").nth(1).getByRole("cell");
  await cells.nth(2).getByRole("textbox").fill(qty);
  await cells.nth(3).getByRole("textbox").fill(cost);
}

async function seed(page: Page, marker: string): Promise<Seed> {
  await page.goto("/");
  return await page.evaluate(async (m: string) => {
    const token = localStorage.getItem("justmart_access_token");
    if (!token) throw new Error("no access token in localStorage");
    const headers = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    };
    const post = async (path: string, body: unknown) => {
      const res = await fetch(`/api/${path}`, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`);
      return res.json();
    };

    // Product with base tablet + strip (×10) + box (×100). Connect JSON encodes
    // int64 (unitPrice, factor, sellPrice) as strings.
    const med = await post("inventory_iface.v1.ProductService/CreateProduct", {
      sku: `RS-${m}`,
      name: `Restock Med ${m}`,
      unit: "tablet",
      unitPrice: "700",
      units: [
        { name: "strip", factor: "10", sellPrice: "6500", sellable: true, purchasable: true },
        { name: "box", factor: "100", sellPrice: "60000", sellable: true, purchasable: true },
      ],
    });

    const sup = await post("inventory_iface.v1.SupplierService/CreateSupplier", {
      code: `RS${m}`,
      name: `Restock Supplier ${m}`,
    });

    return { productId: med.product.id, supplierId: sup.supplier.id };
  }, marker);
}

async function cleanup(
  page: Page,
  ids: { productId: string; supplierId: string; poId?: string },
): Promise<void> {
  await page.evaluate(async (s: { productId: string; supplierId: string; poId?: string }) => {
    const token = localStorage.getItem("justmart_access_token");
    if (!token) return;
    const headers = {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    };
    const post = (path: string, body: unknown) =>
      fetch(`/api/${path}`, { method: "POST", headers, body: JSON.stringify(body) });
    // DRAFT PO is voidable; archive the catalog rows (soft delete).
    if (s.poId) {
      await post("purchasing_iface.v1.PurchaseOrderService/VoidPurchaseOrder", { id: s.poId });
    }
    await post("inventory_iface.v1.SupplierService/ArchiveSupplier", { id: s.supplierId });
    await post("inventory_iface.v1.ProductService/ArchiveProduct", { id: s.productId });
  }, ids);
}

test.describe("restock (purchase order) end-to-end", () => {
  test("create product (units) + supplier → restock in box → detail shows supplier + product", async ({
    page,
  }) => {
    const m = String(Date.now());
    const ids = await seed(page, m);
    let poId: string | undefined;
    try {
      await page.goto("/purchasing/new");

      // Supplier — async SearchableSelect renders <input role="combobox">.
      await page.getByPlaceholder("Select supplier").fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700); // debounced SearchSuppliers
      await page.getByRole("option", { name: new RegExp(`Restock Supplier ${m}`) }).click();

      // Product (line) — checked in the picker dialog. That seeds the line's
      // units and defaults the unit to base ("tablet").
      await addProductLine(page, `Restock Med ${m}`);

      // Switch the line unit to "box". The row has TWO EnumSelect triggers
      // (<button role="combobox">) — unit, then line-discount mode — so take the
      // first, which is the unit cell.
      const unitTrigger = page.getByRole("table").locator('button[role="combobox"]').first();
      await expect(unitTrigger).toBeVisible();
      await unitTrigger.click();
      await page.getByRole("option", { name: "box" }).click();

      // Qty 5 box @ 300 000 per box.
      await fillFirstLine(page, "5", "300000");

      // Create → navigates to the restock detail page.
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/purchasing\/[0-9a-f-]{36}$/);
      poId = page.url().split("/").pop();

      // Restock detail: supplier name + product info are correct.
      await expect(page.getByText(`Restock Supplier ${m}`)).toBeVisible();
      await expect(page.getByText(`Restock Med ${m}`)).toBeVisible();
      // Ordered qty renders unit-aware: 500 base ÷ 100 = "5 box" (shown in both
      // the Ordered and Received cells, so match the first).
      await expect(page.getByText("5 box").first()).toBeVisible();

      // Restock LIST table: the Supplier column resolves to "code · name", not a
      // raw UUID (the ALL_LIMIT name-map preload must include our supplier).
      await page.goto("/purchasing/all");
      await page.getByPlaceholder(/Search PO/i).fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700); // debounced list query
      // Scope to the table cell (a combobox option can carry the same text).
      await expect(
        page.getByRole("cell", { name: new RegExp(`RS${m} · Restock Supplier ${m}`) }),
      ).toBeVisible();
    } finally {
      await cleanup(page, { ...ids, poId });
    }
  });

  // Receive flow: drive the same dialog the user reported errored on.
  // Send → Receive → assert status flips to RECEIVED.
  test("Send + Receive a PO walks the dialog and lands on RECEIVED", async ({ page }) => {
    const m = String(Date.now());
    const ids = await seed(page, m);
    let poId: string | undefined;
    try {
      // Same PO setup as the first test: 5 box of the seeded product.
      await page.goto("/purchasing/new");
      await page.getByPlaceholder("Select supplier").fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700);
      await page.getByRole("option", { name: new RegExp(`Restock Supplier ${m}`) }).click();
      await addProductLine(page, `Restock Med ${m}`);
      const unitTrigger = page.getByRole("table").locator('button[role="combobox"]').first();
      await unitTrigger.click();
      await page.getByRole("option", { name: "box" }).click();
      await fillFirstLine(page, "5", "300000");
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/purchasing\/[0-9a-f-]{36}$/);
      poId = page.url().split("/").pop();

      // Send → status flips to Sent.
      await page.getByRole("button", { name: "Send", exact: true }).click();
      await expect(page.getByText("Sent", { exact: true })).toBeVisible();

      // Receive → dialog opens.
      await page.getByRole("button", { name: "Receive", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();

      // Fill batch number + expiry inside the row. Every input here is a
      // textbox (qty is a <NumberInput>, NOT role "spinbutton"), so address them
      // by CELL: [0] product [1] qty [2] batch # [3] expiry. There is no unit
      // cost cell — the backend derives it from the PO line (net of discount,
      // plus PPN), so a receive can never contradict the order it fulfils.
      // Indexing the row's textboxes flat left expiry empty, which silently
      // kept Receive disabled.
      const receiveCells = dialog.getByRole("table").getByRole("row").nth(1).getByRole("cell");
      await receiveCells.nth(2).getByRole("textbox").fill(`RS-B1-${m}`);
      // DatePicker.Input under en locale takes MM/DD/YYYY.
      const expiry = receiveCells.nth(3).getByRole("textbox");
      await expiry.fill("12/31/2099");
      await expiry.blur();

      // Submit (dialog footer's Receive button).
      await dialog.getByRole("button", { name: "Receive", exact: true }).click();
      await expect(dialog).not.toBeVisible();

      // Status badge flips to Received (5 box ordered, 5 box received = full).
      // The detail page also has a "Received" table column header — match the
      // first occurrence (the badge, which renders above the table).
      await expect(page.getByText("Received", { exact: true }).first()).toBeVisible();
    } finally {
      // A RECEIVED PO can't be voided; sending the request would log a 400 and
      // trip the console-error fixture. Skip the void call by omitting poId.
      // The PO row stays around (timestamp-unique, no test interference);
      // supplier + product still get archived.
      void poId; // referenced for the catch above
      await cleanup(page, { productId: ids.productId, supplierId: ids.supplierId });
    }
  });

  // Cart-level discount + variable PPN rate drive the totals math; detail
  // page surfaces the breakdown including the chosen rate. Two spinbuttons
  // exist on the page: the items-table Qty (scoped via `table`) and the
  // totals-card rate input (outside the table).
  test("create PO with PPN @ variable rate updates totals + detail shows rate", async ({
    page,
  }) => {
    const m = String(Date.now());
    const ids = await seed(page, m);
    let poId: string | undefined;
    try {
      await page.goto("/purchasing/new");

      // Supplier.
      await page.getByPlaceholder("Select supplier").fill(`Restock Supplier ${m}`);
      await page.waitForTimeout(700);
      await page.getByRole("option", { name: new RegExp(`Restock Supplier ${m}`) }).click();

      // Item line: base unit (tablet), qty 10 @ 10 000 PER ITEM → gross 100 000.
      // (The cost column is cost-per-item, not the line total — the DPP the PPN
      // assertions below are built on is the 100 000 product, not the input.)
      await addProductLine(page, `Restock Med ${m}`);
      await fillFirstLine(page, "10", "10000");

      // Toggle PPN on at the default rate (11). Chakra Switch.HiddenInput is
      // display:none, so force-click the only checkbox on this page.
      await page.getByRole("checkbox").click({ force: true });
      // 100 000 × 11% = 11 000 → total 111 000.
      await expect(page.getByText(/111[.,]000/).first()).toBeVisible();

      // Change the rate to 12% (Indonesian rate transition).
      // The rate spinbutton is the rate input in the totals card — i.e. the
      // only non-table spinbutton on the page.
      const rateInput = page.getByLabel("PPN rate (%)");
      await rateInput.fill("12");
      // 100 000 × 12% = 12 000 → total 112 000.
      await expect(page.getByText(/112[.,]000/).first()).toBeVisible();

      // Create.
      await page.getByRole("button", { name: "Create" }).click();
      await page.waitForURL(/\/purchasing\/[0-9a-f-]{36}$/);
      poId = page.url().split("/").pop();

      // Detail page totals breakdown shows the chosen rate.
      await expect(page.getByText("PPN 12%")).toBeVisible();
      await expect(page.getByText(/112[.,]000/).first()).toBeVisible();
    } finally {
      await cleanup(page, { ...ids, poId });
    }
  });
});
