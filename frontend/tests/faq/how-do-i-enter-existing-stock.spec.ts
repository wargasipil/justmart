import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

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

// Recorder for: faq/videos/how-do-i-enter-existing-stock/
//
// "Bagaimana memasukkan stok yang sudah ada di toko?"
//
// The answer is Impor Stok on the Batches page — but the half that actually
// costs a shop money is what happens when the same file is imported twice. So
// the recording imports the file, then imports it AGAIN: rows carrying a batch
// number are skipped (safe to re-run), and the row without one is created a
// second time (stock doubles). Showing only the first import would teach the
// happy path and generate exactly the "my stock is double" report this entry
// exists to prevent.
//
// The CSV also carries two deliberately bad rows, because they fail in two
// different places: a malformed date is caught client-side and never sent (red
// in the preview), while an unknown SKU passes the preview and fails per-row on
// the server. A viewer who has seen both knows why a green preview is not a
// guarantee.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=how-do-i-enter-existing-stock`.

const SLUG = "how-do-i-enter-existing-stock";

type Seed = { productIds: string[]; csvPath: string };

/**
 * Three catalog products with no stock, plus the CSV that will fill them.
 *
 * The rows are chosen so each one demonstrates something:
 *   1. entered in a pack     -> 5 box x 100 = 500 base units
 *   2. plain base units + a lot number
 *   3. no lot number, no expiry -> the row that doubles on a re-import
 *   4. a SKU that is not in the catalog -> passes the preview, fails on import
 *   5. a malformed date -> caught in the preview, never sent
 */
async function seedCatalog(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  marker: string,
): Promise<Seed> {
  const pct = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: `PCT500-${marker}`,
    name: "Paracetamol 500 mg",
    unit: "tablet",
    unitPrice: "1500",
    units: [
      {
        name: "box",
        factor: "100",
        sellPrice: "140000",
        sellable: true,
        purchasable: true,
        active: true,
      },
    ],
  });
  const amx = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: `AMX500-${marker}`,
    name: "Amoxicillin 500 mg",
    unit: "kapsul",
    unitPrice: "2000",
  });
  const trm = await rpc(page, "inventory_iface.v1.ProductService/CreateProduct", {
    sku: `TRM-${marker}`,
    name: "Termometer Digital",
    unit: "pcs",
    unitPrice: "85000",
  });

  const csv = [
    "sku,quantity,unit,cost,batch_number,expiry_date",
    `PCT500-${marker},5,box,400,SA-2026-001,2027-12-31`,
    `AMX500-${marker},240,,1200,SA-2026-002,2027-06-30`,
    `TRM-${marker},12,,62000,,`,
    `GRM500-${marker},30,,2500,,`,
    `AMX500-${marker},50,,1200,SA-2026-003,31/12/2027`,
  ].join("\n");

  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "justmart-faq-csv-"));
  const csvPath = path.join(dir, "stok-awal.csv");
  await fs.writeFile(csvPath, csv, "utf8");

  return { productIds: [pct.product.id, amx.product.id, trm.product.id], csvPath };
}

// The imported lots cannot be removed over the API (there is no DeleteBatch, by
// design — the ledger is insert-only), so teardown archives the products they
// hang off instead. Same documented residue as the restock recorder; the SKUs
// are marker-unique so it accumulates slowly.
async function cleanup(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  seed: Seed,
): Promise<void> {
  for (const id of seed.productIds) {
    await rpcQuiet(page, "inventory_iface.v1.ProductService/ArchiveProduct", { id });
  }
  await fs.rm(path.dirname(seed.csvPath), { recursive: true, force: true }).catch(() => undefined);
}

test("record: memasukkan stok yang sudah ada", async ({ browser }, testInfo) => {
  test.setTimeout(6 * 60_000);

  const backstage = await openBackstage(browser);
  const marker = String(Date.now()).slice(-6);
  const seed = await seedCatalog(backstage.page, marker);

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // Every getByText goes through #root: the caption bar is real DOM outside the
  // app and say() renders its <b> emphasis literally, so a bare getByText would
  // happily match the sentence describing the thing instead of the thing.
  const app = page.locator("#root");

  /**
   * Run the import and wait for the result panel.
   *
   * The explicit timeout is not padding: the first run of a session compiles the
   * Go binary and cold-transforms the Vite module graph, and the default 15s
   * elapsed while the very first ImportStock round trip was still in flight —
   * throwing away a take that had already spent a minute narrating. A recording
   * costs minutes, so the wait is generous on purpose.
   */
  const runImport = async (d: ReturnType<typeof page.getByRole>) => {
    await click(page, d.getByRole("button", { name: /^Impor \d+$/ }));
    await d.getByText(/dibuat/).waitFor({ timeout: 60_000 });
  };

  /** Open Impor Stok, feed it the CSV, and wait for the preview to render. */
  const openImportWithFile = async () => {
    await click(page, app.getByRole("button", { name: "Impor Stok", exact: true }));
    const dialog = page.getByRole("dialog");
    await dialog.waitFor();
    const chooser = page.waitForEvent("filechooser");
    await click(page, dialog.getByRole("button", { name: "Pilih file CSV", exact: true }));
    await (await chooser).setFiles(seed.csvPath);
    await dialog.getByText(/siap diimpor/).waitFor();
    return dialog;
  };

  try {
    // ---- Opening -----------------------------------------------------------
    // Card first, THEN wait for the data — the list finishes loading behind it,
    // so the recording opens on the title instead of a half-painted table.
    await page.goto("/inventory/batches");
    await card(
      page,
      "Stok yang sudah ada di rak, bagaimana memasukkannya?",
      "Satu file CSV — tanpa order restock, tanpa utang ke pemasok.",
      BEAT.card,
    );
    await app.getByRole("button", { name: "Impor Stok", exact: true }).waitFor();

    await say(
      page,
      "Barang lama Anda sudah ada di rak — tinggal <b>dicatat</b>, bukan dibeli ulang lewat restock.",
      BEAT.dwell,
    );

    // ---- 1. Where it lives -------------------------------------------------
    await say(page, "Semua lot stok tercatat di <b>Inventaris ▸ Batch</b>.", BEAT.read);
    const importBtn = app.getByRole("button", { name: "Impor Stok", exact: true });
    await spotlight(page, importBtn, 6);
    await say(
      page,
      "Untuk stok awal, pakai <b>Impor Stok</b> — sekali unggah untuk semua barang.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- 2. The template + the required columns ----------------------------
    let dialog = page.getByRole("dialog");
    await click(page, importBtn);
    await dialog.waitFor();

    await spotlight(page, dialog.getByRole("button", { name: "Unduh templat", exact: true }), 6);
    await say(
      page,
      "Mulai dari <b>Unduh templat</b> — kolomnya sudah benar, tinggal diisi di Excel.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await say(
      page,
      "Yang wajib hanya <b>sku</b> dan <b>quantity</b>. SKU harus sama persis dengan produk di katalog.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- 3. Feed it the file ----------------------------------------------
    const chooser = page.waitForEvent("filechooser");
    await click(page, dialog.getByRole("button", { name: "Pilih file CSV", exact: true }));
    await (await chooser).setFiles(seed.csvPath);
    await dialog.getByText(/siap diimpor/).waitFor();

    await say(page, "Isi filenya ditampilkan dulu, sebelum apa pun disimpan.", BEAT.read);

    // The malformed date — caught here, never sent.
    const badDateRow = dialog.getByRole("row").filter({ hasText: "expiry_date harus" });
    await spotlight(page, badDateRow, 4);
    await say(
      page,
      "Baris yang formatnya salah ditandai <b>merah</b> dan tidak ikut dikirim — sisanya tetap jalan.",
      BEAT.dwell,
    );
    await unspotlight(page);

    // The pack conversion.
    const packRow = dialog.getByRole("row").filter({ hasText: `PCT500-${marker}` });
    await spotlight(page, packRow, 4);
    await say(
      page,
      "Kolom <b>unit</b> ikut dihitung: 5 box otomatis menjadi 500 tablet.",
      BEAT.dwell,
    );
    await say(
      page,
      "Tapi harga modalnya selalu <b>per unit dasar</b> — per tablet, bukan per box.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- 4. Import ---------------------------------------------------------
    await runImport(dialog);

    await spotlight(page, dialog.getByText(/dibuat/).first(), 6);
    await say(page, "Hasilnya dilaporkan per baris.", BEAT.read);
    await unspotlight(page);
    await say(
      page,
      "Satu baris gagal: SKU-nya <b>belum ada di katalog</b>. Pratinjau hanya memeriksa format — SKU dicek saat impor.",
      BEAT.dwell,
    );
    await spotlight(page, dialog.getByText(/GRM500/).first(), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, dialog.getByRole("button", { name: "Tutup", exact: true }));
    await dialog.waitFor({ state: "hidden" });

    // ---- 5. The stock actually landed --------------------------------------
    await type(page, app.getByPlaceholder(/no batch/i), "SA-2026");
    await page.waitForTimeout(BEAT.short);
    await say(
      page,
      "Lot-nya langsung masuk ke <b>gudang yang sedang aktif</b> dan siap dijual di kasir.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- 6. The boundary: importing the same file twice --------------------
    await say(
      page,
      "Lalu bagian yang paling sering bikin repot: <b>file yang sama diimpor dua kali</b>.",
      BEAT.dwell,
    );
    dialog = await openImportWithFile();
    await runImport(dialog);

    await spotlight(page, dialog.getByText(/dibuat/).first(), 6);
    await say(
      page,
      "Baris yang punya <b>nomor batch</b> dilewati — nomor itu sudah ada, jadi stoknya tidak dobel.",
      BEAT.dwell,
    );
    await say(
      page,
      "Tapi baris <b>tanpa nomor batch dibuat lagi</b>. Stok termometernya sekarang dua kali lipat.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await say(
      page,
      "Karena itu isilah <b>batch_number</b> — itulah yang membuat file aman diimpor ulang.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Impor Stok = saldo awal. Restok = beli dari pemasok.",
      "Isi nomor batch supaya file yang sama aman diulang. Hanya Owner / Admin.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    await cleanup(backstage.page, seed);
    await backstage.context.close();
  }
});
