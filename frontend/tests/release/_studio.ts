import { existsSync } from "node:fs";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Browser, BrowserContext, Page } from "@playwright/test";

import { card as drawCard, say as drawSay } from "../faq/_recorder";

// The narration vocabulary is shared with the FAQ recorders — say / card /
// spotlight / click / type all drive DOM ids that `mountStudioChrome` below
// recreates, so the two studios stay interchangeable at the call site.
//
// `say` and `card` are re-exported through a wrapper rather than directly: a
// YouTube upload wants a subtitle track, and the only way it can be guaranteed
// to match the burned-in captions is to derive it from the same call that draws
// them. Anything hand-written drifts on the first re-record.
export { BEAT, click, hush, spotlight, type, unspotlight } from "../faq/_recorder";

// What differs here is the medium. An FAQ clip is a small inline player; a
// release video is a full-screen YouTube page, so this stage records at 1080p
// and scales the caption/ring/cursor chrome to match. That is the whole reason
// this file exists rather than reusing `faq/_recorder`'s openStage.
export const VIDEO_SIZE = { width: 1920, height: 1080 };
const CHROME_SCALE = 1.5;

export type Stage = {
  context: BrowserContext;
  page: Page;
  /**
   * Signed-in Connect call, made from node — NOT through the page. A recorder
   * needs to look things up before the camera has navigated anywhere (the
   * page sits on about:blank, where reading localStorage throws a
   * SecurityError), and a lookup is not something the viewer should watch.
   */
  api: <T = any>(procedure: string, body?: unknown) => Promise<T>;
  /** Cues collected by `say`/`card` — write with `subtitles.write(dest)`. */
  subtitles: Subtitles;
  /** Elapsed ms since the camera started — the chapter clock. */
  elapsed: () => number;
  /** Close the recording and write the .webm to `dest` (absolute path). */
  save: (dest: string) => Promise<string>;
};

/**
 * Injected on every navigation. Self-contained by necessity: addInitScript
 * serializes the function, so it can close over nothing and takes its scale as
 * an argument.
 */
function mountStudioChrome(scale: number): void {
  const ID = "__faq_studio";
  const px = (n: number) => `${Math.round(n * scale)}px`;
  const mount = () => {
    if (document.getElementById(ID)) return;
    const root = document.createElement("div");
    root.id = ID;
    root.innerHTML = [
      "<style>",
      `#${ID} { position: fixed; inset: 0; z-index: 2147483647; pointer-events: none;`,
      '  font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; }',
      `#__faq_ring { position: fixed; left: 0; top: 0; width: 0; height: 0; border-radius: ${px(10)};`,
      "  opacity: 0; transition: opacity .3s ease, left .4s cubic-bezier(.4,0,.2,1),",
      "    top .4s cubic-bezier(.4,0,.2,1), width .4s cubic-bezier(.4,0,.2,1), height .4s cubic-bezier(.4,0,.2,1);",
      `  box-shadow: 0 0 0 ${px(3)} #3b82f6, 0 0 0 9999px rgba(15,23,42,.42); }`,
      "#__faq_ring.on { opacity: 1; }",
      `#__faq_caption { position: fixed; left: 50%; bottom: ${px(34)}; transform: translateX(-50%);`,
      `  max-width: 78%; padding: ${px(15)} ${px(24)}; border-radius: ${px(14)};`,
      `  background: rgba(15,23,42,.94); color: #f8fafc; font-size: ${px(20)}; line-height: 1.45;`,
      `  text-align: center; box-shadow: 0 ${px(12)} ${px(34)} rgba(0,0,0,.4);`,
      "  opacity: 0; transition: opacity .25s ease; }",
      "#__faq_caption.on { opacity: 1; }",
      "#__faq_caption b { color: #93c5fd; font-weight: 600; }",
      "#__faq_card { position: fixed; inset: 0; display: flex; flex-direction: column;",
      `  align-items: center; justify-content: center; gap: ${px(18)}; text-align: center;`,
      "  background: #0f172a; color: #f8fafc; opacity: 0; transition: opacity .4s ease; }",
      "#__faq_card.on { opacity: 1; }",
      `#__faq_card h1 { font-size: ${px(40)}; font-weight: 700; line-height: 1.25; max-width: 78%; margin: 0; }`,
      `#__faq_card p { font-size: ${px(21)}; color: #94a3b8; line-height: 1.5; max-width: 66%; margin: 0; }`,
      `#__faq_cursor { position: fixed; left: 0; top: 0; width: ${px(22)}; height: ${px(22)};`,
      `  margin: ${px(-11)} 0 0 ${px(-11)}; border-radius: 50%; background: rgba(59,130,246,.3);`,
      `  border: ${px(2)} solid #3b82f6; opacity: 0; transition: opacity .2s ease; }`,
      "#__faq_cursor.on { opacity: 1; }",
      "#__faq_cursor.tap { background: rgba(59,130,246,.75); }",
      // Dev-only chrome that does not exist in a real install. Harmless against
      // the packaged demo build, but a recording made against `make web` would
      // otherwise carry the TanStack Query devtools toggle in the corner of a
      // video whose whole claim is "this is your app".
      ".tsqd-parent-container, .tsqd-open-btn-container, #tsqd-open-btn { display: none !important; }",
      "</style>",
      '<div id="__faq_ring"></div>',
      '<div id="__faq_cursor"></div>',
      '<div id="__faq_caption"></div>',
      '<div id="__faq_card"><h1></h1><p></p></div>',
    ].join("\n");
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

/**
 * Sign in over the API and hand the tokens to the browser, rather than driving
 * the login form in a throwaway context first.
 *
 * The reason is the login rate limiter: 5 attempts per email, refilling one a
 * minute. Iterating on a recording means running it many times in a row, and a
 * form-driven login burns the bucket within a few takes — after which every run
 * dies at a bare navigation timeout that looks nothing like its cause.
 */
async function signIn(baseURL: string, email: string, password: string) {
  const post = async (procedure: string, body: unknown, token?: string) => {
    const res = await fetch(`${baseURL}/api/${procedure}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    const text = await res.text();
    if (!res.ok) throw new Error(`${procedure} -> ${res.status} ${text}`);
    return JSON.parse(text);
  };

  const auth = await post("user_iface.v1.AuthService/Login", { email, password });

  // Same reason global.setup.ts pins this: without an explicit active
  // warehouse the app can fall back to whichever one sorts first, and the
  // recording would read stock from a warehouse the seed never filled.
  const whs = await post(
    "warehouse_iface.v1.WarehouseService/ListUserWarehouses",
    { userId: "", limit: 1000 },
    auth.accessToken,
  );
  const warehouseId =
    (whs.memberships ?? []).find((m: { isDefault?: boolean }) => m.isDefault)?.warehouseId ??
    whs.warehouses?.[0]?.id ??
    "";

  const api = <T = any>(procedure: string, body: unknown = {}) =>
    post(procedure, body, auth.accessToken) as Promise<T>;

  return { access: auth.accessToken, refresh: auth.refreshToken, warehouseId, api };
}

export type OpenTourOptions = {
  baseURL: string;
  email: string;
  password: string;
};

/** Open the recording context: signed in, Indonesian UI, studio chrome on. */
export async function openTour(browser: Browser, opts: OpenTourOptions): Promise<Stage> {
  const { access, refresh, warehouseId, api } = await signIn(opts.baseURL, opts.email, opts.password);

  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "justmart-release-"));
  const context = await browser.newContext({
    viewport: VIDEO_SIZE,
    recordVideo: { dir, size: VIDEO_SIZE },
    locale: "id-ID",
  });

  // i18next reads the language from localStorage first (lib/i18n.ts,
  // lookupLocalStorage: "justmart_lang"). Without this the narration would be
  // Indonesian over an English UI.
  await context.addInitScript(
    (a: { access: string; refresh: string; warehouse: string }) => {
      localStorage.setItem("justmart_lang", "id");
      localStorage.setItem("justmart_access_token", a.access);
      localStorage.setItem("justmart_refresh_token", a.refresh);
      if (a.warehouse) localStorage.setItem("justmart_warehouse_id", a.warehouse);
    },
    { access, refresh, warehouse: warehouseId },
  );
  await context.addInitScript(mountStudioChrome, CHROME_SCALE);

  const page = await context.newPage();
  const t0 = Date.now();
  const elapsed = () => Date.now() - t0;
  const subtitles = new Subtitles();
  narration.set(page, { subs: subtitles, elapsed });

  const save = async (dest: string): Promise<string> => {
    const video = page.video();
    if (!video) throw new Error("recordVideo was not enabled on the stage context");
    await context.close(); // finalizes the .webm
    await fs.mkdir(path.dirname(dest), { recursive: true });
    await fs.rm(dest, { force: true });
    await video.saveAs(dest);
    await video.delete().catch(() => {});
    await fs.rm(dir, { recursive: true, force: true }).catch(() => {});
    return dest;
  };

  return { context, page, api, subtitles, elapsed, save };
}

/**
 * Repo root, found by walking up from this file until the Makefile appears.
 *
 * `testInfo.config.rootDir` is NOT it — Playwright sets rootDir to the config's
 * testDir (here frontend/tests/release), so resolving output paths against it
 * silently buries artifacts under the test tree instead of in release/.
 */
export function repoRoot(): string {
  // ESM here, so no __dirname.
  let dir = path.dirname(fileURLToPath(import.meta.url));
  for (let i = 0; i < 8; i++) {
    if (existsSync(path.join(dir, "Makefile"))) return dir;
    dir = path.dirname(dir);
  }
  throw new Error("repoRoot: no Makefile found above tests/release");
}

// --- subtitles ---------------------------------------------------------------

/**
 * SRT cues collected from the narration calls themselves.
 *
 * The captions are burned into the picture, so a subtitle track is not what the
 * viewer reads — it is what YouTube indexes for search and what a viewer with
 * autotranslate turned on gets. It therefore has to say exactly what is on
 * screen at exactly the moment it is on screen, which is only true if both come
 * from one call.
 */
export class Subtitles {
  private readonly cues: { start: number; end: number; text: string }[] = [];

  add(text: string, start: number, end: number): void {
    // A cue shorter than a beat is a flash nobody can read, and SRT tools
    // dislike zero-length cues.
    if (end - start < 400) return;
    this.cues.push({ start, end, text });
  }

  get length(): number {
    return this.cues.length;
  }

  toSrt(): string {
    const stamp = (ms: number) => {
      const t = Math.max(0, Math.round(ms));
      const h = String(Math.floor(t / 3_600_000)).padStart(2, "0");
      const m = String(Math.floor(t / 60_000) % 60).padStart(2, "0");
      const s = String(Math.floor(t / 1000) % 60).padStart(2, "0");
      const f = String(t % 1000).padStart(3, "0");
      return `${h}:${m}:${s},${f}`;
    };
    return (
      this.cues
        .map((c, i) => `${i + 1}\n${stamp(c.start)} --> ${stamp(c.end)}\n${c.text}\n`)
        .join("\n") + "\n"
    );
  }

  async write(dest: string): Promise<void> {
    await fs.mkdir(path.dirname(dest), { recursive: true });
    // No BOM: YouTube accepts plain UTF-8 SRT and chokes less often without one.
    await fs.writeFile(dest, this.toSrt(), "utf8");
  }
}

/**
 * Per-page narration state, so the wrapped `say`/`card` keep the FAQ signature
 * (`say(page, ...)`) and the tour reads the same as an FAQ recorder. Keyed on
 * the Page rather than held in a module variable so nothing leaks between runs.
 */
const narration = new WeakMap<Page, { subs: Subtitles; elapsed: () => number }>();

/** Strip the caption's <b> emphasis — SRT carries no markup. */
const plain = (html: string) => html.replace(/<[^>]+>/g, "").replace(/\s+/g, " ").trim();

/** Show a caption, hold it, and record the cue. Same signature as faq/_recorder. */
export async function say(page: Page, text: string, hold?: number): Promise<void> {
  const state = narration.get(page);
  const start = state?.elapsed() ?? 0;
  await drawSay(page, text, hold);
  state?.subs.add(plain(text), start, state.elapsed());
}

/** Full-screen title / outro card, recorded as a two-line cue. */
export async function card(
  page: Page,
  title: string,
  subtitle = "",
  hold?: number,
): Promise<void> {
  const state = narration.get(page);
  const start = state?.elapsed() ?? 0;
  await drawCard(page, title, subtitle, hold);
  state?.subs.add([title, subtitle].filter(Boolean).map(plain).join("\n"), start, state.elapsed());
}

// --- chapters ----------------------------------------------------------------

export type Chapter = { at: number; title: string };

/**
 * YouTube chapters have to be real timestamps, and the only thing that knows
 * when a section actually started is the recorder itself — beat lengths drift
 * the moment a `waitFor` takes longer than the take before. So the tour stamps
 * them as it goes and the description is generated from the result.
 */
export class Chapters {
  private readonly items: Chapter[] = [];

  constructor(private readonly stage: Stage) {}

  mark(title: string): void {
    this.items.push({ at: Math.max(0, Math.round(this.stage.elapsed() / 1000)), title });
  }

  /** `0:00 Title` lines, first one forced to 0:00 as YouTube requires. */
  toList(): Chapter[] {
    return this.items.map((c, i) => (i === 0 ? { ...c, at: 0 } : c));
  }

  async write(dest: string): Promise<void> {
    await fs.mkdir(path.dirname(dest), { recursive: true });
    await fs.writeFile(dest, JSON.stringify(this.toList(), null, 2) + "\n", "utf8");
  }
}
