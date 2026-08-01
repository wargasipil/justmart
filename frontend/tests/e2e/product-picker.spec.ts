import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// <ProductPickerDialog> on the restock (purchase order) form: products are
// added by CHECKING them in a searchable, server-paginated modal — one line per
// checked product. So the same dialog is also how a line is removed (re-open,
// untick, Done), which is what makes the check set the source of truth.
//
// Seeds its own product over the Connect JSON API (fast + deterministic, same
// approach as restock.spec.ts) so it does not depend on dev-DB contents.

async function seedProduct(page: Page, marker: string): Promise<string> {
  await page.goto("/");
  return await page.evaluate(async (m: string) => {
    const token = localStorage.getItem("justmart_access_token");
    if (!token) throw new Error("no access token in localStorage");
    const res = await fetch("/api/inventory_iface.v1.ProductService/CreateProduct", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({
        sku: `PICK-${m}`,
        name: `Picker Med ${m}`,
        unit: "tablet",
        unitPrice: "700",
        units: [
          { name: "box", factor: "100", sellPrice: "60000", sellable: true, purchasable: true },
        ],
      }),
    });
    if (!res.ok) throw new Error(`CreateProduct: ${res.status} ${await res.text()}`);
    return (await res.json()).product.id as string;
  }, marker);
}

async function archiveProduct(page: Page, id: string): Promise<void> {
  await page.evaluate(async (productId: string) => {
    const token = localStorage.getItem("justmart_access_token");
    if (!token) return;
    await fetch("/api/inventory_iface.v1.ProductService/ArchiveProduct", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ id: productId }),
    });
  }, id);
}

test.describe("product picker dialog", () => {
  test("search → check → Done adds a line; re-open → untick removes it", async ({ page }) => {
    const m = String(Date.now());
    const name = `Picker Med ${m}`;
    const productId = await seedProduct(page, m);
    try {
      await page.goto("/purchasing/new");

      // Nothing is picked yet, so the line table shows its empty state.
      await expect(page.getByRole("cell", { name: /No .* yet/ })).toBeVisible();

      // The button is glossary-aware: "Add product" in retail, "Add medicine"
      // in pharmacy mode.
      const addButton = page.getByRole("button", { name: /^Add (product|medicine)$/ });
      await addButton.click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();

      // Server-side search, debounced 250ms.
      await dialog.getByPlaceholder(/Search by name or SKU/i).fill(name);
      await page.waitForTimeout(700);
      await expect(dialog.getByRole("row", { name: new RegExp(name) })).toBeVisible();

      // Clicking the row toggles its checkbox; the footer counts the draft.
      await dialog.getByRole("row", { name: new RegExp(name) }).click();
      await expect(dialog.getByText(/1 selected/)).toBeVisible();
      await dialog.getByRole("button", { name: "Done" }).click();
      await expect(dialog).toBeHidden();

      // The line carries name + SKU as a static label (the product is chosen in
      // the dialog, not on the row) and qty defaulted to 1. The unit cell is a
      // combobox because this product has 2 purchasable units — its value is
      // seeded to the BASE unit from the picked Product, which is the part the
      // dialog is responsible for. (Combobox values don't reach the row name,
      // hence the separate assertion.)
      const line = page.getByRole("row", { name: new RegExp(`${name} PICK-${m} 1`) });
      await expect(line).toBeVisible();
      await expect(line.getByRole("combobox").first()).toHaveText("tablet");

      // Re-opening pre-checks what the form already holds…
      await addButton.click();
      await expect(dialog).toBeVisible();
      await expect(dialog.getByText(/1 selected/)).toBeVisible();

      // …and unticking is how the line is removed.
      await dialog.getByPlaceholder(/Search by name or SKU/i).fill(name);
      await page.waitForTimeout(700);
      await dialog.getByRole("row", { name: new RegExp(name) }).click();
      await expect(dialog.getByText(/0 selected/)).toBeVisible();
      await dialog.getByRole("button", { name: "Done" }).click();
      await expect(dialog).toBeHidden();
      await expect(page.getByRole("cell", { name: /No .* yet/ })).toBeVisible();
    } finally {
      await archiveProduct(page, productId);
    }
  });

  test("Cancel discards the draft selection", async ({ page }) => {
    const m = String(Date.now());
    const name = `Picker Med ${m}`;
    const productId = await seedProduct(page, m);
    try {
      await page.goto("/purchasing/new");
      const addButton = page.getByRole("button", { name: /^Add (product|medicine)$/ });
      await addButton.click();
      const dialog = page.getByRole("dialog");
      await dialog.getByPlaceholder(/Search by name or SKU/i).fill(name);
      await page.waitForTimeout(700);
      await dialog.getByRole("row", { name: new RegExp(name) }).click();
      await expect(dialog.getByText(/1 selected/)).toBeVisible();

      await dialog.getByRole("button", { name: "Cancel" }).click();
      await expect(dialog).toBeHidden();

      // No line was added, and the next open re-seeds from the form (not the
      // abandoned draft).
      await expect(page.getByRole("cell", { name: /No .* yet/ })).toBeVisible();
      await addButton.click();
      await expect(dialog.getByText(/0 selected/)).toBeVisible();
    } finally {
      await archiveProduct(page, productId);
    }
  });
});
