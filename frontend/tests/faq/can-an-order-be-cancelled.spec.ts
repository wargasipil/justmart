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

// Recorder for: faq/videos/can-an-order-be-cancelled/
//
// "Pesanan yang sudah selesai, bisa dibatalkan?"
//
// Yes — Refund, within a day, Owner/Admin only, once. Three things the take has
// to show rather than claim:
//
//   1. the order is NOT deleted — it stays in the history marked Dikembalikan,
//      with the reason, which is the half people expect to be a delete;
//   2. the stock really does come back — filmed as a before/after on the
//      product's own "Siap" tile (197 of 200 sold, then 200 again), because a
//      caption saying so proves nothing;
//   3. the restock switch is a real choice — a second order is refunded with it
//      OFF, and the app itself says "Tidak dikembalikan".
//
// The 1-day window and the role limit are stated, not filmed: both are an
// ABSENT button, which on screen is indistinguishable from any other reason to
// be absent. They live in question.md's blocked-reason table instead.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=can-an-order-be-cancelled`.
//
// Residue: a REFUNDED sale is terminal — it can be neither voided nor
// discarded — so two small sales stay in the dev DB per run. Their products are
// archived on the way out and their SKUs carry the run marker.

const SLUG = "can-an-order-be-cancelled";

type Product = { id: string; name: string; price: number };
type Order = { id: string; saleNo: string };

async function createProduct(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  opts: { name: string; sku: string; unit: string; price: number; stock: number; batch: string; cost: number },
): Promise<Product> {
  const res = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: opts.sku,
    name: opts.name,
    unit: opts.unit,
    unitPrice: String(opts.price),
  });
  const id = res.product.id as string;
  await rpc(page, "inventory_iface.v1.BatchService/CreateBatch", {
    productId: id,
    batchNumber: opts.batch,
    expiryDate: "2028-09-30",
    costPrice: String(opts.cost),
    initialQuantity: String(opts.stock),
  });
  return { id, name: opts.name, price: opts.price };
}

/** A finished sale, exactly as the till would leave it: DRAFT -> COMPLETED. */
async function sell(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  product: Product,
  qty: number,
  paid: number,
): Promise<Order> {
  const started = await rpc(page, "pos_iface.v1.SaleService/StartSale", {});
  const saleId = started.sale.id as string;
  await rpc(page, "pos_iface.v1.SaleService/AddItem", { saleId, productId: product.id, qty });
  // Enum by NAME: Connect JSON answers with the name, and the numeric form is
  // easy to get wrong for a field whose zero value is UNSPECIFIED.
  const done = await rpc(page, "pos_iface.v1.SaleService/CompleteSale", {
    saleId,
    paymentSource: "PAYMENT_SOURCE_CASH",
    paidAmount: String(paid),
  });
  return { id: saleId, saleNo: done.sale.saleNo as string };
}

test("record: bisakah pesanan yang sudah selesai dibatalkan", async ({ browser }, testInfo) => {
  test.setTimeout(6 * 60_000);

  const backstage = await openBackstage(browser);
  const marker = String(Date.now()).slice(-6);

  // A: sold from a 200-tablet lot, refunded WITH restock — the before/after
  // numbers (197 -> 200) are what make the stock claim checkable on screen.
  const para = await createProduct(backstage.page, {
    name: "Paracetamol 500 mg",
    sku: `FAQ-RFD-PCT-${marker}`,
    unit: "tablet",
    price: 1500,
    stock: 200,
    batch: `B-RFD-${marker}`,
    cost: 900,
  });
  // B: refunded money-only. A broken bottle is the everyday reason the goods
  // do not come back, so the switch has an obvious motive rather than being a
  // toggle demonstrated for its own sake.
  const sirup = await createProduct(backstage.page, {
    name: "Sirup Obat Batuk 60 ml",
    sku: `FAQ-RFD-SRP-${marker}`,
    unit: "botol",
    price: 18000,
    stock: 40,
    batch: `B-RFD-S${marker}`,
    cost: 11000,
  });

  const orderA = await sell(backstage.page, para, 3, 5000);
  const orderB = await sell(backstage.page, sirup, 1, 20000);

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // The caption bar and the spotlight ring are real DOM outside the app, and a
  // caption naming a button would otherwise make getByText ambiguous with the
  // very element it is about to ring.
  const app = page.locator("#root");

  try {
    // ---- Opening -----------------------------------------------------------
    // Card first, THEN wait for the data — the page finishes loading behind it,
    // so the recording opens on the title instead of on a half-painted app.
    await page.goto(`/products/${para.id}`);
    await card(
      page,
      "Pesanan yang sudah selesai, bisa dibatalkan?",
      "Bisa — lewat Refund, maksimal 1 hari setelah transaksi.",
      BEAT.card,
    );
    const readyTile = app.getByText("Siap", { exact: true }).first().locator("..");
    await readyTile.waitFor();

    // ---- 1. The stock before ----------------------------------------------
    await say(
      page,
      "Sebelum mulai: <b>Paracetamol</b> punya 200 tablet, dan 3 di antaranya baru saja terjual.",
      BEAT.read,
    );
    // Ring FIRST, then say the number: spotlight() scrolls the tile into view,
    // so captioning first spends the whole caption on a figure that is still
    // below the fold.
    await spotlight(page, readyTile, 6);
    await say(page, "Jadi stok siapnya sekarang <b>197</b>. Ingat angka ini.", BEAT.dwell);
    await unspotlight(page);
    await hush(page);

    // ---- 2. Finding the order ---------------------------------------------
    // Search first, caption second. The dev shop's history is full of rows left
    // by other recorders and by the e2e suite; filtering before dwelling keeps
    // that out of frame instead of holding a caption over it.
    await page.goto("/orders");
    await type(page, page.getByPlaceholder(/Cari no penjualan/i).first(), orderA.saleNo);
    await say(page, "Transaksinya ada di <b>Riwayat order</b> — cari nomornya.", BEAT.read);
    const row = page.getByRole("row").filter({ hasText: orderA.saleNo }).first();
    await row.waitFor();
    await click(page, row);

    // ---- 3. What you are allowed to do ------------------------------------
    const refundBtn = page.getByRole("button", { name: "Refund", exact: true });
    await refundBtn.waitFor();
    await say(page, "Statusnya <b>Selesai</b> — transaksinya sudah jadi penjualan.", BEAT.read);
    await spotlight(page, app.getByText("Selesai", { exact: true }).first(), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(
      page,
      "Tombol <b>Refund</b> inilah cara membatalkannya — hanya untuk <b>Owner</b> dan <b>Admin</b>, dan hanya sampai <b>1 hari</b> setelah transaksi.",
      BEAT.dwell,
    );
    await spotlight(page, refundBtn, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, refundBtn);

    // ---- 4. The dialog, and the one real decision --------------------------
    // ConfirmDialog is role="alertdialog", not "dialog" (see CLAUDE.md).
    const dialog = page.getByRole("alertdialog");
    await dialog.waitFor();
    await say(page, "Refund selalu <b>satu pesanan penuh</b> — tidak bisa sebagian.", BEAT.dwell);
    await say(page, "<b>Alasan</b> ikut tersimpan di pesanannya.", BEAT.read);
    await type(page, dialog.getByRole("textbox").first(), "Pelanggan berubah pikiran");
    await hush(page);

    await say(
      page,
      "Ini keputusan pentingnya: <b>Kembalikan barang ke stok</b>. Nyala berarti barangnya masuk lagi ke rak.",
      BEAT.dwell,
    );
    await spotlight(page, dialog.getByText("Kembalikan barang ke stok"), 8);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, dialog.getByRole("button", { name: "Refund pesanan" }));
    await dialog.waitFor({ state: "hidden" });

    // ---- 5. The order is not deleted --------------------------------------
    await app.getByText("Dikembalikan", { exact: true }).first().waitFor();
    await say(
      page,
      "Pesanannya <b>tidak dihapus</b>. Nomornya tetap ada, statusnya jadi <b>Dikembalikan</b>.",
      BEAT.dwell,
    );
    await spotlight(page, app.getByText("Dikembalikan", { exact: true }).first(), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(
      page,
      "Waktu, jumlah, alasan, dan nasib barangnya <b>tercatat di pesanan itu sendiri</b>.",
      BEAT.dwell,
    );
    await spotlight(page, app.getByText("Kembali ke stok", { exact: true }).first(), 8);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(
      page,
      "Tombol <b>Refund</b> dan <b>Cetak struk</b> hilang — satu pesanan hanya bisa di-refund <b>sekali</b>.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- 6. The stock after ------------------------------------------------
    await page.goto(`/products/${para.id}`);
    const readyAfter = app.getByText("Siap", { exact: true }).first().locator("..");
    await readyAfter.waitFor();
    await spotlight(page, readyAfter, 6);
    await say(page, "Dan stoknya benar-benar kembali: <b>197 jadi 200</b>.", BEAT.dwell);
    await say(
      page,
      "Barangnya masuk ke <b>batch asalnya</b>, jadi tanggal kedaluwarsanya tetap benar.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- 7. When the goods do NOT come back --------------------------------
    await page.goto(`/orders/${orderB.id}`);
    const refundB = page.getByRole("button", { name: "Refund", exact: true });
    await refundB.waitFor();
    await say(
      page,
      "Pesanan kedua: <b>sirup obat batuk</b>, botolnya pecah. Uangnya dikembalikan, barangnya tidak.",
      BEAT.dwell,
    );
    await click(page, refundB);

    const dialogB = page.getByRole("alertdialog");
    await dialogB.waitFor();
    await type(page, dialogB.getByRole("textbox").first(), "Botol pecah, barang tidak kembali");
    await say(page, "Untuk kasus ini, <b>matikan</b> sakelarnya.", BEAT.read);
    await click(page, dialogB.getByText("Kembalikan barang ke stok"));
    // Sync point, not an assertion: if the toggle did not land, the take would
    // narrate a money-only refund over one that quietly restocked — exactly the
    // kind of lie this format exists to prevent. Chakra's Switch input is
    // display:none, so it is read from the DOM rather than located.
    await page.waitForFunction(() => {
      const el = document.querySelector<HTMLInputElement>('[role="alertdialog"] input[type="checkbox"]');
      return el != null && !el.checked;
    });
    await page.waitForTimeout(BEAT.short);
    await hush(page);
    await click(page, dialogB.getByRole("button", { name: "Refund pesanan" }));
    await dialogB.waitFor({ state: "hidden" });

    await app.getByText("Tidak dikembalikan", { exact: true }).first().waitFor();
    await say(
      page,
      "Statusnya tetap <b>Dikembalikan</b>, tapi stoknya tidak disentuh — aplikasinya menuliskannya sendiri.",
      BEAT.dwell,
    );
    await spotlight(page, app.getByText("Tidak dikembalikan", { exact: true }).first(), 8);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 8. Where they live afterwards -------------------------------------
    // The claim has to be the one this screen actually supports. An earlier take
    // said refunds "disappear from the revenue reports" while standing on the
    // Dikembalikan tab — whose summary bar dutifully totals the refunded orders
    // (Rp 22.500) directly above the caption. Said on the Selesai tab, where
    // they really are gone from both the rows and the total, it is the same
    // point without the contradiction.
    await page.goto("/orders");
    await say(
      page,
      "Di tab <b>Selesai</b>, pesanan yang sudah di-refund <b>tidak ikut dihitung lagi</b>.",
      BEAT.dwell,
    );
    await click(page, page.getByRole("tab", { name: "Dikembalikan" }));
    await page.waitForTimeout(BEAT.short);
    await say(
      page,
      "Semuanya pindah ke tab <b>Dikembalikan</b> — nomor pesanannya tetap utuh.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Refund = batalkan pesanan yang sudah selesai.",
      "Maksimal 1 hari, hanya Owner dan Admin, dan hanya sekali per pesanan.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    for (const p of [para, sirup]) {
      await rpcQuiet(backstage.page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: p.id });
    }
    await backstage.context.close();
  }
});
