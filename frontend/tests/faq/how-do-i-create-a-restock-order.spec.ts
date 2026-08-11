import { test } from "@playwright/test";
import type { Locator, Page } from "@playwright/test";

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

// Recorder for: faq/videos/how-do-i-create-a-restock-order/
//
// "Bagaimana cara membuat restock (pesanan ke pemasok)?"
//
// The happy path is the form itself, so the two halves worth showing beyond it
// are (1) that creating a restock adds NO stock — the order lands as Draf and
// only the later Terima moves the ledger — and (2) the price-agreement warning,
// which is the one check on this form that fires without blocking. A supplier
// price agreement is seeded on the box unit precisely so the row can be made to
// turn red on camera and then be talked back down.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=how-do-i-create-a-restock`.
//
// Residue: the PO is voided in teardown (VoidPurchaseOrder accepts DRAFT and
// SENT), so what survives is one VOIDED PO row plus archived catalog rows.

const SLUG = "how-do-i-create-a-restock-order";

type Unit = { id: string; name: string; factor: string; isBase?: boolean };
type Seeded = { id: string; name: string; units: Unit[] };

async function createProduct(
  page: Page,
  opts: {
    name: string;
    sku: string;
    unit: string;
    price: number;
    packs?: { name: string; factor: number; sellPrice: number }[];
  },
): Promise<Seeded> {
  const res = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: opts.sku,
    name: opts.name,
    unit: opts.unit,
    unitPrice: String(opts.price),
    // Non-base units only — the base one is derived from unit/unitPrice, and
    // sending it here is rejected ("factor must be > 1").
    units: (opts.packs ?? []).map((p, i) => ({
      name: p.name,
      factor: String(p.factor),
      sellPrice: String(p.sellPrice),
      sellable: true,
      purchasable: true,
      sortOrder: i + 1,
      active: true,
    })),
  });
  return { id: res.product.id, name: opts.name, units: res.product.units ?? [] };
}

/** Re-type over a field that already holds a value (qty defaults to 1). */
async function retype(page: Page, target: Locator, text: string): Promise<void> {
  await click(page, target);
  await target.press("Control+a");
  await target.pressSequentially(text, { delay: 70 });
  await page.waitForTimeout(BEAT.short);
}

test("record: bagaimana cara membuat restock", async ({ browser }, testInfo) => {
  test.setTimeout(7 * 60_000);

  const backstage = await openBackstage(browser);
  const marker = String(Date.now()).slice(-6);

  const supplier = await rpc(backstage.page, "inventory_iface.v1.SupplierService/CreateSupplier", {
    code: `KMN${marker}`,
    name: "PT Kimia Nusantara",
  });
  const supplierId = supplier.supplier.id as string;

  const amox = await createProduct(backstage.page, {
    name: "Amoxicillin 500 mg",
    sku: `FAQ-PO-AMX-${marker}`,
    unit: "tablet",
    price: 3500,
    packs: [
      { name: "strip", factor: 10, sellPrice: 33000 },
      { name: "box", factor: 100, sellPrice: 320000 },
    ],
  });
  const vitb = await createProduct(backstage.page, {
    name: "Vitamin B Kompleks",
    sku: `FAQ-PO-VTB-${marker}`,
    unit: "tablet",
    price: 900,
  });

  // The negotiated price for ONE box, so entering more than it turns the line
  // red. Bound to the box unit: an agreement only applies to the unit it was
  // negotiated for, which is why the recording switches the unit first.
  const boxUnit = amox.units.find((u) => u.name === "box");
  if (!boxUnit) throw new Error("seed: Amoxicillin has no box unit");
  const agreement = await rpc(
    backstage.page,
    "inventory_iface.v1.PriceAgreementService/CreatePriceAgreement",
    {
      supplierId,
      productId: amox.id,
      productUnitId: boxUnit.id,
      price: "240000",
      note: "Harga kontrak 2026",
    },
  );

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // Every getByText goes through #root: the caption bar is real DOM outside the
  // app and say() renders its <b> emphasis literally, so narrating a label makes
  // a bare lookup ambiguous with the element it is about to ring.
  const app = page.locator("#root");
  const lineRow = (n: number) => app.getByRole("table").getByRole("row").nth(n);
  const cellBox = (row: number, cell: number) =>
    lineRow(row).getByRole("cell").nth(cell).getByRole("textbox");

  let poId = "";

  try {
    // ---- Opening -----------------------------------------------------------
    // commit + wait for the studio chrome, THEN the card: the list page's first
    // paint waits on its own round trip, and goto's default wait would spend it
    // on a white screen.
    await page.goto("/purchasing/all", { waitUntil: "commit" });
    await page.locator("#__faq_card").waitFor({ state: "attached" });
    await card(
      page,
      "Bagaimana cara membuat restock?",
      "Pesan dulu ke pemasok — stoknya bertambah nanti, saat barangnya datang.",
      BEAT.card,
    );

    // ---- 1. Where it lives -------------------------------------------------
    const newPo = app.getByRole("button", { name: "PO baru" });
    await newPo.waitFor();
    await say(page, "Restock ada di <b>Inventaris ▸ Restok</b>.", BEAT.read);
    await spotlight(page, newPo, 6);
    await say(page, "Mulai dari <b>PO baru</b>.", BEAT.read);
    await unspotlight(page);
    await hush(page);
    await click(page, newPo);

    // ---- 2. Supplier -------------------------------------------------------
    const supplierBox = page.getByPlaceholder("Pilih pemasok");
    await supplierBox.waitFor();
    await say(
      page,
      "Pilih <b>pemasoknya</b> — ini satu-satunya isian wajib selain barangnya.",
      BEAT.read,
    );
    await type(page, supplierBox, "Kimia Nusantara");
    await page.waitForTimeout(700); // debounced SearchSuppliers
    await click(page, page.getByRole("option", { name: /PT Kimia Nusantara/ }).first());

    await say(
      page,
      "No. faktur, tanggal, dan jatuh tempo boleh dikosongkan.",
      BEAT.read,
    );
    await spotlight(page, app.getByText("Jatuh tempo bayar").locator(".."), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 3. Pick the goods -------------------------------------------------
    const addBtn = app.getByRole("button", { name: /^Tambah (obat|produk)$/i });
    await say(page, "Barangnya dipilih lewat <b>Tambah produk</b>.", BEAT.read);
    await click(page, addBtn);

    const picker = page.getByRole("dialog");
    await picker.waitFor();
    await say(
      page,
      "Centang <b>semua barang</b> yang dipesan sekaligus — satu baris per barang.",
      BEAT.dwell,
    );
    const pickerSearch = picker.getByPlaceholder(/Cari nama atau SKU/i);
    await type(page, pickerSearch, "Amoxicillin");
    await page.waitForTimeout(700); // debounced ListProducts
    await click(page, picker.getByRole("row", { name: /Amoxicillin 500 mg/ }).first());
    await pickerSearch.fill("");
    await type(page, pickerSearch, "Vitamin B");
    await page.waitForTimeout(700);
    await click(page, picker.getByRole("row", { name: /Vitamin B Kompleks/ }).first());
    await hush(page);
    await click(page, picker.getByRole("button", { name: "Selesai" }));
    await picker.waitFor({ state: "hidden" });

    // ---- 4. The lines ------------------------------------------------------
    await say(
      page,
      "Tiap baris punya <b>satuan beli</b>, jumlah, dan harga modal per item.",
      BEAT.read,
    );
    const unitTrigger = lineRow(1).locator('button[role="combobox"]').first();
    await click(page, unitTrigger);
    await click(page, page.getByRole("option", { name: "box" }).first());
    await say(
      page,
      "Beli per <b>box</b> — stoknya nanti tetap dihitung per tablet.",
      BEAT.dwell,
    );
    await hush(page);

    await retype(page, cellBox(1, 2), "5");
    await type(page, cellBox(1, 3), "250000");

    // ---- 5. The one check that warns instead of blocking -------------------
    await say(
      page,
      "Kalau ada <b>kesepakatan harga</b> dengan pemasok ini, harganya ikut ditampilkan…",
      BEAT.dwell,
    );
    await spotlight(page, lineRow(1).getByText(/Disepakati/), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(
      page,
      "…dan barisnya jadi <b>merah</b> begitu Anda mengisi lebih mahal.",
      BEAT.read,
    );
    await spotlight(page, lineRow(1).getByText("Di atas harga kesepakatan"), 6);
    await say(
      page,
      "Sifatnya <b>mengingatkan, bukan melarang</b> — harga memang bisa naik.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);
    await retype(page, cellBox(1, 3), "240000");

    // ---- 6. Second line + the derived cost ---------------------------------
    await retype(page, cellBox(2, 2), "200");
    await type(page, cellBox(2, 3), "900");
    await say(
      page,
      "Kolom <b>Harga / satuan dasar</b> memecah harga beli tadi ke satuan terkecil…",
      BEAT.dwell,
    );
    await spotlight(page, lineRow(1).getByRole("cell").nth(5), 6);
    await say(page, "…dan angka itulah yang nanti menempel di stok.", BEAT.read);
    await unspotlight(page);
    await hush(page);

    // ---- 7. PPN ------------------------------------------------------------
    await say(page, "Nyalakan <b>PPN</b> kalau fakturnya kena pajak.", BEAT.read);
    const ppn = app.getByText("PPN", { exact: true }).locator("..").locator('[data-part="control"]');
    await click(page, ppn);
    await say(
      page,
      "PPN dihitung sebagai bagian dari <b>harga modal</b>, bukan biaya terpisah.",
      BEAT.dwell,
    );
    await spotlight(page, app.getByText("Total", { exact: true }).locator(".."), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 8. Create ---------------------------------------------------------
    await say(page, "Terakhir, <b>Buat</b>.", BEAT.read);
    await hush(page);
    await click(page, app.getByRole("button", { name: "Buat", exact: true }));
    await page.waitForURL(/\/purchasing\/[0-9a-f-]{36}$/);
    poId = page.url().split("/").pop() ?? "";

    // By role: the breadcrumb trail prints the same PO number, so a plain text
    // lookup matches twice.
    const poNo = app.getByRole("heading", { name: /^PO-\d{4}-\d{4}$/ });
    await poNo.waitFor();
    await say(
      page,
      "Ordernya dapat <b>nomor sendiri</b>, dan statusnya masih <b>Draf</b>.",
      BEAT.dwell,
    );
    await spotlight(page, poNo.locator(".."), 6);
    await say(page, "Draf berarti <b>belum ada stok yang bertambah</b>.", BEAT.dwell);
    await unspotlight(page);
    await hush(page);

    // ---- 9. Send -----------------------------------------------------------
    const sendBtn = app.getByRole("button", { name: "Kirim", exact: true });
    await say(
      page,
      "Tekan <b>Kirim</b> kalau pesanannya sudah benar-benar dikirim ke pemasok.",
      BEAT.read,
    );
    await hush(page);
    await click(page, sendBtn);
    await app.getByText("Dikirim", { exact: true }).first().waitFor();
    await say(page, "Statusnya jadi <b>Dikirim</b> — tinggal menunggu barang.", BEAT.read);
    await hush(page);

    // ---- 10. What actually adds stock --------------------------------------
    const receiveBtn = app.getByRole("button", { name: "Terima", exact: true });
    await spotlight(page, receiveBtn, 6);
    await say(
      page,
      "Saat barangnya datang, <b>Terima</b>-lah yang menambah stok — lengkap dengan no. batch dan tanggal kedaluwarsa.",
      BEAT.dwell,
    );
    await unspotlight(page);

    await say(
      page,
      "Salah isi? Selama Draf atau Dikirim, <b>Batalkan</b> lalu buat ulang — order yang sudah jadi tidak bisa diedit.",
      BEAT.dwell,
    );
    await spotlight(page, app.getByRole("button", { name: "Batalkan", exact: true }), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Buat → Kirim → Terima.",
      "Stok bertambah di langkah Terima, bukan saat pesanannya dibuat.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    if (poId) {
      await rpcQuiet(backstage.page, "purchasing_iface.v1.PurchaseOrderService/VoidPurchaseOrder", {
        id: poId,
      });
    }
    await rpcQuiet(
      backstage.page,
      "inventory_iface.v1.PriceAgreementService/ArchivePriceAgreement",
      { id: agreement.agreement.id },
    );
    await rpcQuiet(backstage.page, "inventory_iface.v1.SupplierService/ArchiveSupplier", {
      id: supplierId,
    });
    for (const p of [amox, vitb]) {
      await rpcQuiet(backstage.page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: p.id });
    }
    await backstage.context.close();
  }
});
