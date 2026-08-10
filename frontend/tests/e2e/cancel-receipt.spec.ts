import type { Page } from "@playwright/test";

import { expect, test } from "./_helpers";

// Cancelling an accepted restock ("batal terima"), driven through the UI.
//
// The Go handler tests cover the rules exhaustively; what only a browser can
// prove is the wiring — that the row button reaches CancelReceipt, that the
// reason gates submission, and that a lot which has been touched offers no
// button at all. The setup (PO → send → receive) goes through the Connect JSON
// API because it is not what is under test here; restock.spec.ts already walks
// that flow by hand.

type Seed = { productId: string; supplierId: string; poId: string; receiptItemId: string };

async function seed(page: Page, marker: string): Promise<Seed> {
  await page.goto("/");
  return await page.evaluate(async (m: string) => {
    const token = localStorage.getItem("justmart_access_token");
    if (!token) throw new Error("no access token in localStorage");
    const headers = { "Content-Type": "application/json", Authorization: `Bearer ${token}` };
    const post = async (path: string, body: unknown) => {
      const res = await fetch(`/api/${path}`, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`);
      return res.json();
    };

    const prod = await post("inventory_iface.v1.ProductService/CreateProduct", {
      sku: `CR-${m}`,
      name: `Cancel Med ${m}`,
      unit: "tablet",
      unitPrice: "700",
    });
    const sup = await post("inventory_iface.v1.SupplierService/CreateSupplier", {
      code: `CR${m}`,
      name: `Cancel Supplier ${m}`,
    });

    const po = await post("purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder", {
      supplierId: sup.supplier.id,
      items: [{ productId: prod.product.id, orderedQty: 10, unitCostPrice: "500" }],
    });
    await post("purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: po.order.id });

    const rcpt = await post("purchasing_iface.v1.PurchaseReceiptService/CreateReceipt", {
      purchaseOrderId: po.order.id,
      lines: [
        {
          purchaseOrderItemId: po.order.items[0].id,
          qty: 10,
          batchNumber: `CB-${m}`,
          expiryDate: "2099-12-31",
        },
      ],
    });

    return {
      productId: prod.product.id,
      supplierId: sup.supplier.id,
      poId: po.order.id,
      receiptItemId: rcpt.receipt.items[0].id,
    };
  }, marker);
}

// voidPo: only pass true when the PO is actually voidable (DRAFT/SENT). Voiding
// a PARTIALLY_RECEIVED / RECEIVED one answers 400, and the shared fixture fails
// any test that logs a console error — including from teardown. The PO row is
// timestamp-unique either way, so leaving it costs nothing.
async function cleanup(page: Page, s: Seed, opts: { voidPo: boolean }): Promise<void> {
  await page.evaluate(
    async (a: { ids: Seed; voidPo: boolean }) => {
      const token = localStorage.getItem("justmart_access_token");
      if (!token) return;
      const headers = { "Content-Type": "application/json", Authorization: `Bearer ${token}` };
      const post = (path: string, body: unknown) =>
        fetch(`/api/${path}`, { method: "POST", headers, body: JSON.stringify(body) });
      if (a.voidPo) {
        await post("purchasing_iface.v1.PurchaseOrderService/VoidPurchaseOrder", {
          id: a.ids.poId,
        });
      }
      await post("inventory_iface.v1.SupplierService/ArchiveSupplier", { id: a.ids.supplierId });
      await post("inventory_iface.v1.ProductService/ArchiveProduct", { id: a.ids.productId });
    },
    { ids: s, voidPo: opts.voidPo },
  );
}

test.describe("cancel an accepted restock", () => {
  test("cancels an untouched receipt and reopens the restock order", async ({ page }) => {
    const m = String(Date.now());
    const ids = await seed(page, m);
    try {
      await page.goto(`/purchasing/${ids.poId}`);
      // Fully received, so the PO badge reads Received before we start.
      await expect(page.getByText("Received", { exact: true }).first()).toBeVisible();

      // The row action. Same label as the dialog's confirm button, so scope to
      // the page (the dialog is not open yet).
      await page.getByRole("button", { name: "Cancel receipt", exact: true }).click();

      const dialog = page.getByRole("dialog");
      await expect(dialog).toBeVisible();

      // Confirm is gated on a reason — this is the guard the handler also has.
      const confirm = dialog.getByRole("button", { name: "Cancel receipt", exact: true });
      await expect(confirm).toBeDisabled();

      await dialog.getByRole("textbox").last().fill("salah input e2e");
      await expect(confirm).toBeEnabled();
      await confirm.click();
      await expect(dialog).not.toBeVisible();

      // The receipt stays listed, marked cancelled, carrying its reason.
      await expect(page.getByText("Cancelled", { exact: true })).toBeVisible();
      await expect(page.getByText(/salah input e2e/)).toBeVisible();

      // received_qty was given back, so the order is awaiting delivery again.
      await expect(page.getByText("Sent", { exact: true }).first()).toBeVisible();

      // And the action is gone — nothing left to cancel.
      await expect(
        page.getByRole("button", { name: "Cancel receipt", exact: true }),
      ).toHaveCount(0);
    } finally {
      await cleanup(page, ids, { voidPo: true });
    }
  });

  test("offers no cancel once the lot has been touched, and says why", async ({ page }) => {
    const m = String(Date.now() + 1);
    const ids = await seed(page, m);
    try {
      // Return one unit: that writes a movement against the lot, so the receipt
      // is no longer an untouched delivery.
      await page.evaluate(async (s: Seed) => {
        const token = localStorage.getItem("justmart_access_token");
        const headers = { "Content-Type": "application/json", Authorization: `Bearer ${token}` };
        const res = await fetch("/api/purchasing_iface.v1.PurchaseReturnService/CreatePurchaseReturn", {
          method: "POST",
          headers,
          body: JSON.stringify({
            purchaseOrderId: s.poId,
            reason: "rusak",
            lines: [{ purchaseReceiptItemId: s.receiptItemId, qty: 1 }],
          }),
        });
        if (!res.ok) throw new Error(`return: ${res.status} ${await res.text()}`);
      }, ids);

      await page.goto(`/purchasing/${ids.poId}`);

      // No dead button to click…
      await expect(
        page.getByRole("button", { name: "Cancel receipt", exact: true }),
      ).toHaveCount(0);
      // …the reason is shown instead, pointing at a purchase return.
      await expect(page.getByText(/already been used/i)).toBeVisible();
    } finally {
      await cleanup(page, ids, { voidPo: false });
    }
  });
});
