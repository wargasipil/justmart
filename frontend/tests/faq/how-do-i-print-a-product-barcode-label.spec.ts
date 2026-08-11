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
  unspotlight,
  videoDest,
} from "./_recorder";

// Recorder for: faq/videos/how-do-i-print-a-product-barcode-label/
//
// "Bagaimana cara mencetak label barcode produk?"
//
// Two halves, because the interesting rule is the second one: a label prints
// name + a CODE128 of the SKU + the chosen unit's price, and it is REFUSED when
// the barcode would be wider than the receipt paper. That refusal is the whole
// point — a thermal printer given an over-wide symbol silently truncates it, so
// the alternative to refusing is a sticker that looks perfect and scans as
// nothing.
//
// SAFETY, load-bearing: the recorder never dispatches to a real printer. The
// only Cetak we click is the boundary product, whose SKU (30 chars) cannot fit
// ANY supported paper width — the handler refuses at the fit check, which runs
// BEFORE the dispatch. The happy-path dialog is shown and then cancelled. This
// matters because config.yaml ships `connector.mode: usb`, so a successful
// dispatch would spool to the recording machine's default Windows printer and
// physically print during a docs run.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=barcode`.

const SLUG = "how-do-i-print-a-product-barcode-label";

type Seed = { okId: string; longId: string };

test("record: bagaimana cara mencetak label barcode produk", async ({ browser }, testInfo) => {
  test.setTimeout(5 * 60_000);

  const backstage = await openBackstage(browser);
  const marker = String(Date.now()).slice(-6);

  // A: a normal product with a short SKU and a second, larger unit — the unit
  // picker is what makes the price on the label change, so there has to be
  // something to pick.
  const ok = await rpc(backstage.page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: `PCT${marker}`,
    name: "Paracetamol 500 mg",
    unit: "tablet",
    unitPrice: "700",
    // Only the NON-base units go here; the base tablet comes from unit/unitPrice.
    units: [
      { name: "box", factor: "100", sellPrice: "62000", sellable: true, purchasable: true },
    ],
  });

  // B: the boundary. 30 characters of SKU needs ~730 dots of bar; 58mm paper
  // prints 384 and 80mm prints 576, so this is refused whatever the shop's
  // paper is set to — which is exactly why it is safe to press Cetak on camera.
  const long = await rpc(backstage.page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: `SIRUP-OBAT-BATUK-HERBAL-${marker}`,
    name: "Sirup Obat Batuk Herbal 100 ml",
    unit: "botol",
    unitPrice: "24000",
  });

  const seeds: Seed = { okId: ok.product.id, longId: long.product.id };

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // Every text lookup is scoped to the app: the caption bar is real DOM outside
  // #root and renders its <b> emphasis literally, so a bare getByText can match
  // the very caption that is narrating the element.
  const app = page.locator("#root");

  try {
    // ---- Opening -----------------------------------------------------------
    // Card first, THEN wait for data — the page loads behind the title instead
    // of opening on a half-painted app.
    await page.goto(`/products/${seeds.okId}`);
    await card(
      page,
      "Mau pasang barcode di rak. Cetaknya dari mana?",
      "Dari halaman produknya — barcode-nya memakai SKU produk itu.",
      BEAT.card,
    );
    await app.getByText("Paracetamol 500 mg").first().waitFor();

    // ---- 1. The affordance -------------------------------------------------
    const printBtn = page.getByRole("button", { name: "Cetak barcode", exact: true });
    await say(page, "Buka produknya, lalu klik <b>Cetak barcode</b>.", BEAT.read);
    await spotlight(page, printBtn, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, printBtn);

    const dialog = page.getByRole("dialog");
    await dialog.waitFor();

    // ---- 2. What is on the label ------------------------------------------
    await say(
      page,
      "Yang dicetak: nama produk, barcode dari <b>SKU</b>-nya, dan harga.",
      BEAT.read,
    );
    const preview = dialog.locator("svg").last();
    await spotlight(page, preview, 10);
    await say(
      page,
      "SKU-nya ikut tercetak di bawah garis — kalau scanner gagal, kodenya masih bisa diketik.",
      BEAT.dwell,
    );
    await unspotlight(page);

    await say(
      page,
      "Kasir mencari barang lewat SKU yang sama persis, jadi label ini <b>pasti terbaca</b> di kasir.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- 3. The unit drives the price -------------------------------------
    // button[role=combobox], not getByRole("combobox"): EnumSelect also renders
    // Select.HiddenSelect, a hidden native <select> that matches the role first.
    const unitSelect = dialog.locator('button[role="combobox"]').first();
    await say(
      page,
      "Harga di label mengikuti <b>satuan</b> yang dipilih di sini.",
      BEAT.read,
    );
    await spotlight(page, unitSelect, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await click(page, unitSelect);
    // CSS, not getByRole: EnumSelect portals its listbox to <body>, and the open
    // dialog marks everything outside itself aria-hidden — so the options are
    // on screen (a real user picks them fine) but absent from the a11y tree.
    const boxOption = page.locator('[role="option"]:visible').filter({ hasText: /box/i }).first();
    await boxOption.waitFor();
    await click(page, boxOption);

    await say(
      page,
      "Pilih <b>box</b>, dan pratinjaunya langsung memakai harga box — bukan harga per butir.",
      BEAT.dwell,
    );
    await spotlight(page, dialog.getByText(/\/ box/).first(), 8);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 4. Copies + destination -------------------------------------------
    const copies = dialog.locator("input[type=number]").first();
    await say(page, "<b>Jumlah</b> menentukan berapa lembar label yang sama.", BEAT.read);
    // Select the existing "1" before typing: the field is controlled and clamped,
    // so appending would run 1 -> 11 -> 112 -> 100 on camera.
    await click(page, copies);
    await copies.selectText();
    await copies.pressSequentially("12", { delay: 55 });
    await page.waitForTimeout(BEAT.short);

    await say(
      page,
      "<b>Printer termal</b> mengirim ke printer struk toko — printer yang sama dipakai kasir.",
      BEAT.dwell,
    );
    const sheetTab = dialog.getByText("Lembar label", { exact: true });
    await spotlight(page, sheetTab, 6);
    await say(
      page,
      "<b>Lembar label</b> mencetak lewat printer biasa, untuk kertas stiker.",
      BEAT.read,
    );
    await unspotlight(page);
    await click(page, sheetTab);
    await say(
      page,
      "Untuk kertas stiker muncul <b>Per baris</b> — berapa label sebaris di halaman.",
      BEAT.dwell,
    );
    await hush(page);

    await click(page, dialog.getByRole("button", { name: "Batal", exact: true }));
    await dialog.waitFor({ state: "hidden" });

    // ---- 5. The boundary ---------------------------------------------------
    await page.goto(`/products/${seeds.longId}`);
    await app.getByText("Sirup Obat Batuk Herbal 100 ml").first().waitFor();

    await say(
      page,
      "Satu hal yang perlu diketahui: kertas struk itu sempit.",
      BEAT.read,
    );
    await click(page, page.getByRole("button", { name: "Cetak barcode", exact: true }));
    await dialog.waitFor();

    await say(page, "Produk ini SKU-nya panjang sekali.", BEAT.read);
    await spotlight(page, dialog.locator("svg").last(), 10);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    // Back to the thermal target: the destination is remembered from the
    // previous product, and the paper limit only exists on that side.
    await click(page, dialog.getByText("Printer termal", { exact: true }));
    await hush(page);
    await click(page, dialog.getByRole("button", { name: "Cetak", exact: true }));

    // No spotlight on the toast: it animates in and auto-dismisses, so it never
    // holds still long enough to ring — and it is already the loudest thing on
    // screen. Keep the caption over it short enough to outlive it.
    const refusal = page.getByText(/terlalu panjang/i).first();
    await refusal.waitFor();
    await say(
      page,
      "Barcode-nya lebih lebar dari kertas, jadi aplikasi <b>menolak mencetak</b>.",
      BEAT.read,
    );

    await say(
      page,
      "Kalau dipaksakan, printer akan <b>memotong</b> barcode-nya — stikernya kelihatan normal tapi tidak terbaca.",
      BEAT.dwell,
    );
    await say(
      page,
      "Solusinya: kertas 80mm, SKU yang lebih pendek, atau cetak lewat <b>Lembar label</b>.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "SKU pendek = barcode tebal = gampang discan.",
      "58mm muat sekitar 12 karakter, 80mm sekitar 21. Lembar label tidak ada batasnya.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    await rpcQuiet(backstage.page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: seeds.okId });
    await rpcQuiet(backstage.page, "inventory_iface.v1.ProductService/ArchiveProduct", { id: seeds.longId });
    await backstage.context.close();
  }
});
