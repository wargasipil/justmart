import { execFile } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

import type { Browser, BrowserContext, Locator, Page, TestInfo } from "@playwright/test";

import { loginAs, OWNER } from "../e2e/_helpers";

// Toolkit for the FAQ tutorial recorders (see faq/README.md).
//
// These are NOT tests — nothing is asserted, and a recorder is not part of
// `make test-browser`. They drive the real app through a real flow and capture
// it to faq/videos/<question>/tutorial.webm, so an FAQ answer can show the
// thing instead of describing it. They live under frontend/tests/ purely
// because that is where node_modules resolves from; the artifact they produce
// lives in faq/.
//
// Two things a recording needs that a test does not:
//   - It must be legible. Playwright renders no cursor and gives no narration,
//     so a raw capture is a UI twitching by itself. `studioChrome` injects a
//     caption bar, a spotlight ring and a synthetic cursor into every page.
//   - It must own its video file. Playwright's own `video:` option writes an
//     opaque name into the output dir, so the recorder opens its OWN context
//     with `recordVideo` and saves the result to a path we choose.

export const VIDEO_SIZE = { width: 1280, height: 720 };

/**
 * Where a recorder writes its artifact: faq/videos/<slug>/tutorial.webm.
 *
 * Resolved from the config FILE, not `config.rootDir` — Playwright sets rootDir
 * to the common ancestor of the test dirs (frontend/tests/faq), so resolving
 * against it buries the video inside the test tree instead of the repo's faq/.
 */
export function videoDest(testInfo: TestInfo, slug: string): string {
  const frontendDir = testInfo.config.configFile
    ? path.dirname(testInfo.config.configFile)
    : path.resolve(testInfo.config.rootDir, "..", "..");
  return path.resolve(frontendDir, "..", "faq", "videos", slug, "tutorial.webm");
}

/** Beat lengths (ms). Named so pacing is tuned in one place, not per line. */
export const BEAT = {
  /** Between two mechanical steps — long enough to see what moved. */
  short: 900,
  /** A caption the viewer only needs to glance at. */
  read: 2600,
  /** A caption carrying the actual answer. */
  dwell: 4200,
  /** A full-screen title / outro card. */
  card: 3800,
} as const;

/**
 * Injected into every page of the recording context, on every navigation.
 * Self-contained by necessity — addInitScript serializes the function, so it
 * can close over nothing.
 */
function studioChrome(): void {
  const ID = "__faq_studio";
  const mount = () => {
    if (document.getElementById(ID)) return;
    const root = document.createElement("div");
    root.id = ID;
    root.innerHTML = `
<style>
  #${ID} {
    position: fixed; inset: 0; z-index: 2147483647; pointer-events: none;
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  }
  #__faq_ring {
    position: fixed; left: 0; top: 0; width: 0; height: 0; border-radius: 10px;
    opacity: 0; transition: opacity .3s ease, left .4s cubic-bezier(.4,0,.2,1),
      top .4s cubic-bezier(.4,0,.2,1), width .4s cubic-bezier(.4,0,.2,1),
      height .4s cubic-bezier(.4,0,.2,1);
    box-shadow: 0 0 0 3px #3b82f6, 0 0 0 9999px rgba(15,23,42,.42);
  }
  #__faq_ring.on { opacity: 1; }
  #__faq_caption {
    position: fixed; left: 50%; bottom: 34px; transform: translateX(-50%);
    max-width: 78%; padding: 15px 24px; border-radius: 14px;
    background: rgba(15,23,42,.94); color: #f8fafc;
    font-size: 20px; line-height: 1.45; text-align: center;
    box-shadow: 0 12px 34px rgba(0,0,0,.4);
    opacity: 0; transition: opacity .25s ease;
  }
  #__faq_caption.on { opacity: 1; }
  #__faq_caption b { color: #93c5fd; font-weight: 600; }
  #__faq_card {
    position: fixed; inset: 0; display: flex; flex-direction: column;
    align-items: center; justify-content: center; gap: 18px; text-align: center;
    background: #0f172a; color: #f8fafc;
    opacity: 0; transition: opacity .4s ease;
  }
  #__faq_card.on { opacity: 1; }
  #__faq_card h1 { font-size: 40px; font-weight: 700; line-height: 1.25; max-width: 78%; margin: 0; }
  #__faq_card p { font-size: 21px; color: #94a3b8; line-height: 1.5; max-width: 66%; margin: 0; }
  #__faq_cursor {
    position: fixed; left: 0; top: 0; width: 22px; height: 22px; margin: -11px 0 0 -11px;
    border-radius: 50%; background: rgba(59,130,246,.3); border: 2px solid #3b82f6;
    opacity: 0; transition: opacity .2s ease;
  }
  #__faq_cursor.on { opacity: 1; }
  #__faq_cursor.tap { background: rgba(59,130,246,.75); }
  /* Dev-only chrome that does not exist in a real install. The TanStack Query
     devtools toggle sits in the bottom-right corner and would otherwise appear
     in every recording, in a video whose whole claim is "this is your app". */
  .tsqd-parent-container, .tsqd-open-btn-container, #tsqd-open-btn { display: none !important; }
</style>
<div id="__faq_ring"></div>
<div id="__faq_cursor"></div>
<div id="__faq_caption"></div>
<div id="__faq_card"><h1></h1><p></p></div>`;
    document.body.appendChild(root);

    const cursor = document.getElementById("__faq_cursor") as HTMLElement;
    window.addEventListener(
      "mousemove",
      (e) => {
        cursor.classList.add("on");
        cursor.style.transform = `translate(${e.clientX}px, ${e.clientY}px)`;
      },
      true,
    );
    window.addEventListener("mousedown", () => cursor.classList.add("tap"), true);
    window.addEventListener("mouseup", () => cursor.classList.remove("tap"), true);
  };
  if (document.body) mount();
  else document.addEventListener("DOMContentLoaded", mount);
}

// --- music -------------------------------------------------------------------

const run = promisify(execFile);

export const MUSIC = {
  /**
   * The looping bed, relative to faq/. Generated by faq/tools/make_music.py —
   * synthesized rather than licensed, so a video we hand to a shop owner
   * carries no third-party rights. Swap this file to change the music.
   */
  asset: path.join("assets", "music", "corporate-bed.opus"),
  /** Trim on top of the asset's own mastering (-20 dBFS RMS). */
  gainDb: -3,
  fadeIn: 1.5,
  fadeOut: 2.5,
} as const;

async function probeDuration(file: string): Promise<number> {
  const { stdout } = await run("ffprobe", [
    "-v", "error",
    "-show_entries", "format=duration",
    "-of", "default=nw=1:nk=1",
    file,
  ]);
  const seconds = Number.parseFloat(stdout.trim());
  if (!Number.isFinite(seconds)) throw new Error(`ffprobe gave no duration for ${file}`);
  return seconds;
}

/**
 * Mix the looping bed under the silent capture, fading in and out.
 *
 * The bed is looped (`-stream_loop -1`) and cut to the video's own length, so a
 * recording is never bounded by how long the music happens to be — one asset
 * scores every question. Video is `-c:v copy`: scoring must not re-encode what
 * the recorder just captured.
 *
 * Returns false when ffmpeg or the bed is unavailable — a missing tool degrades
 * the video to silent, it does not throw away a recording that took minutes.
 */
async function score(silent: string, music: string, dest: string): Promise<boolean> {
  try {
    await fs.access(music);
  } catch {
    console.warn(`  ! music bed not found at ${music} — writing a silent video`);
    return false;
  }
  try {
    const duration = await probeDuration(silent);
    const fadeStart = Math.max(0, duration - MUSIC.fadeOut);
    await run("ffmpeg", [
      "-y", "-loglevel", "error",
      "-i", silent,
      "-stream_loop", "-1", "-i", music,
      "-filter_complex",
      `[1:a]volume=${MUSIC.gainDb}dB,` +
        `afade=t=in:st=0:d=${MUSIC.fadeIn},` +
        `afade=t=out:st=${fadeStart.toFixed(3)}:d=${MUSIC.fadeOut}[a]`,
      "-map", "0:v", "-map", "[a]",
      "-c:v", "copy",
      "-c:a", "libopus", "-b:a", "96k",
      // Explicit -t rather than -shortest: the looped bed is infinite, so the
      // output length must come from the video, stated outright.
      "-t", duration.toFixed(3),
      dest,
    ]);
    return true;
  } catch (err) {
    console.warn(`  ! could not add music (${(err as Error).message.split("\n")[0]}) — writing a silent video`);
    return false;
  }
}

export type Stage = {
  context: BrowserContext;
  page: Page;
  /**
   * Close the recording, score it, and write the .webm to `dest` (absolute).
   * The music bed is found relative to `dest`'s faq/ root; pass `music: null`
   * to keep a recording silent.
   */
  save: (dest: string, opts?: { music?: string | null }) => Promise<string>;
};

/**
 * A signed-in browser context that is NOT recorded — used to seed the scenario
 * before the camera rolls, and to clean it up after. Returns its storage state
 * so the recording context starts already authenticated (no login on camera,
 * and no credentials in the video).
 */
export async function openBackstage(browser: Browser): Promise<{
  context: BrowserContext;
  page: Page;
  storageState: Awaited<ReturnType<BrowserContext["storageState"]>>;
}> {
  const context = await browser.newContext({ viewport: VIDEO_SIZE });
  const page = await context.newPage();
  await loginAs(page, OWNER);

  // Same reason global.setup.ts does this: without an explicit active
  // warehouse the app may fall back to whichever one sorts first, and the
  // scenario would be seeded into one warehouse and read from another.
  await page.evaluate(async () => {
    const token = localStorage.getItem("justmart_access_token");
    const res = await fetch("/api/warehouse_iface.v1.WarehouseService/ListUserWarehouses", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ userId: "", limit: 1000 }),
    });
    const data = await res.json();
    const def = (data.memberships || []).find((m: { isDefault?: boolean }) => m.isDefault);
    const id = def?.warehouseId ?? data.warehouses?.[0]?.id;
    if (id) localStorage.setItem("justmart_warehouse_id", id);
  });

  return { context, page, storageState: await context.storageState() };
}

/** Open the recording context: authenticated, Indonesian UI, studio chrome on. */
export async function openStage(
  browser: Browser,
  storageState: Awaited<ReturnType<BrowserContext["storageState"]>>,
): Promise<Stage> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "justmart-faq-"));
  const context = await browser.newContext({
    storageState,
    viewport: VIDEO_SIZE,
    recordVideo: { dir, size: VIDEO_SIZE },
    locale: "id-ID",
  });
  // i18next detects the language from localStorage first (lib/i18n.ts,
  // lookupLocalStorage: "justmart_lang"). Without this it falls through to
  // navigator and the tutorial would be narrated in Indonesian over an English
  // UI. An init script beats the app's own bootstrap on every navigation.
  await context.addInitScript(() => localStorage.setItem("justmart_lang", "id"));
  await context.addInitScript(studioChrome);

  const page = await context.newPage();

  const save = async (dest: string, opts: { music?: string | null } = {}): Promise<string> => {
    const video = page.video();
    if (!video) throw new Error("recordVideo was not enabled on the stage context");
    await context.close(); // finalizes the .webm
    await fs.mkdir(path.dirname(dest), { recursive: true });
    await fs.rm(dest, { force: true });

    const silent = path.join(dir, "silent.webm");
    await video.saveAs(silent);
    await video.delete().catch(() => {});

    // dest is <repo>/faq/videos/<slug>/tutorial.webm by convention, so faq/ is
    // three levels up — that is what makes the bed a property of the FAQ rather
    // than something each recorder has to thread through.
    const music =
      opts.music === undefined
        ? path.resolve(path.dirname(dest), "..", "..", MUSIC.asset)
        : opts.music;

    if (!music || !(await score(silent, music, dest))) {
      await fs.copyFile(silent, dest);
    }
    await fs.rm(dir, { recursive: true, force: true }).catch(() => {});
    return dest;
  };

  return { context, page, save };
}

/** POST a Connect JSON RPC as the signed-in user. Throws on a non-2xx. */
export async function rpc<T = any>(page: Page, procedure: string, body: unknown): Promise<T> {
  return await page.evaluate(
    async (a: { procedure: string; body: unknown }) => {
      const token = localStorage.getItem("justmart_access_token");
      if (!token) throw new Error("not signed in: no access token in localStorage");
      const res = await fetch(`/api/${a.procedure}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(a.body),
      });
      if (!res.ok) throw new Error(`${a.procedure}: ${res.status} ${await res.text()}`);
      return res.json();
    },
    { procedure, body },
  );
}

/** Best-effort RPC for teardown — a failed cleanup must not fail a recording. */
export async function rpcQuiet(page: Page, procedure: string, body: unknown): Promise<void> {
  await rpc(page, procedure, body).catch(() => undefined);
}

// --- narration ---------------------------------------------------------------

/** Show a caption and hold it. `text` may contain <b> for emphasis. */
export async function say(page: Page, text: string, hold: number = BEAT.read): Promise<void> {
  await page.evaluate((t) => {
    const el = document.getElementById("__faq_caption");
    if (!el) return;
    el.innerHTML = t;
    el.classList.add("on");
  }, text);
  await page.waitForTimeout(hold);
}

export async function hush(page: Page): Promise<void> {
  await page.evaluate(() => document.getElementById("__faq_caption")?.classList.remove("on"));
  await page.waitForTimeout(300);
}

/** Full-screen title / outro card. Covers the app while it holds. */
export async function card(
  page: Page,
  title: string,
  subtitle = "",
  hold: number = BEAT.card,
): Promise<void> {
  await page.evaluate(
    (a: { title: string; subtitle: string }) => {
      const el = document.getElementById("__faq_card");
      if (!el) return;
      el.querySelector("h1")!.textContent = a.title;
      el.querySelector("p")!.textContent = a.subtitle;
      el.classList.add("on");
    },
    { title, subtitle },
  );
  await page.waitForTimeout(hold);
  await page.evaluate(() => document.getElementById("__faq_card")?.classList.remove("on"));
  await page.waitForTimeout(500);
}

/** Ring the target and dim everything else. Scrolls it into view first. */
export async function spotlight(page: Page, target: Locator, pad = 8): Promise<void> {
  await target.scrollIntoViewIfNeeded();
  await page.waitForTimeout(350); // let smooth-scroll settle before measuring
  const box = await target.boundingBox();
  if (!box) throw new Error("spotlight: target has no bounding box (not visible?)");
  await page.evaluate(
    (b: { x: number; y: number; w: number; h: number }) => {
      const ring = document.getElementById("__faq_ring");
      if (!ring) return;
      ring.style.left = `${b.x}px`;
      ring.style.top = `${b.y}px`;
      ring.style.width = `${b.w}px`;
      ring.style.height = `${b.h}px`;
      ring.classList.add("on");
    },
    { x: box.x - pad, y: box.y - pad, w: box.width + pad * 2, h: box.height + pad * 2 },
  );
  await page.waitForTimeout(450);
}

export async function unspotlight(page: Page): Promise<void> {
  await page.evaluate(() => document.getElementById("__faq_ring")?.classList.remove("on"));
  await page.waitForTimeout(320);
}

/** Glide the synthetic cursor to the target, pause, then click it. */
export async function click(page: Page, target: Locator): Promise<void> {
  await target.scrollIntoViewIfNeeded();
  const box = await target.boundingBox();
  if (box) {
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 24 });
    await page.waitForTimeout(400);
  }
  await target.click();
  await page.waitForTimeout(BEAT.short);
}

/** Type at a human rate so the viewer can follow what is being entered. */
export async function type(page: Page, target: Locator, text: string): Promise<void> {
  await click(page, target);
  await target.pressSequentially(text, { delay: 55 });
  await page.waitForTimeout(BEAT.short);
}
