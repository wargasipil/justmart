# Release video — the product tour published to YouTube

```
release/
  video/justmart-product-tour-1080p.mp4   the upload
  video/subtitles.id.srt                  Indonesian subtitle track
  video/thumbnail-kasir.jpg               thumbnail A (POS)
  video/thumbnail-laporan.jpg             thumbnail B (report charts)
  video/frame-kasir.png                   clean frame, no text, for a designer
  video/product-tour.webm                 raw capture (no audio)
  video/chapters.json                     chapter marks, stamped while recording
  youtube.md                              the upload guide + checklist
  youtube/{title,description,tags}.txt    paste-ready, no markdown fences
```

Everything under `video/` is gitignored — it is 20-30 MB of build output. The
recorder, the seed and the generators are what's versioned.

The tour runs about 3½ minutes, in Indonesian, and covers: dashboard → kasir
(including grosir pricing) → riwayat order → katalog & stok → receiving a
delivery → analitik → multi-gudang → mode apotek.

## Why it records against a separate instance

The FAQ tutorials record against the dev stack. This one must not.

A release video is published, and the dev database is a working surface — it
accumulates rows like `Restock Med 1785524168486` and `FAQ-PCT-282867`, products
sitting at zero stock, and whatever half-finished experiment was running that
week. So the tour drives a **throwaway demo instance**: the single binary,
serving its own SQLite file on port 8099, seeded by
[`seed-demo.mjs`](seed-demo.mjs) with a small Indonesian minimarket.

The instance is also a *packaged* build rather than the Vite dev server, so the
recording shows what a shop actually installs — no dev-only chrome.

## Recording it

Needs `node`, `go`, `ffmpeg` on PATH, and the frontend deps installed
(`make web-install`).

```sh
# terminal 1 — build the binary and serve the demo instance (leave running)
make release-demo

# terminal 2 — seed the shop ONCE, into a fresh demo.db
make release-seed

# record (~3.5 min), then encode + regenerate the description
make release-video
make release-encode
```

`make release-demo` writes into `.release-demo/` (gitignored). To start over,
stop the server, delete `.release-demo/demo.db*`, and seed again — the seed is
not idempotent by design: fixed SKUs and supplier codes mean a second run fails
loudly instead of quietly doubling the catalog.

## What the seed builds, and why each part is there

- **12 products** with a base unit and a larger `dus`, so buying-in-cases and
  selling-in-pieces both have something to show.
- **A grosir ladder** on four of them — the wholesale pricing the POS chapter
  demonstrates live.
- **Four restock orders**, three received and one still in transit. The one in
  transit is what gets received on camera; without it there is nothing live to
  demonstrate, because the app will not let you receive an order twice.
- **~130 sales spread over 30 days**, including today. Sales are created through
  the API at `now` and then moved into the past by a direct SQLite pass, because
  `CompleteSale` stamps the current time — an API-only shop has its entire
  history at one timestamp and every chart in the video is a single spike.
  Stock arrivals are moved *before* the trading window for the same reason: leave
  them at `now` and a month of sales draws on stock that had not arrived yet, so
  every historical stock level reads negative.
- **Three products left under the low-stock threshold** and none at zero. An
  empty shelf is a real state the app handles, but a promo video should not open
  on one, and the low-stock bell has nothing to say if everything is comfortable.

## The recorder

[`frontend/tests/release/product-tour.spec.ts`](../frontend/tests/release/product-tour.spec.ts),
sharing narration helpers (`say` / `card` / `spotlight` / `click` / `type`) with
the FAQ recorders and adding its own stage in
[`_studio.ts`](../frontend/tests/release/_studio.ts) — 1080p instead of 720p,
with the caption chrome scaled to match, because this plays full-screen on
YouTube rather than inline in an FAQ.

It asserts nothing and is excluded from `make test-browser` by living under its
own [`playwright.release.config.ts`](../frontend/playwright.release.config.ts).

Two things it does off camera, both so a re-record just works:

- **Resets the shop to retail mode.** The tour ends by switching to pharmacy
  mode, so a second take would otherwise open on "Obat", a Resep menu, and a
  settings panel already reading "Apotek" — every selector in the retail half
  would miss.
- **Raises an incoming delivery if none exists**, since the take consumes the
  one it receives.

It signs in over the API and injects the tokens rather than driving the login
form. The login rate limiter allows 5 attempts per email, refilling one a
minute; iterating on a recording burns that within a few takes, after which
every run dies at a bare navigation timeout that looks nothing like its cause.

## Encoding

[`encode.mjs`](encode.mjs) transcodes the VP8 capture to H.264/AAC MP4 and mixes
in the same generated music bed the FAQ tutorials use
(`faq/assets/music/corporate-bed.opus` — synthesized, not licensed, so a public
upload carries no third-party rights). The bed is normalized to −16 LUFS: YouTube
attenuates anything above about −14 and never boosts anything below, so louder
would simply be turned back down.

There is **no voice-over**. The narration is burned-in Indonesian captions, and
the pacing was written to be narratable if you later want to record one over the
same file.

## Subtitles

`subtitles.id.srt` is written by the recorder, not transcribed afterwards: the
`say`/`card` wrappers in [`_studio.ts`](../frontend/tests/release/_studio.ts)
stamp a cue as they draw each caption. That is the only way the track is
guaranteed to match the burned-in text — a hand-written SRT drifts on the first
re-record.

The captions are already in the picture, so the track is not what the viewer
reads. It is what YouTube indexes for search and what auto-translate works from,
which is worth having for a video whose whole audience searches in Indonesian.

## Thumbnails

[`thumbnail.mjs`](thumbnail.mjs) pulls frames out of the finished MP4 and
composites the text as HTML, screenshotted with the same Playwright that
recorded the tour — `drawtext` gives no real typography, and no readable escape
story for Windows font paths.

Frame times are **chapter-relative**, so a re-record that shifts the timeline
still lands on the right screen. Both variants pan LEFT, because the copy sits
on the right and a full 1920px UI shrunk to a 360px feed thumbnail is grey mush.
Frame choice, crop and copy are the constants at the top of the script.

## Uploading

[`youtube.md`](youtube.md) is the checklist; `youtube/*.txt` are the paste-ready
strings. Both are regenerated by `make release-encode`.

Chapter timestamps are never hand-written, and `description.mjs` **fails** if any
two marks land under 10 s apart — YouTube silently ignores the entire chapter
list in that case, so it is better to stop here than to find out after
publishing. If it fires, lengthen that section in the spec and re-record.

Before publishing, settle the claim no recording can verify ("jalan di satu
komputer, tanpa langganan bulanan") and fill in the two LINK placeholders.

## Re-recording after a UI change

Re-run `make release-video`. If a screen has moved, the recorder fails loudly on
that step rather than shipping a video that disagrees with the product, and
drops a screenshot at `release/video/_failure.png` showing the page at the
moment it gave up.
