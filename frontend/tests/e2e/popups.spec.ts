import type { Page } from "@playwright/test";

import { CATALOG_NOUN_RE, expect, test } from "./_helpers";

// Seed fixtures over the wire (same shape as medicines/grosir/restock specs) so
// a test only drives the UI it actually asserts on.
async function api<T = unknown>(page: Page, path: string, body: unknown): Promise<T> {
  return await page.evaluate(
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
  ) as Promise<T>;
}

test.describe("EntityDrawer (slide-over)", () => {
  test("Customers Add → Cancel closes the drawer", async ({ page }) => {
    await page.goto("/customers");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await expect(drawer).toBeVisible();
    await expect(drawer.getByRole("heading", { name: "Add customer" })).toBeVisible();
    await drawer.getByRole("button", { name: "Cancel" }).click();
    await expect(drawer).toBeHidden();
  });

  test("Customers Add → Save creates a row and closes", async ({ page }) => {
    // Unique name per run so suites stay independent.
    const name = `e2e-customer-${Date.now()}`;
    await page.goto("/customers");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    await drawer.getByRole("textbox", { name: "Name" }).fill(name);
    await drawer.getByRole("button", { name: "Save" }).click();
    await expect(drawer).toBeHidden();
    await expect(page.getByRole("cell", { name })).toBeVisible();

    // Cleanup: archive the row so future runs stay clean.
    const row = page.getByRole("row", { name: new RegExp(name) });
    await row.getByRole("button", { name: "Archive" }).click();
  });

  test("Warehouses Add → required fields are enforced on submit", async ({ page }) => {
    await page.goto("/warehouses");
    await page.getByRole("button", { name: "Add" }).click();
    const drawer = page.getByRole("dialog");
    const save = drawer.getByRole("button", { name: "Save" });

    // The app does NOT gate Save on form validity (no `isValid` anywhere) —
    // per the validation HARD RULE, submitting runs the Zod schema and
    // <FormField> renders the message under the offending field. Submitting an
    // empty form must therefore keep the drawer open and show an error.
    await save.click();
    await expect(drawer).toBeVisible();
    await expect(drawer.getByText(/required|wajib/i).first()).toBeVisible();

    const inputs = drawer.locator("input");
    await inputs.nth(0).fill("E2E01"); // code
    await inputs.nth(1).fill("E2E warehouse"); // name

    await drawer.getByRole("button", { name: "Cancel" }).click();
    await expect(drawer).toBeHidden();
  });

  // The drawer's useForm lives in the component that RENDERS <EntityDrawer>, so
  // it survives close and RHF keeps values in a ref — every drawer resets on the
  // open edge via useResetOnOpen (lib/formReset.ts). These two pin both halves.
  test("EntityDrawer resets form state when re-opened after Cancel", async ({ page }) => {
    await page.goto("/customers");
    await page.getByRole("button", { name: "Add" }).click();
    await page
      .getByRole("dialog")
      .getByRole("textbox", { name: "Name" })
      .fill("Ghost dummy");
    await page.getByRole("dialog").getByRole("button", { name: "Cancel" }).click();
    await page.getByRole("button", { name: "Add" }).click();
    const value = await page
      .getByRole("dialog")
      .getByRole("textbox", { name: "Name" })
      .inputValue();
    expect(value).toBe("");
  });

  test("EntityDrawer discards an abandoned EDIT when re-opened on the same record", async ({
    page,
  }) => {
    // The edit path needs its own cover: RHF's `values` prop only re-syncs when
    // the object changes, so re-opening the SAME record is deep-equal and would
    // silently keep the abandoned draft.
    await page.goto("/warehouses");
    // No "0" prefix: this spec finds its row by search, and a code that sorts to
    // the very top would compete to be the setup's fallback warehouse.
    const code = `RST${Date.now() % 1000000}`;
    const originalName = "Reset gudang";
    // Seed over the API, not the Add drawer: the fixture is not what's under
    // test here, and four extra UI steps is four extra things to flake on.
    const created = await api<{ warehouse: { id: string } }>(
      page,
      "warehouse_iface.v1.WarehouseService/CreateWarehouse",
      { code, name: originalName },
    );

    await page.goto(`/warehouses/${created.warehouse.id}`);

    // Type a new name, then abandon via Cancel.
    await page.getByRole("button", { name: /^Edit$|^Ubah$/ }).click();
    const editDrawer = page.getByRole("dialog");
    const editName = editDrawer.locator("input").nth(1);
    // The drawer mounts before the GetWarehouse seed lands, so wait for the
    // pre-fill — typing into the pre-seed input races the re-seed remount.
    await expect(editName).toHaveValue(originalName, { timeout: 15_000 });
    await editName.fill("Abandoned rename");
    await editDrawer.getByRole("button", { name: /^Cancel$|^Batal$/ }).click();
    await expect(editDrawer).toBeHidden();

    // Re-open the SAME record — the field must show the saved name again.
    await page.getByRole("button", { name: /^Edit$|^Ubah$/ }).click();
    await expect(editDrawer).toBeVisible();
    await expect(editDrawer.locator("input").nth(1)).toHaveValue(originalName);
    await editDrawer.getByRole("button", { name: /^Cancel$|^Batal$/ }).click();
  });
});

test.describe("Dialog (centered modal)", () => {
  test("F4 on POS opens the customer picker and auto-focuses the search", async ({ page }) => {
    await page.goto("/pos");
    // StartSale fires on mount; give it a beat so the page is interactive.
    await page.waitForLoadState("networkidle");
    await page.keyboard.press("F4");
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Attach customer" })).toBeVisible();
    // Search input is auto-focused.
    const focusedPlaceholder = await page.evaluate(
      () => (document.activeElement as HTMLInputElement | null)?.placeholder ?? "",
    );
    expect(focusedPlaceholder).toMatch(/search/i);
  });

  test("Dialog dismisses via Escape, close button, and outside-positioner click", async ({
    page,
  }) => {
    await page.goto("/pos");
    await page.waitForLoadState("networkidle");

    // 1. Escape closes
    await page.keyboard.press("F4");
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toBeHidden();

    // 2. Close [×] button closes
    await page.keyboard.press("F4");
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByRole("dialog").getByRole("button", { name: "close" }).click();
    await expect(page.getByRole("dialog")).toBeHidden();

    // 3. Outside-positioner click closes. The visible "backdrop" is shielded
    // by the positioner div which is what Chakra listens on for outside
    // clicks; we synthesize the pointer events at coordinates outside the
    // content card.
    await page.keyboard.press("F4");
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.evaluate(() => {
      const target = document.elementFromPoint(50, 500) as HTMLElement | null;
      const opts = { bubbles: true, clientX: 50, clientY: 500, button: 0 };
      target?.dispatchEvent(new PointerEvent("pointerdown", opts));
      target?.dispatchEvent(new MouseEvent("mousedown", opts));
      target?.dispatchEvent(new MouseEvent("mouseup", opts));
      target?.dispatchEvent(new PointerEvent("pointerup", opts));
      target?.dispatchEvent(new MouseEvent("click", opts));
    });
    await expect(page.getByRole("dialog")).toBeHidden();
  });
});

test.describe("RouteTabs (Chakra Tabs + NavLink)", () => {
  test("Analytics tab strip is Chakra Tabs and clicking changes the URL", async ({ page }) => {
    await page.goto("/analytics/daily");

    // Hard rule: the tab strip is a Chakra Tabs.List, not a hand-rolled
    // NavLink row. The accessibility tree exposes [role=tablist] +
    // [role=tab] only when the Chakra primitive is in use. Daily now has an
    // INNER tab strip (Table | Graph) on top of the outer Analytics strip;
    // scope to the outer one via `.first()` (DOM order).
    const tablist = page.getByRole("tablist").first();
    await expect(tablist).toBeVisible();
    await expect(tablist.getByRole("tab")).toHaveCount(3);

    // The active tab matches the URL.
    await expect(page.getByRole("tab", { name: "Daily" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    // Tag the live document. A full-page reload would wipe this; SPA navigation
    // preserves it. Guards the RouteTabs full-reload regression (tabs must
    // navigate client-side, not follow an <a href>).
    await page.evaluate(() => {
      (window as unknown as { __noReload?: boolean }).__noReload = true;
    });

    // Clicking a tab updates the URL (no full reload) and shifts active state.
    // The middle tab is labelled with the mode-aware catalog noun.
    await page.getByRole("tab", { name: CATALOG_NOUN_RE }).click();
    await expect(page).toHaveURL(/\/analytics\/product$/);
    await expect(page.getByRole("tab", { name: CATALOG_NOUN_RE })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    // The marker survived → no full document reload occurred.
    const survived = await page.evaluate(
      () => (window as unknown as { __noReload?: boolean }).__noReload === true,
    );
    expect(survived).toBe(true);
  });
});
