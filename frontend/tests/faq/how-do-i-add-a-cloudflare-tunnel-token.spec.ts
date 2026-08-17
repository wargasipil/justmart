import { test } from "@playwright/test";

import {
  BEAT,
  card,
  click,
  hush,
  openBackstage,
  openStage,
  rpc,
  say,
  spotlight,
  unspotlight,
  videoDest,
} from "./_recorder";

// Recorder for: faq/videos/how-do-i-add-a-cloudflare-tunnel-token/
//
// "Bagaimana cara mengisi token tunnel Cloudflare?"
//
// Two things make this recorder different from the others.
//
// 1. The state on screen depends on how the SERVER was started, not on anything
//    seedable. The panel reports which token won — the saved one, config.yaml,
//    or the environment — so a server booted with a token anywhere but the
//    database renders a banner no shop owner will ever see. The backstage check
//    below refuses to record in that case rather than narrating over it. This is
//    why playwright.faq.config.ts hands the backend a COPY of config.yaml with
//    the token stripped instead of relying on the env off-switch: "off" is safe
//    but produces exactly the wrong screen.
//
// 2. The token is a credential, so what is typed on camera must be fake. It is
//    shaped like a real one (base64url, long enough to pass validation) and
//    routes nowhere. It also never starts a tunnel during the take: the token is
//    read once at boot, and this server booted without one.
//
// It writes a shop-wide setting, so it forces a known starting state before
// rolling and clears it again in the finally block — a failed restore here would
// become the starting state of the next take.
//
// Not a test: it asserts nothing and is excluded from `make test-browser`.
// Run with `make faq-video q=how-do-i-add-a-cloudflare-tunnel-token`.

const SLUG = "how-do-i-add-a-cloudflare-tunnel-token";

// Fake, and deliberately so — shaped like a Cloudflare token (base64url, well
// over the 32-char floor) but authenticating nothing.
const DEMO_TOKEN = "eyJhIjoiN2E4YjljMGQxZTJmM2E0YjVjNmQ3ZThmIiwidCI6ImRlbW8ifQ";
// What Cloudflare actually puts on screen under "Install connector": a command
// line, not a bare token. The field takes the token only, so pasting the whole
// line is the mistake almost everyone makes once — which is why it is the
// boundary this recording shows, rather than invented nonsense.
const PASTED_LINE = `cloudflared.exe service install ${DEMO_TOKEN}`;

type BackstagePage = Awaited<ReturnType<typeof openBackstage>>["page"];

const getTunnel = (page: BackstagePage) =>
  rpc(page, "settings_iface.v1.SettingsService/GetTunnelSettings", {});
const setTunnel = (page: BackstagePage, token: string) =>
  rpc(page, "settings_iface.v1.SettingsService/SetTunnelSettings", { token });

test("record: bagaimana cara mengisi token tunnel Cloudflare", async ({ browser }, testInfo) => {
  test.setTimeout(5 * 60_000);

  const backstage = await openBackstage(browser);

  // Clear first, THEN check: a leftover token from an interrupted take would
  // otherwise report source "settings" and fail the guard below for the one
  // reason this recorder can fix by itself.
  await setTunnel(backstage.page, "");

  const state = await getTunnel(backstage.page);
  if (state.source !== "none") {
    throw new Error(
      `the recording server resolves its tunnel token from "${state.source}", not "none".\n` +
        `  A shop owner opening this panel sees "none", so recording now would narrate over\n` +
        `  a banner nobody else gets. Start the backend with a config.yaml whose\n` +
        `  cloudflare_tunnel_token is empty and no JUSTMART_CLOUDFLARE_TUNNEL_TOKEN set —\n` +
        `  playwright.faq.config.ts does this for you, so this most likely means a\n` +
        `  separately started "make run" was reused on the port.`,
    );
  }

  const stage = await openStage(browser, backstage.storageState);
  const page = stage.page;
  // Scoped to the app: the caption bar is real DOM outside #root and renders its
  // <b> emphasis literally, so a bare getByText collides with the narration.
  const app = page.locator("#root");
  const tokenField = app.getByRole("textbox").first();
  const saveButton = app.getByRole("button", { name: "Simpan" });

  try {
    // ---- Opening -----------------------------------------------------------
    await page.goto("/settings/tunnel", { waitUntil: "commit" });
    await page.locator("#__faq_card").waitFor({ state: "attached" });
    await card(
      page,
      "Bagaimana cara mengisi token tunnel Cloudflare?",
      "Supaya toko bisa dibuka dari luar, tanpa utak-atik router.",
      BEAT.card,
    );
    await app.getByText("Token tunnel Cloudflare").waitFor();

    // ---- 1. Where it lives, and what "off" looks like -----------------------
    await say(page, "Buka <b>Pengaturan ▸ Akses jarak jauh</b>. Khusus Owner.", BEAT.read);
    await spotlight(page, app.getByRole("tab", { name: "Akses jarak jauh" }), 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    await say(
      page,
      "Selama masih <b>Mati</b>, aplikasi hanya bisa dibuka dari PC ini dan jaringan lokal.",
      BEAT.dwell,
    );
    await spotlight(page, app.getByText("Mati", { exact: true }), 8);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    // ---- 2. The boundary: the paste that gets refused ------------------------
    // Filmed before the happy path on purpose. Cloudflare shows a command line,
    // so pasting the whole thing is the natural first attempt — showing it fail
    // here means the viewer recognizes the message instead of filing a bug.
    await say(page, "Cloudflare menampilkan <b>satu baris perintah</b>, bukan token polos.", BEAT.read);
    await click(page, tokenField);
    await tokenField.pressSequentially(PASTED_LINE, { delay: 26 });
    await page.waitForTimeout(BEAT.short);
    await say(page, "Kalau seluruh barisnya ditempel, Justmart menolaknya.", BEAT.read);
    await click(page, saveButton);

    const error = app.getByText("Isi tokennya saja", { exact: false });
    await error.waitFor();
    await spotlight(page, error, 6);
    await say(
      page,
      "Kolom ini minta <b>tokennya saja</b> — bagian panjang setelah kata install.",
      BEAT.dwell,
    );
    await unspotlight(page);
    await hush(page);

    // ---- 3. The happy path: the token on its own ----------------------------
    await click(page, tokenField);
    await tokenField.fill("");
    await tokenField.pressSequentially(DEMO_TOKEN, { delay: 32 });
    await page.waitForTimeout(BEAT.short);
    await say(page, "Tanpa perintah, tanpa tanda kutip, tanpa spasi.", BEAT.dwell);
    await spotlight(page, tokenField, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await hush(page);

    await click(page, saveButton);

    // ---- 4. Saved — but nothing is running yet ------------------------------
    // Matched on a fragment unique to the alert. The status line above it also
    // talks about the saved token and about restarting, so the obvious phrases
    // ("Token sudah tersimpan", "Jalankan ulang Justmart") are ambiguous.
    const restart = app.getByText("supaya berlaku", { exact: false });
    await restart.waitFor();
    await say(
      page,
      "Tersimpan. Tapi tunnelnya <b>baru menyala setelah aplikasi dijalankan ulang</b>.",
      BEAT.dwell,
    );
    await spotlight(page, restart, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);

    const savedHelp = app.getByText("tidak pernah ditampilkan utuh", { exact: false });
    await savedHelp.waitFor();
    await say(page, "Setelah ini tokennya <b>tidak pernah ditampilkan utuh lagi</b>.", BEAT.dwell);
    await spotlight(page, savedHelp, 6);
    await page.waitForTimeout(BEAT.short);
    await unspotlight(page);
    await say(page, "Mau mematikannya lagi: <b>Hapus token</b>, lalu jalankan ulang.", BEAT.read);
    await hush(page);

    // ---- Closing -----------------------------------------------------------
    await card(
      page,
      "Simpan token ▸ jalankan ulang Justmart.",
      "Alamatnya publik, tapi tetap berhenti di halaman login — pastikan password Owner kuat.",
      BEAT.card + 800,
    );

    const written = await stage.save(videoDest(testInfo, SLUG));
    console.log(`\n  ✓ tutorial written to ${written}\n`);
  } finally {
    await stage.context.close().catch(() => undefined);

    // Leave the shop with no tunnel token. This is shop-wide state on a SHARED
    // dev instance: a silent failure here does not just leave residue, it
    // becomes the opening state of the next take — and the guard at the top
    // would then refuse to record at all. So report it, then verify it landed.
    await setTunnel(backstage.page, "").catch((err: Error) =>
      console.warn(`  ! could not clear the tunnel token: ${err.message}`),
    );
    const after = await getTunnel(backstage.page).catch(() => null);
    if (after && after.configured) {
      console.warn(
        "  ! the demo tunnel token is STILL saved — clear it in Settings ▸ Akses jarak jauh " +
          "before the next recording (it is fake, so no tunnel can come up, but the next " +
          "take would open on the wrong screen)",
      );
    }
    await backstage.context.close();
  }
});
