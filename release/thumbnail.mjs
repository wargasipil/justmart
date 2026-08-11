// Build the YouTube thumbnail(s) from frames of the finished video.
//
// Rendered as HTML and screenshotted with the same Playwright that recorded the
// tour, rather than composited with ffmpeg's drawtext: the text needs real
// layout — weight, tracking, a wrap that breaks where it should — and drawtext
// gives none of that (nor a readable escape story for Windows font paths).
//
// A YouTube thumbnail is displayed at ~360px wide in a feed and full width on a
// TV, so the rule that drives every choice here is: at most four or five words,
// set big enough to read at thumbnail size, over a screenshot that is dimmed
// and zoomed rather than shown whole. A full 1920px UI shrunk to 360px is grey
// mush.
//
// Usage:
//   node release/thumbnail.mjs                    # both variants
//   node release/thumbnail.mjs --only kasir       # just one

import { execFile } from "node:child_process";
import { mkdir, readFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);
const HERE = path.dirname(fileURLToPath(import.meta.url));

// Playwright lives in frontend/node_modules, and release/ is not below it, so
// normal resolution walks past it to the repo root and finds nothing. Resolve
// from the frontend package explicitly rather than adding a second install.
const { chromium } = createRequire(path.join(HERE, "..", "frontend", "package.json"))("playwright");
const VIDEO = path.join(HERE, "video", "justmart-product-tour-1080p.mp4");
const OUT_DIR = path.join(HERE, "video");
const TMP = path.join(HERE, "video", "_thumb-frames");

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, a, i, arr) => {
    if (a.startsWith("--")) acc.push([a.slice(2), arr[i + 1]]);
    return acc;
  }, []),
);

// Frame times are chapter-relative rather than absolute so a re-record that
// shifts the timeline by a few seconds still lands inside the right screen.
const chapters = JSON.parse(await readFile(path.join(HERE, "video", "chapters.json"), "utf8"));
const chapterAt = (title) => chapters.find((c) => c.title.startsWith(title))?.at ?? 0;

// `posX`/`posY` are CSS background-position percentages: with the shot scaled
// past 100%, 0% pins its left edge and 100% its right. The copy sits on the
// right, so each variant pans LEFT — onto the part of the screen that still
// reads as an app once the thumbnail is 360px wide.
const VARIANTS = [
  {
    name: "kasir",
    // Cart filled and the product list populated. Earlier in the chapter the
    // list is still empty and the shot is mostly white space.
    at: chapterAt("Kasir") + 29,
    headline: "Kasir + Stok",
    kicker: "SATU APLIKASI",
    sub: "Toko & Apotek",
    // Pinned near the left edge: a POS row is a name on the left and a price on
    // the right with air between, so cropping into the middle of the list gives
    // half-names over an empty band.
    focus: { scale: 1.3, posX: 4, posY: 22 },
  },
  {
    name: "laporan",
    // The Grafik tab, not the table: four coloured charts survive being shrunk
    // to a feed thumbnail; a wall of numbers turns into grey texture.
    at: chapterAt("Analitik") + 14,
    headline: "Untung Rugi",
    kicker: "OTOMATIS DARI KASIR",
    sub: "Terjual · HPP · Profit",
    focus: { scale: 1.4, posX: 32, posY: 58 },
  },
];

const wanted = args.only ? VARIANTS.filter((v) => v.name === args.only) : VARIANTS;
if (!wanted.length) throw new Error(`no variant named "${args.only}"`);

await mkdir(TMP, { recursive: true });

const html = (frameDataUri, v) => `
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html, body { width: 1280px; height: 720px; overflow: hidden; }
  body {
    position: relative; background: #0f172a;
    font-family: "Segoe UI", ui-sans-serif, system-ui, Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
  .shot {
    position: absolute; inset: 0;
    background-image: url("${frameDataUri}");
    background-size: ${v.focus.scale * 100}%;
    background-position: ${v.focus.posX}% ${v.focus.posY}%;
    background-repeat: no-repeat;
  }
  /* Darken the right-hand two thirds so the type has somewhere to sit without
     hiding the part of the UI that makes the shot recognisable. */
  .scrim {
    position: absolute; inset: 0;
    background: linear-gradient(100deg,
      rgba(15,23,42,.10) 0%, rgba(15,23,42,.55) 34%,
      rgba(15,23,42,.90) 60%, rgba(15,23,42,.96) 100%);
  }
  .copy {
    position: absolute; right: 56px; top: 50%; transform: translateY(-50%);
    width: 660px; text-align: right; color: #f8fafc;
  }
  .kicker {
    font-size: 30px; font-weight: 700; letter-spacing: .16em;
    color: #7dd3fc; margin-bottom: 14px;
  }
  .headline {
    font-size: 132px; font-weight: 800; line-height: .96; letter-spacing: -.03em;
    text-shadow: 0 6px 30px rgba(0,0,0,.55);
  }
  .sub {
    margin-top: 22px; font-size: 44px; font-weight: 600; color: #cbd5e1;
  }
  .rule {
    margin: 26px 0 0 auto; width: 190px; height: 8px; border-radius: 4px;
    background: #3b82f6;
  }
  .brand {
    position: absolute; left: 48px; bottom: 40px;
    display: flex; align-items: center; gap: 14px;
    color: #f8fafc; font-size: 40px; font-weight: 700; letter-spacing: -.01em;
    text-shadow: 0 4px 18px rgba(0,0,0,.6);
  }
  .dot { width: 22px; height: 22px; border-radius: 6px; background: #3b82f6; }
</style>
<div class="shot"></div>
<div class="scrim"></div>
<div class="copy">
  <div class="kicker">${v.kicker}</div>
  <div class="headline">${v.headline}</div>
  <div class="sub">${v.sub}</div>
  <div class="rule"></div>
</div>
<div class="brand"><span class="dot"></span>Justmart</div>
`;

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1280, height: 720 }, deviceScaleFactor: 1 });

for (const v of wanted) {
  const frame = path.join(TMP, `${v.name}.png`);
  await run("ffmpeg", [
    "-y", "-loglevel", "error",
    "-ss", String(v.at),
    "-i", VIDEO,
    "-frames:v", "1",
    frame,
  ]);
  const dataUri = `data:image/png;base64,${(await readFile(frame)).toString("base64")}`;

  const dest = path.join(OUT_DIR, `thumbnail-${v.name}.jpg`);
  await page.setContent(html(dataUri, v), { waitUntil: "load" });
  // JPEG at q92: YouTube's thumbnail ceiling is 2 MB and a PNG of a photographic
  // screenshot blows past it at 1280x720.
  await page.screenshot({ path: dest, type: "jpeg", quality: 92 });
  console.log(`  ${path.basename(dest)}  (frame at ${v.at}s)`);
}

await browser.close();
await rm(TMP, { recursive: true, force: true });

// Also keep one clean, untouched frame — some people prefer a bare screenshot,
// and it is the fastest thing to hand to a designer.
const plain = path.join(OUT_DIR, "frame-kasir.png");
await run("ffmpeg", ["-y", "-loglevel", "error", "-ss", String(chapterAt("Kasir") + 29), "-i", VIDEO, "-frames:v", "1", plain]);
console.log(`  ${path.basename(plain)}  (clean frame, no text)`);
