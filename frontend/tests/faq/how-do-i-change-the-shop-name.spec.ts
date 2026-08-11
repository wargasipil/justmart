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

// Recorder for: faq/videos/how-do-i-change-the-shop-name/
//
// "Bagaimana cara mengganti judul / nama toko yang tampil?"
//
// The question is ambiguous in the way that matters: "the title" is TWO
// settings — the app title (sidebar / tab / login, Settings ▸ Umum) and the
// printed receipt header (Settings ▸ Pencetakan). Someone who changes one and
// expects the other to follow is the whole reason this entry exists, so the
// video does both in sequence and names the difference out loud.
//
// There is nothing to seed: both halves are edits to app_settings rows that
// already exist. What the recorder must do instead is RESTORE them — it changes
// shop-wide settings on the dev instance, so the originals are read backstage
// first and written back in the finally block.
//
// What it cannot show: the printed receipt. The header only reaches paper (or
// the spooler), and the on-screen receipt dialog after a sale does not render
// it, so there is no honest way to film the effect — a mock-up would be exactly
// the lie this format exists to avoid. The video shows the edit and says what
// it does; question.md carries the rest.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=how-do-i-change-the-shop-name`.

const SLUG = "how-do-i-change-the-shop-name";

const START_TITLE = "Apotek Sehat Farma";
const NEW_TITLE = "Apotek Melati";
const NEW_HEADER = "APOTEK MELATI\nJl. Merdeka No. 12, Bandung\nTelp. 022-1234567";

/**
 * Write the app title, passing the threshold through so it isn't reset.
 *
 * UpdateSettings takes all three General fields in one call: the threshold has
 * to ride along, and businessType stays UNSPECIFIED, which the handler reads as
 * "leave the mode alone".
 */
async function setAppTitle(
  page: Awaited<ReturnType<typeof openBackstage>>["page"],
  appTitle: string,
  lowStockThreshold: number,
): Promise<void> {
  await rpc(page, "settings_iface.v1.SettingsService/UpdateSettings", {
    appTitle,
    lowStockThreshold,
    businessType: 0,
  });
}

test("record: bagaimana cara mengganti judul dan header struk", async ({ browser }, testInfo) => {
  test.setTimeout(5 * 60_000);

  const backstage = await openBackstage(browser);

  // Capture what the shop looks like now so the finally block can put it back.
  // UpdateSettings writes threshold + title in one call, so the threshold has to
  // ride along or restoring the title would reset it; businessType is left
  // UNSPECIFIED, which the handler reads as "don't touch the mode".
  const before = await rpc(backstage.page, "settings_iface.v1.SettingsService/GetSettings", {});
  const beforeReceipt = await rpc(
    backstage.page,
    "settings_iface.v1.SettingsService/GetReceiptSettings",
    {},
  );
  const original = {
    appTitle: before.settings?.appTitle ?? "",
    lowStockThreshold: before.settings?.lowStockThreshold ?? 10,
    header: beforeReceipt.header ?? "",
    footer: beforeReceipt.footer ?? "",
    width: beforeReceipt.width ?? 32,
  };

  // Force a known starting name rather than trusting whatever the dev shop
  // happens to carry. The whole first half of the video is "watch this change",
  // and if a previous run's restore has already left NEW_TITLE in place the
  // take silently becomes a no-op that narrates a change nobody can see. The
  // original is still captured above and put back in the finally block.
  await setAppTitle(backstage.page, START_TITLE, original.lowStockThreshold);

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // Scoped to the app: the caption bar is real DOM outside #root and renders
  // its <b> emphasis literally, so a bare getByText collides with the narration.
  const app = page.locator("#root");
  const brand = (name: string) => app.locator("aside").getByText(name, { exact: true });

  try {
    // ---- Opening -----------------------------------------------------------
    await page.goto("/settings/general", { waitUntil: "commit" });
    await page.locator("#__faq_card").waitFor({ state: "attached" });
    await card(
      page,
      "Bagaimana cara mengganti nama toko yang tampil?",
      "Ada dua tempat — dan keduanya bukan hal yang sama.",
      BEAT.card,
    );
    await app.getByText("Judul aplikasi").waitFor();

    // ---- 1. The name on screen ---------------------------------------------
    await say(page, "Nama di pojok kiri atas ini datang dari satu pengaturan.", BEAT.read);
    await spotlight(page, brand(START_TITLE), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(page, "Tempatnya di <b>Pengaturan ▸ Umum</b>, kolom paling atas.", BEAT.read);
    const titleField = app.getByRole("textbox").first();
    await spotlight(page, titleField, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // Not `type()` here: it glides the cursor by clicking the target, which
    // would collapse a select-all and append to the old name instead of
    // replacing it. Glide, clear, then type at the same human rate.
    await click(page, titleField);
    await titleField.fill("");
    await titleField.pressSequentially(NEW_TITLE, { delay: 55 });
    await page.waitForTimeout(BEAT.short);
    await click(page, app.getByRole("button", { name: "Simpan" }));

    // ---- 2. It lands immediately -------------------------------------------
    await brand(NEW_TITLE).waitFor();
    await say(page, "Begitu disimpan, namanya <b>langsung berubah</b> — tanpa muat ulang.", BEAT.dwell);
    await spotlight(page, brand(NEW_TITLE), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(
      page,
      "Nama yang sama juga dipakai di <b>tab browser</b> dan di <b>halaman login</b>.",
      BEAT.read,
    );
    await say(page, "Kosongkan kolomnya kalau mau kembali ke nama bawaan.", BEAT.read);
    await hush(page);

    // ---- 3. The other title, the one on paper ------------------------------
    await card(
      page,
      "Tapi struknya masih mencetak nama lama.",
      "Nama yang tercetak di struk diatur terpisah.",
      BEAT.card,
    );

    await click(page, app.getByRole("tab", { name: "Pencetakan" }));
    const headerBox = app.getByRole("textbox").nth(0);
    await app.getByText("Header & footer struk").waitFor();
    await say(page, "Ada di <b>Pengaturan ▸ Pencetakan</b>, bagian Header & footer struk.", BEAT.dwell);
    await spotlight(page, app.getByText("Header & footer struk"), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, headerBox);
    await headerBox.fill("");
    await headerBox.pressSequentially(NEW_HEADER, { delay: 45 });
    await page.waitForTimeout(BEAT.short);
    await say(page, "Satu baris di sini = <b>satu baris di struk</b>.", BEAT.dwell);
    await spotlight(page, headerBox, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, app.getByRole("button", { name: "Simpan" }).last());
    await say(
      page,
      "Ini yang tercetak di atas struk — <b>bukan</b> yang tampil di aplikasi.",
      BEAT.dwell,
    );
    await say(page, "Di sebelahnya ada <b>Lebar kertas</b>: 58 mm atau 80 mm, sesuaikan printernya.", BEAT.read);
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Nama di aplikasi ▸ Umum. Nama di struk ▸ Pencetakan.",
      "Dua pengaturan terpisah — mengubah satu tidak mengubah yang lain. Khusus Owner.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    // Put the shop back. These are shop-wide settings on a SHARED dev instance,
    // so a failed restore does not just leave residue — it becomes the starting
    // state of the next take. A silent rpcQuiet here already cost one recording
    // that opened on "Apotek Melati" and narrated a change nobody could see, so
    // this restore reports instead of swallowing, and then checks it landed.
    await setAppTitle(backstage.page, original.appTitle, original.lowStockThreshold).catch(
      (err: Error) => console.warn(`  ! could not restore the app title: ${err.message}`),
    );
    await rpcQuiet(backstage.page, "settings_iface.v1.SettingsService/SetReceiptSettings", {
      header: original.header,
      footer: original.footer,
      width: original.width,
    });

    const after = await rpc(backstage.page, "settings_iface.v1.SettingsService/GetSettings", {}).catch(
      () => null,
    );
    if (after && (after.settings?.appTitle ?? "") !== original.appTitle) {
      console.warn(
        `  ! app title NOT restored: shop is on "${after.settings?.appTitle ?? ""}", ` +
          `expected "${original.appTitle}" — fix before the next recording`,
      );
    }
    await backstage.context.close();
  }
});
