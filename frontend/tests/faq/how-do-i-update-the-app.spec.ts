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

// Recorder for: faq/videos/how-do-i-update-the-app/
//
// "Bagaimana cara memperbarui aplikasi Justmart?"
//
// Unlike the other recorders, the scenario here cannot be seeded over the API —
// what makes the Updates panel interesting is that the RUNNING BUILD is older
// than the latest published release. That comes from the build stamp, so
// playwright.faq.config.ts starts the backend with
// `-ldflags "-X main.version=1.1.0"`; everything else in the take (the release,
// its notes, the download, the checksum) is the real GitHub release, fetched
// live. The guard below refuses to record against an unstamped server rather
// than filming a green "Sudah terbaru" while narrating the opposite.
//
// The video deliberately puts the BACKUP step in the middle of the update flow:
// the question is really "how do I do this without breaking my shop", and the
// one-click backup is the answer to that half.
//
// Two things it cannot show, both covered by a card + question.md: restarting
// Justmart (that happens outside the browser, in the portable launcher) and the
// "Kembalikan ke v…" rollback button, which only appears once a justmart.exe.bak
// exists — i.e. after a real restart-and-swap.
//
// Not a test: it asserts nothing about the product and is excluded from
// `make test-browser`. Run with `make faq-video q=how-do-i-update`.
//
// Residue: clicking "Perbarui sekarang" really downloads the release zip and
// stages justmart.exe.new next to the `go run` temp binary (thrown away with the
// build cache), and writes an `update_prev_version` row into app_settings. Both
// are harmless; there is no RPC to unwrite the latter.

const SLUG = "how-do-i-update-the-app";

test("record: bagaimana cara memperbarui aplikasi", async ({ browser }, testInfo) => {
  test.setTimeout(5 * 60_000);

  const backstage = await openBackstage(browser);

  // Fail loudly, off camera, if the scenario isn't there. The usual cause is a
  // `make dev` backend being reused (playwright's reuseExistingServer), which
  // runs the unstamped "dev" build.
  const info = await rpc(backstage.page, "settings_iface.v1.SettingsService/CheckUpdate", {});
  if (!info.updateAvailable || !info.canSelfApply) {
    await backstage.context.close();
    throw new Error(
      "Updates panel has nothing to record " +
        `(current=${info.currentVersion} latest=${info.latestVersion ?? "?"} ` +
        `checked=${!!info.checked} canSelfApply=${!!info.canSelfApply}).\n` +
        "Stop any running `make dev` backend so this config can start its own " +
        "version-stamped one, and check the machine is online.",
    );
  }

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // Text lookups are scoped to the app, not the document: the studio caption bar
  // lives outside #root and renders its <b> emphasis as real DOM, so narrating
  // "…di <b>Catatan rilis</b>" would otherwise make a bare getByText ambiguous
  // with the very element it is pointing at.
  const app = page.locator("#root");
  let backupName: string | null = null;

  try {
    // ---- Opening -----------------------------------------------------------
    // Card first, THEN wait for the panel — the page loads behind it, so the
    // recording opens on the title instead of on a spinner. `commit` rather than
    // the default load: this panel's first paint waits on a live GitHub round
    // trip, and a plain goto held the take on three seconds of white before the
    // card could be shown. Waiting for the studio card element (mounted by the
    // init script at DOMContentLoaded) is what makes that safe.
    await page.goto("/settings/updates", { waitUntil: "commit" });
    await page.locator("#__faq_card").waitFor({ state: "attached" });
    await card(
      page,
      "Bagaimana cara memperbarui aplikasi Justmart?",
      "Dari dalam aplikasi — Pengaturan ▸ Pembaruan.",
      BEAT.card,
    );
    await app.getByText("Versi saat ini").waitFor();

    // ---- 1. Where it lives -------------------------------------------------
    await say(page, "Semuanya ada di <b>Pengaturan ▸ Pembaruan</b>. Halaman ini khusus Owner.", BEAT.read);
    await spotlight(page, page.getByRole("tab", { name: "Pembaruan" }), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    // ---- 2. What it tells you ----------------------------------------------
    await say(
      page,
      "Justmart membandingkan versi yang sedang jalan dengan <b>rilis terbaru</b>.",
      BEAT.read,
    );
    await spotlight(page, app.getByText("Versi saat ini").locator("xpath=../.."), 4);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(page, "Lencana oranye berarti <b>ada versi yang lebih baru</b>.", BEAT.read);
    await spotlight(page, app.getByText("Pembaruan tersedia"), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(page, "Isi rilisnya bisa dibaca dulu di <b>Catatan rilis</b>.", BEAT.read);
    await spotlight(page, app.getByText("Catatan rilis"));
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 3. Back up first --------------------------------------------------
    await say(page, "Sebelum memperbarui — <b>buat backup dulu</b>. Satu klik.", BEAT.read);
    await click(page, page.getByRole("tab", { name: "Backup" }));
    await page.getByRole("button", { name: "Buat backup" }).waitFor();
    await click(page, page.getByRole("button", { name: "Buat backup" }));

    const firstRow = app.locator("table tbody tr").first();
    await firstRow.waitFor();
    backupName = (await firstRow.locator("td").first().innerText()).trim();
    await say(page, "Backup-nya tersimpan di folder Justmart, lengkap dengan tanggalnya.", BEAT.read);
    await spotlight(page, firstRow, 4);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 4. Update ---------------------------------------------------------
    await click(page, page.getByRole("tab", { name: "Pembaruan" }));
    const updateBtn = page.getByRole("button", { name: "Perbarui sekarang" });
    await updateBtn.waitFor();
    await say(page, "Sekarang klik <b>Perbarui sekarang</b>.", BEAT.read);
    await hush(page);
    await click(page, updateBtn);

    const dialog = page.getByRole("alertdialog");
    await dialog.waitFor();
    await say(
      page,
      "Justmart <b>tidak pernah memperbarui diam-diam</b> — selalu minta konfirmasi dulu.",
      BEAT.dwell,
    );
    await click(page, dialog.getByRole("button", { name: "Perbarui sekarang" }));

    // The download is real (the release zip from GitHub), so give it room.
    const restartHint = app.getByText(/mulai ulang Justmart untuk menyelesaikan/i);
    await restartHint.waitFor({ timeout: 60_000 });
    await say(
      page,
      "Berkasnya diunduh lalu <b>dicocokkan checksum-nya</b>. Kalau tidak cocok, tidak dipasang.",
      BEAT.dwell,
    );
    await spotlight(page, restartHint, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 5. The part that is not in the browser ----------------------------
    await card(
      page,
      "Terakhir: mulai ulang Justmart.",
      "Tutup jendela hitamnya, lalu klik dua kali Start Justmart.bat. Versi barunya dipasang saat itu.",
      BEAT.card + 1200,
    );

    await say(
      page,
      "Yang diganti <b>hanya programnya</b>. Database, pengaturan, dan backup Anda tidak disentuh.",
      BEAT.dwell,
    );
    await say(
      page,
      "Versi lama tetap disimpan — kalau bermasalah, ada tombol <b>Kembalikan ke versi sebelumnya</b> di halaman ini.",
      BEAT.dwell,
    );
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Backup ▸ Perbarui sekarang ▸ mulai ulang.",
      "Tersedia di edisi portable Windows, dan hanya untuk Owner.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);
    if (backupName) {
      await rpcQuiet(backstage.page, "backup_iface.v1.BackupService/DeleteBackup", { name: backupName });
    }
    await backstage.context.close();
  }
});
