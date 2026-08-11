import path from "node:path";

import { test } from "@playwright/test";

import { BEAT, card, Chapters, click, hush, openTour, repoRoot, say, spotlight, type, unspotlight } from "./_studio";

// Recorder for the release video: release/video/product-tour.webm.
//
// A single continuous take through the shop's actual working day — ring up a
// sale, look it up afterwards, check stock, receive a delivery, read the
// numbers, and finish on the pharmacy mode. Every screen is the real app
// driven live, so the video cannot quietly disagree with the product; when a
// screen changes you re-record instead of re-editing.
//
// NOT a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make release-video` (see release/README.md), which stands up the
// seeded demo instance first.

const EMAIL = process.env.JUSTMART_DEMO_EMAIL ?? "owner@justmart.local";
const PASSWORD = process.env.JUSTMART_DEMO_PASSWORD ?? "demo12345";

/**
 * Return a purchase order that is SENT but not yet received, raising one if the
 * shop has none. Kept to a single line — every extra line is another batch
 * number and expiry date typed on camera for no new information.
 */
async function ensureIncomingDelivery(stage: { api: <T>(p: string, b?: unknown) => Promise<T> }) {
  type Po = { id: string; status: string; poNo: string };

  const list = await stage.api<{ orders?: Po[] }>(
    "purchasing_iface.v1.PurchaseOrderService/ListPurchaseOrders",
    { limit: 50 },
  );
  const existing = (list.orders ?? []).find((o) => o.status === "PO_STATUS_SENT");
  if (existing) return existing;

  const { products } = await stage.api<{
    products: { id: string; sku: string; units?: { id: string; isBase: boolean }[] }[];
  }>("inventory_iface.v1.ProductService/ListProducts", { limit: 200 });
  const { suppliers } = await stage.api<{ suppliers: { id: string; code: string }[] }>(
    "inventory_iface.v1.SupplierService/ListSuppliers",
    { limit: 50 },
  );

  const product = products.find((p) => p.sku === "MI-001");
  const supplier = suppliers.find((s) => s.code === "SNS-01") ?? suppliers[0];
  if (!product || !supplier) {
    throw new Error("demo shop is not seeded — run release/seed-demo.mjs first");
  }
  const pack = (product.units ?? []).find((u) => !u.isBase) ?? (product.units ?? [])[0];

  const created = await stage.api<{ order: Po }>(
    "purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder",
    {
      supplierId: supplier.id,
      items: [{ productId: product.id, orderedQty: 6, unitCostPrice: "2850", productUnitId: pack.id }],
      invoiceNo: "FK/SNS-01/2026/07",
      note: "Menunggu kiriman",
    },
  );
  await stage.api("purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: created.order.id });
  return created.order;
}

test("record: product tour", async ({ browser, baseURL }, testInfo) => {
  test.setTimeout(15 * 60_000);

  const stage = await openTour(browser, { baseURL: baseURL!, email: EMAIL, password: PASSWORD });
  const page = stage.page;
  const chapters = new Chapters(stage);

  // The tour FINISHES by switching the shop to pharmacy mode, so a second take
  // would open on "Obat", a Resep menu and a Settings panel already reading
  // "Apotek" — every selector in the retail half would miss. Put it back first,
  // off camera.
  await stage.api("settings_iface.v1.SettingsService/UpdateSettings", {
    lowStockThreshold: 10,
    appTitle: "Justmart",
    businessType: "BUSSINESS_TYPE_RETAIL",
  });

  // The delivery received on camera, provisioned off screen.
  //
  // The tour RECEIVES this order, so it is gone by the end of a take — a second
  // run would otherwise fail with "no incoming delivery" and require a full
  // re-seed just to re-record. Find one, or raise one.
  const openPo = await ensureIncomingDelivery(stage);

  try {
    // ---- Opening -----------------------------------------------------------
    await page.goto("/");
    await page.getByText("Kesehatan bisnis").first().waitFor();
    chapters.mark("Sekilas Justmart");

    await card(
      page,
      "Justmart",
      "Kasir, stok, restok, dan laporan — satu aplikasi untuk toko Anda.",
      BEAT.card,
    );

    // ---- 1. Dashboard ------------------------------------------------------
    await say(page, "Begitu dibuka, kondisi toko <b>hari ini</b> langsung terlihat.", BEAT.read);
    await spotlight(page, page.getByText("Pendapatan hari ini").first(), 14);
    await say(page, "Pendapatan, profit, jumlah transaksi — semuanya berjalan otomatis dari kasir.", BEAT.dwell);
    await unspotlight(page);

    await spotlight(page, page.getByText("Stok menipis").first(), 14);
    await say(page, "Termasuk peringatan <b>stok menipis</b>, supaya tidak kehabisan barang laris.", BEAT.read);
    await unspotlight(page);
    await hush(page);

    // ---- 2. POS ------------------------------------------------------------
    chapters.mark("Kasir & harga grosir");
    await page.goto("/pos");
    const search = page.getByPlaceholder(/Cari produk/i);
    await search.waitFor();
    await page.waitForTimeout(BEAT.short);

    await say(page, "Ini halaman <b>Kasir</b>. Ketik nama barang, atau scan barcode-nya.", BEAT.read);
    await type(page, search, "indomie");
    await page.getByText("Indomie Goreng 85 g").first().waitFor();
    await hush(page);

    await say(
      page,
      "Harga <b>grosir</b> tampil langsung di daftar — beli 12, harganya turun sendiri.",
      BEAT.dwell,
    );
    await spotlight(page, page.getByText(/^Grosir$/).first(), 10);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, page.getByText("Indomie Goreng 85 g").first());

    const qty = page.getByLabel("line quantity").first();
    await qty.waitFor();
    await say(page, "Ubah jumlahnya jadi <b>12</b>…", BEAT.read);
    await click(page, qty);
    await qty.press("Control+a");
    await qty.pressSequentially("12", { delay: 90 });
    await qty.press("Enter");
    await page.waitForTimeout(BEAT.read);

    await say(page, "…dan harga grosir <b>langsung dipakai</b>, tanpa dihitung manual.", BEAT.dwell);
    await hush(page);

    await type(page, search, "aqua");
    await page.getByText("Aqua Botol 600 ml").first().waitFor();
    await click(page, page.getByText("Aqua Botol 600 ml").first());

    const exact = page.getByRole("button", { name: "Uang pas" });
    await say(page, "Pilih pembayaran — <b>Uang pas</b> untuk pembayaran tepat.", BEAT.read);
    await click(page, exact);
    await hush(page);

    await click(page, page.getByRole("button", { name: /Selesaikan/ }));

    const receipt = page.getByRole("dialog");
    await receipt.waitFor();
    await say(page, "Transaksi selesai, <b>struk langsung terbit</b> dan siap dicetak.", BEAT.dwell);
    await page.waitForTimeout(BEAT.short);
    await hush(page);
    // Close the dialog properly before routing away: Ark restores the body
    // lock on the open -> false transition only, and navigating out of an open
    // dialog leaves the next page unclickable.
    await click(page, receipt.getByRole("button", { name: /Transaksi baru/ }));
    await page.waitForTimeout(BEAT.short);

    // ---- 3. Order history --------------------------------------------------
    chapters.mark("Riwayat order");
    await page.goto("/orders");
    await page.getByRole("heading", { name: "Riwayat order" }).waitFor();
    await page.waitForTimeout(BEAT.short);
    await say(page, "Setiap transaksi tersimpan — bisa dicari, difilter, dan dicetak ulang.", BEAT.dwell);
    await spotlight(page, page.locator("tbody tr").first(), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    // This second beat is not padding for its own sake: YouTube drops the whole
    // chapter list if any two marks are under 10s apart, and this section ran to
    // 9s on the take before.
    await say(page, "Cari per nomor, pelanggan, atau produk — dan saring per tanggal atau per kasir.", BEAT.dwell);
    await hush(page);

    // ---- 4. Catalog + stock ------------------------------------------------
    chapters.mark("Katalog & stok");
    await page.goto("/products");
    await page.getByRole("heading", { name: "Produk" }).first().waitFor();
    await page.waitForTimeout(BEAT.short);
    await say(page, "Katalog menampilkan stok <b>Siap</b> dan yang masih <b>Dipesan</b>.", BEAT.dwell);
    await spotlight(page, page.locator("thead th").filter({ hasText: /^Siap$/ }).first(), 10);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await click(page, page.getByText("Indomie Goreng 85 g").first());
    await page.waitForTimeout(BEAT.read);
    await say(page, "Buka satu produk: harga per satuan, batch, kedaluwarsa, dan riwayat harganya.", BEAT.dwell);
    await hush(page);

    // ---- 5. Receiving a delivery -------------------------------------------
    chapters.mark("Restok & terima barang");
    await page.goto(`/purchasing/${openPo.id}`);
    await page.getByText("Dikirim", { exact: true }).first().waitFor();
    await page.waitForTimeout(BEAT.short);
    await say(page, "Barang dari pemasok dicatat lewat <b>Restok</b>. Order ini sedang dikirim.", BEAT.dwell);

    await click(page, page.getByRole("button", { name: "Terima", exact: true }).first());
    // Scope to THIS dialog by its own title. A bare getByRole("dialog") also
    // matches whatever else the page keeps mounted, and the field offsets then
    // point into the wrong form.
    const receive = page.getByRole("dialog").filter({ hasText: "Catat pengiriman" });
    await receive.waitFor();
    await say(page, "Saat barang datang, catat <b>no. batch</b> dan <b>tanggal kedaluwarsa</b>.", BEAT.read);

    // The line's fields carry no label of their own (the column headers do), so
    // anchor on the one field that is identifiable — the expiry date picker —
    // and take the batch input immediately before it. That holds however many
    // lines the delivery has, and does not count the header's date/note fields.
    const expiry = receive.locator('input[placeholder="dd/mm/yyyy"]').last();
    const batch = expiry.locator("xpath=preceding::input[1]");
    await type(page, batch, "B-2608-311");
    await type(page, expiry, "31/12/2027");
    // The date field parses on blur, so without this the text is on screen but
    // the form still counts the expiry as unset and leaves Terima disabled.
    await expiry.press("Tab");
    await page.waitForTimeout(BEAT.short);
    await hush(page);

    await click(page, receive.getByRole("button", { name: "Terima", exact: true }));
    await page.getByText("Diterima", { exact: true }).first().waitFor();
    await say(page, "Stok bertambah otomatis, lengkap dengan <b>modal per batch</b> untuk hitung profit.", BEAT.dwell);
    await hush(page);

    // ---- 6. Analytics ------------------------------------------------------
    chapters.mark("Analitik");
    await page.goto("/analytics/daily");
    await page.getByRole("heading", { name: "Analitik" }).first().waitFor();
    await page.waitForTimeout(BEAT.read);
    await say(page, "Semua itu bermuara di <b>Analitik</b>: terjual, HPP, dan profit per hari.", BEAT.dwell);
    await spotlight(page, page.locator("thead th").filter({ hasText: /^Profit$/ }).first(), 10);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await click(page, page.getByRole("tab", { name: "Grafik" }));
    await page.waitForTimeout(BEAT.read);
    await say(page, "Lihat sebagai grafik, atau ganti rentang tanggalnya sesuai kebutuhan.", BEAT.dwell);
    await hush(page);

    // ---- 7. Multi-warehouse ------------------------------------------------
    chapters.mark("Banyak gudang");
    await say(page, "Punya lebih dari satu gudang atau cabang? Tinggal pindah di sini.", BEAT.read);
    await click(page, page.getByRole("button", { name: /Gudang Utama/ }).first());
    await page.waitForTimeout(BEAT.short);
    await say(page, "Stok, penjualan, dan laporan mengikuti gudang yang sedang aktif.", BEAT.dwell);
    await page.keyboard.press("Escape");
    await page.waitForTimeout(BEAT.short);
    // Same reason as the order-history beat above: this chapter measured 11s,
    // which is one slow `waitFor` away from being dropped by YouTube.
    await say(page, "Barang juga bisa dipindahkan antar gudang lewat menu Transfer.", BEAT.read);
    await hush(page);

    // ---- 8. Pharmacy mode --------------------------------------------------
    chapters.mark("Mode apotek");
    await page.goto("/settings/general");
    await page.getByRole("heading", { name: "Pengaturan" }).first().waitFor();
    await page.waitForTimeout(BEAT.short);
    await say(page, "Punya apotek? Justmart bisa berganti ke <b>mode apotek</b>.", BEAT.read);

    // The mode picker is a Chakra Select: its trigger is a combobox whose
    // accessible name comes from the "Mode bisnis" label, not from the value on
    // it — so match the value as text instead of as the name.
    await click(page, page.getByRole("combobox").filter({ hasText: "Retail" }).first());
    await page.waitForTimeout(BEAT.short);
    await click(page, page.getByRole("option", { name: "Apotek" }).first());
    await click(page, page.getByRole("button", { name: "Simpan" }));
    await page.waitForTimeout(BEAT.read);

    await say(page, "Menu <b>Resep</b> muncul, katalog berubah jadi <b>Obat</b>…", BEAT.read);
    await spotlight(page, page.getByText("Resep", { exact: true }).first(), 8);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(page, "…dan kasir otomatis meminta resep untuk obat yang memerlukannya.", BEAT.dwell);
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Satu aplikasi. Toko atau apotek.",
      "Kasir · stok · restok · laporan — jalan di satu komputer, tanpa langganan bulanan.",
      BEAT.card + 1200,
    );

    const out = path.join(repoRoot(), "release", "video");
    await chapters.write(path.join(out, "chapters.json"));
    await stage.subtitles.write(path.join(out, "subtitles.id.srt"));
    const written = await stage.save(path.join(out, "product-tour.webm"));
    console.log(`\n  ✓ tour written to ${written}`);
    console.log(`  ✓ chapters: ${chapters.toList().map((c) => `${c.at}s ${c.title}`).join(" | ")}`);
    console.log(`  ✓ ${stage.subtitles.length} subtitle cues -> subtitles.id.srt\n`);
  } catch (err) {
    // A tour is minutes long, so a failure two thirds of the way in is
    // expensive to reproduce blind. Leave behind what the page actually looked
    // like at the moment it gave up.
    const shot = path.join(repoRoot(), "release", "video", "_failure.png");
    await page.screenshot({ path: shot, fullPage: false }).catch(() => undefined);
    console.log(`\n  ! failed at ${Math.round(stage.elapsed() / 1000)}s — screenshot: ${shot}\n`);
    throw err;
  } finally {
    await stage.context.close().catch(() => undefined);
  }
});
