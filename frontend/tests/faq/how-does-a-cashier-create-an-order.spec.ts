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

// Recorder for: faq/videos/how-does-a-cashier-create-an-order/
//
// "Bagaimana kasir membuat transaksi (order)?"
//
// The happy path is the whole feature here — search, cart, pay, receipt — so
// the boundary this take has to show is the set of states where the till
// REFUSES: a zero-stock row that cannot be clicked, and a Selesaikan button
// that stays dead while the cash tendered is under the total. Both are seeded
// rather than described: one product deliberately gets no batch, and the
// payment step is filmed before the amount is entered.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=how-does-a-cashier`.
//
// Residue: the COMPLETED sale cannot be voided (VoidSale is DRAFT-only) and no
// refund is issued here, so one small sale stays in the dev DB per run. Its
// products are archived, and their names carry the run marker.

const SLUG = "how-does-a-cashier-create-an-order";

type Seeded = { id: string; name: string; sku: string };

async function createProduct(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  opts: {
    name: string;
    sku: string;
    unit: string;
    price: number;
    /** Extra sellable unit (e.g. a bottle of 30 tablets). */
    pack?: { name: string; factor: number; price: number };
    /** Omit to leave the product with no stock at all. */
    stock?: { qty: number; batch: string; cost: number };
  },
): Promise<Seeded> {
  // `units` carries the NON-base units only — the base one is derived from
  // unit/unitPrice, and sending it here is rejected ("factor must be > 1").
  const units = opts.pack
    ? [
        {
          name: opts.pack.name,
          factor: opts.pack.factor,
          isBase: false,
          sellPrice: String(opts.pack.price),
          sellable: true,
          purchasable: true,
          sortOrder: 1,
          active: true,
        },
      ]
    : [];
  const res = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: opts.sku,
    name: opts.name,
    unit: opts.unit,
    unitPrice: String(opts.price),
    units,
  });
  const id = res.product.id as string;

  if (opts.stock) {
    await rpc(page, "inventory_iface.v1.BatchService/CreateBatch", {
      productId: id,
      batchNumber: opts.stock.batch,
      expiryDate: "2028-09-30",
      costPrice: String(opts.stock.cost),
      initialQuantity: String(opts.stock.qty),
    });
  }
  return { id, name: opts.name, sku: opts.sku };
}

test("record: bagaimana kasir membuat transaksi", async ({ browser }, testInfo) => {
  test.setTimeout(6 * 60_000);

  const backstage = await openBackstage(browser);
  const marker = String(Date.now()).slice(-6);

  // The two products the sale is built from, plus one with no batch at all —
  // that third one is what makes the "cannot be clicked" half of the answer
  // real instead of narrated.
  const para = await createProduct(backstage.page, {
    name: "Paracetamol 500 mg",
    sku: `FAQ-KSR-PCT-${marker}`,
    unit: "tablet",
    price: 1500,
    stock: { qty: 200, batch: `B-KSR-${marker}`, cost: 900 },
  });
  const vitc = await createProduct(backstage.page, {
    name: "Vitamin C 500 mg",
    // Numeric so the scan step reads like a real barcode rather than a slug.
    sku: `899100${marker}`,
    unit: "tablet",
    price: 2500,
    pack: { name: "botol", factor: 30, price: 65000 },
    stock: { qty: 120, batch: `B-KSR-V${marker}`, cost: 1600 },
  });
  const empty = await createProduct(backstage.page, {
    name: "Salep Hidrokortison 2,5%",
    sku: `FAQ-KSR-SLP-${marker}`,
    unit: "tube",
    price: 18000,
    // no stock: the row renders dimmed and refuses the click
  });

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;

  try {
    // ---- Opening -----------------------------------------------------------
    // Card first, THEN wait for the catalog — the till finishes loading behind
    // it, so the recording opens on the title, not on a half-painted app.
    await page.goto("/pos");
    await card(
      page,
      "Bagaimana kasir membuat transaksi?",
      "Cari barangnya, atur jumlah, terima uangnya, selesai.",
      BEAT.card,
    );
    const search = page.getByPlaceholder(/scan barcode/i);
    await search.waitFor();
    await page.getByText(para.name, { exact: true }).first().waitFor();

    // ---- 1. The screen -----------------------------------------------------
    await say(
      page,
      "Menu <b>Kasir</b> memakai satu layar penuh: pencarian di kiri, keranjang di kanan.",
      BEAT.read,
    );
    await say(
      page,
      "Di atas tertera <b>gudang</b> tempat Anda berjualan. Stok yang tampil adalah stok gudang itu.",
      BEAT.dwell,
    );
    await spotlight(page, page.getByRole("button", { name: /Gudang Utama/ }).first(), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 2. Find and add ---------------------------------------------------
    await say(page, "Ketik nama atau kode barangnya.", BEAT.read);
    await type(page, search, "Paracetamol");
    await say(
      page,
      "Setiap baris adalah satu <b>satuan jual</b> — lengkap dengan harga dan sisa stoknya.",
      BEAT.dwell,
    );
    const paraRow = page.getByText(para.name, { exact: true }).first();
    await spotlight(page, paraRow, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await say(page, "Sekali klik, barangnya masuk keranjang.", BEAT.read);
    await click(page, paraRow);
    await page.getByRole("button", { name: "increase quantity" }).first().waitFor();
    await hush(page);

    // ---- 3. Quantity -------------------------------------------------------
    await say(
      page,
      "Jumlahnya diatur lewat tombol <b>−</b> dan <b>+</b>, atau diketik langsung.",
      BEAT.read,
    );
    const plus = page.getByRole("button", { name: "increase quantity" }).first();
    await click(page, plus);
    await click(page, plus);
    await hush(page);

    // ---- 4. Barcode --------------------------------------------------------
    await say(
      page,
      "Pakai <b>scanner barcode</b>? Kode yang persis sama dengan SKU langsung masuk begitu Enter.",
      BEAT.dwell,
    );
    await type(page, search, vitc.sku);
    await search.press("Enter");
    await page.waitForTimeout(BEAT.short);
    const vitcLine = page.getByText(vitc.name, { exact: true }).last();
    await spotlight(page, vitcLine.locator("..").locator(".."), 6);
    await say(
      page,
      "Kalau satu barang punya beberapa satuan, satuannya bisa diganti di baris keranjang ini.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- 5. What the till refuses -----------------------------------------
    await say(page, "Barang yang <b>stoknya habis</b> di gudang ini tampil redup…", BEAT.read);
    await type(page, search, "Salep");
    const emptyRow = page.getByText(empty.name, { exact: true }).first();
    await spotlight(page, emptyRow, 6);
    await say(page, "…dan <b>tidak bisa diklik</b> sampai barangnya direstock.", BEAT.dwell);
    await unspotlight(page);
    await say(
      page,
      "Matikan <b>Tampilkan stok habis</b> kalau ingin baris seperti ini disembunyikan.",
      BEAT.read,
    );
    // Scoped to #root: the caption above says the same words, and the studio
    // chrome lives outside #root — an unscoped getByText matches both.
    await spotlight(page, page.locator("#root").getByText("Tampilkan stok habis"), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);
    await search.fill("");
    await page.waitForTimeout(BEAT.short);

    // ---- 6. Optional bits on the cart --------------------------------------
    await say(
      page,
      "<b>Pelanggan</b> boleh dikosongkan — isi kalau transaksinya perlu tercatat atas nama seseorang.",
      BEAT.dwell,
    );
    await spotlight(page, page.getByRole("button", { name: "Pilih pelanggan" }), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(
      page,
      "Baris <b>Resep</b> dan kolom <b>Biaya jasa</b> hanya ada di mode apotek.",
      BEAT.read,
    );
    await spotlight(page, page.getByRole("button", { name: "Lampirkan resep (F5)" }), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 7. Payment --------------------------------------------------------
    await say(page, "Totalnya terhitung sendiri.", BEAT.read);
    await spotlight(page, page.getByText("TOTAL", { exact: true }).locator(".."), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(page, "Pilih <b>Tunai</b> atau <b>Non-tunai</b>.", BEAT.read);
    await spotlight(page, page.getByRole("radiogroup"), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    const complete = page.getByRole("button", { name: "Selesaikan (F8)" });
    await say(
      page,
      "Selama uang yang diterima <b>masih kurang dari total</b>, tombol Selesaikan mati.",
      BEAT.dwell,
    );
    await spotlight(page, complete, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(page, "Isi <b>Dibayar</b> dengan uang yang diterima.", BEAT.read);
    // Typed rather than filled from a pay chip: the chip's label is a formatted
    // currency string, and matching it by accessible name is exactly the kind
    // of locator that quietly resolves to nothing and leaves the take with an
    // unpaid cart. The chips are still shown and named in the next caption.
    await type(
      page,
      page.locator("#root").getByText("Dibayar", { exact: true }).locator("..").getByRole("textbox"),
      "10000",
    );
    await say(
      page,
      "Atau sekali klik lewat <b>Uang pas</b> dan pecahan uang di bawahnya.",
      BEAT.read,
    );
    await spotlight(page, page.getByRole("button", { name: "Uang pas" }).locator(".."), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(page, "<b>Kembaliannya</b> langsung terhitung.", BEAT.read);
    await spotlight(page, page.getByText("Kembalian", { exact: true }).locator(".."), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 8. Complete -------------------------------------------------------
    await say(page, "Tekan <b>Selesaikan</b> — atau <b>F8</b> dari keyboard.", BEAT.read);
    // Sync point, not an assertion: the button is dead until the cash covers
    // the total, and clicking a dead one is a no-op the recorder would sail
    // straight past — the failure would only surface later, at the receipt.
    await page.locator("button:not([disabled])").filter({ hasText: "Selesaikan" }).waitFor();
    await click(page, complete);

    const receipt = page.getByRole("dialog");
    await receipt.waitFor();
    await say(page, "Struk muncul dengan <b>nomor transaksi</b>-nya.", BEAT.read);
    await spotlight(page, receipt.getByText(/Struk/).first(), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(
      page,
      "<b>Cetak</b> ke printer, atau langsung <b>Transaksi baru</b> untuk pelanggan berikutnya. Struk bisa dicetak ulang kapan saja.",
      BEAT.dwell,
    );
    // Drop the caption BEFORE the click: it names two buttons, and the click
    // closes the dialog they live in — leaving it up would hold a caption over
    // a screen that no longer contains its subject.
    await hush(page);
    await click(page, receipt.getByRole("button", { name: "Transaksi baru" }));
    await receipt.waitFor({ state: "hidden" });

    // ---- 9. The draft is not a sale ---------------------------------------
    await say(
      page,
      "Keranjang yang <b>belum diselesaikan</b> hanya draf — dibuang begitu Anda keluar dari Kasir.",
      BEAT.dwell,
    );
    await hush(page);
    await click(page, page.getByRole("button", { name: "exit" }));
    await page.waitForURL(/\/$/);

    // ---- 10. Where it lands ------------------------------------------------
    await page.goto("/orders");
    await say(
      page,
      "Yang <b>sudah diselesaikan</b> masuk ke <b>Riwayat order</b>.",
      BEAT.read,
    );
    await type(page, page.getByPlaceholder(/Cari/i).first(), para.name);
    const row = page.getByRole("row").filter({ hasText: para.name }).first();
    await row.waitFor();
    await spotlight(page, row, 4);
    await say(
      page,
      "Kasir melihat transaksinya sendiri; Admin dan Owner melihat semuanya.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Cari → klik → bayar → Selesaikan (F8).",
      "Stok berkurang otomatis dari batch yang paling dekat kedaluwarsa, di gudang transaksi itu.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    for (const p of [para, vitc, empty]) {
      await rpcQuiet(backstage.page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: p.id });
    }
    await backstage.context.close();
  }
});
