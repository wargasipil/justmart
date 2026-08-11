import { test } from "@playwright/test";

import {
  BEAT,
  card,
  click,
  hush,
  openBackstage,
  openStage,
  rpc,
  rpcQuiet,
  say,
  spotlight,
  type,
  unspotlight,
  videoDest,
} from "./_recorder";

// Recorder for: faq/videos/can-an-accepted-restock-be-cancelled/
//
// "Penerimaan restock yang sudah diterima, bisa dibatalkan tidak?"
//
// The answer has two halves and the video shows both, because showing only the
// happy path would teach the wrong rule: a receipt can be cancelled while the
// lot it created is still untouched, and NOT once any of that stock has moved.
// The second scenario is seeded by returning a single unit — the cheapest way
// to write a movement against the lot — so the viewer sees the button replaced
// by the reason rather than just being told it can happen.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=can-an-accepted-restock`.

const SLUG = "can-an-accepted-restock-be-cancelled";

type Seed = { productId: string; supplierId: string; poId: string; receiptItemId: string };

/** A restock order, sent and fully received — i.e. an accepted delivery. */
async function seedAcceptedRestock(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  opts: { marker: string; product: string; sku: string; supplier: string; code: string; batch: string },
): Promise<Seed> {
  const prod = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: opts.sku,
    name: opts.product,
    unit: "tablet",
    unitPrice: "1500",
  });
  const sup = await rpc(page, "inventory_iface.v1.SupplierService/CreateSupplier", {
    code: opts.code,
    name: opts.supplier,
  });
  const po = await rpc(page, "purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder", {
    supplierId: sup.supplier.id,
    items: [{ productId: prod.product.id, orderedQty: 60, unitCostPrice: "900" }],
  });
  await rpc(page, "purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: po.order.id });
  const rcpt = await rpc(page, "purchasing_iface.v1.PurchaseReceiptService/CreateReceipt", {
    purchaseOrderId: po.order.id,
    lines: [
      {
        purchaseOrderItemId: po.order.items[0].id,
        qty: 60,
        batchNumber: opts.batch,
        expiryDate: "2028-06-30",
      },
    ],
  });

  return {
    productId: prod.product.id,
    supplierId: sup.supplier.id,
    poId: po.order.id,
    receiptItemId: rcpt.receipt.items[0].id,
  };
}

// voidPo only where the PO is actually voidable (DRAFT/SENT); a
// PARTIALLY_RECEIVED one answers 400. The rows are marker-unique either way, so
// leaving one behind costs nothing.
async function cleanup(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  s: Seed,
  opts: { voidPo: boolean },
): Promise<void> {
  if (opts.voidPo) {
    await rpcQuiet(page, "purchasing_iface.v1.PurchaseOrderService/VoidPurchaseOrder", { id: s.poId });
  }
  await rpcQuiet(page, "inventory_iface.v1.SupplierService/ArchiveSupplier", { id: s.supplierId });
  await rpcQuiet(page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: s.productId });
}

test("record: bisakah penerimaan restock dibatalkan", async ({ browser }, testInfo) => {
  test.setTimeout(5 * 60_000);

  const backstage = await openBackstage(browser);
  const marker = String(Date.now()).slice(-6);

  // A: the delivery we will cancel — nothing has touched its lot.
  const clean = await seedAcceptedRestock(backstage.page, {
    marker,
    product: "Paracetamol 500 mg",
    sku: `FAQ-PCT-${marker}`,
    supplier: "PT Sumber Sehat Farma",
    code: `SSF${marker}`,
    batch: "B-2604-118",
  });

  // B: an identical delivery whose lot HAS moved. One returned unit is enough —
  // it writes a movement against the batch, which is exactly what the cancel
  // guard looks for.
  const used = await seedAcceptedRestock(backstage.page, {
    marker: `${marker}b`,
    product: "Amoxicillin 500 mg",
    sku: `FAQ-AMX-${marker}`,
    supplier: "PT Anugerah Medika",
    code: `AGM${marker}`,
    batch: "B-2604-207",
  });
  await rpc(backstage.page, "purchasing_iface.v1.PurchaseReturnService/CreatePurchaseReturn", {
    purchaseOrderId: used.poId,
    reason: "1 strip rusak",
    lines: [{ purchaseReceiptItemId: used.receiptItemId, qty: 1 }],
  });

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;

  try {
    // ---- Opening -----------------------------------------------------------
    // Card first, THEN wait for the data — the page finishes loading behind it,
    // so the recording opens on the title instead of on a half-painted app.
    await page.goto(`/purchasing/${clean.poId}`);
    await card(
      page,
      "Penerimaan restock sudah terlanjur dicatat. Bisa dibatalkan?",
      "Bisa — selama stoknya belum terpakai sama sekali.",
      BEAT.card,
    );
    await page.getByText("Diterima", { exact: true }).first().waitFor();

    // ---- 1. Where you are --------------------------------------------------
    await say(
      page,
      "Ini order restock yang barangnya <b>sudah diterima</b>.",
      BEAT.read,
    );
    await spotlight(page, page.getByText("Diterima", { exact: true }).first());
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    // ---- 2. The affordance -------------------------------------------------
    const cancelBtn = page.getByRole("button", { name: "Batalkan", exact: true });
    await say(
      page,
      "Di bagian <b>Tanda terima</b>, penerimaan yang stoknya masih utuh punya tombol <b>Batalkan</b>.",
      BEAT.dwell,
    );
    await spotlight(page, cancelBtn, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, cancelBtn);

    // ---- 3. The confirmation -----------------------------------------------
    const dialog = page.getByRole("dialog");
    await dialog.waitFor();
    await say(
      page,
      "Dialognya menyebut <b>persis stok mana</b> yang akan dihapus dari gudang.",
      BEAT.dwell,
    );

    await say(
      page,
      "Tombol konfirmasi <b>terkunci sampai alasannya diisi</b> — alasan ini ikut tersimpan.",
      BEAT.read,
    );
    const confirm = dialog.getByRole("button", { name: "Batalkan penerimaan", exact: true });
    await spotlight(page, confirm, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await type(page, dialog.getByRole("textbox").last(), "Salah input, barang belum datang");
    await hush(page);
    await click(page, confirm);
    await dialog.waitFor({ state: "hidden" });

    // ---- 4. What actually happened ----------------------------------------
    await page.getByText("Dibatalkan", { exact: true }).first().waitFor();
    await say(
      page,
      "Stoknya hilang dari gudang, tapi <b>dokumennya tetap ada</b> — ditandai Dibatalkan beserta alasannya.",
      BEAT.dwell,
    );
    await spotlight(page, page.getByText(/Salah input, barang belum datang/).first());
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(
      page,
      "Dan order restoknya <b>kembali ke status Dikirim</b> — dianggap menunggu barang lagi.",
      BEAT.dwell,
    );
    await spotlight(page, page.getByText("Dikirim", { exact: true }).first());
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 5. When you can't -------------------------------------------------
    await page.goto(`/purchasing/${used.poId}`);
    await page.getByText(/sudah terpakai/i).first().waitFor();

    await say(
      page,
      "Tapi kalau stoknya <b>sudah bergerak</b> — terjual, dipindah, diretur, atau kena opname…",
      BEAT.dwell,
    );
    await say(page, "…tombol Batalkan <b>tidak muncul sama sekali</b>.", BEAT.read);
    await spotlight(page, page.getByText(/sudah terpakai/i).first());
    await say(
      page,
      "Alasannya ditulis di tempat tombolnya. Untuk barang yang memang dikembalikan ke pemasok, pakai <b>Retur</b>.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Batalkan = salah catat. Retur = barang benar-benar dikembalikan.",
      "Batalkan hanya bisa selama stoknya belum tersentuh, dan hanya untuk Owner / Admin.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    await cleanup(backstage.page, clean, { voidPo: true });
    await cleanup(backstage.page, used, { voidPo: false });
    await backstage.context.close();
  }
});
