import { expect, test } from "./_helpers";

// Pembelian (Purchasing) UI refactor: status tabs + search on the list, and the
// create form's line-total input. Backend coverage is in
// backend/e2e/purchasing_test.go.
test.describe("purchasing", () => {
  test("status tabs switch the list + URL, search box is present", async ({ page }) => {
    await page.goto("/purchasing/all");

    // Status tab row includes Draft and a Pemasok (Suppliers) tab.
    const draftTab = page.getByRole("tab", { name: "Draft" });
    await expect(draftTab).toBeVisible();
    await expect(page.getByRole("tab", { name: "Suppliers" })).toBeVisible();

    await draftTab.click();
    await page.waitForURL(/\/purchasing\/draft$/);

    // The search box drives the new server-side PO search.
    await expect(page.getByPlaceholder(/Search PO/i)).toBeVisible();
  });

  test("create PO shows a per-item cost input and the derived per-base cost", async ({ page }) => {
    await page.goto("/purchasing/new");
    // The line cost column is entered PER ITEM ("Cost / item (IDR)" — it was
    // once the line total, hence the old "Total cost" label); the derived
    // per-base cost shows as "Cost / base unit" (relabelled when buy-in-units
    // landed). Both headers render even with no lines picked yet.
    await expect(page.getByText("Cost / item (IDR)")).toBeVisible();
    await expect(page.getByText("Cost / base unit")).toBeVisible();
  });
});
