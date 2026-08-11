# FAQ — questions answered with a real screen recording

One folder per question, named after the question as a slug:

```
faq/videos/<question-slug>/
  question.md      the question + the written answer (Indonesian — shop staff read this)
  tutorial.webm    a screen recording of the real app doing it, with a music bed
  youtube.md       optional: title / description / tags, if the video is published
  thumbnail.jpg    optional: 1280x720 upload thumbnail, from faq/tools/make_thumbnail.py

faq/assets/music/   the looping bed every video is scored with (generated — see its README)
faq/tools/          make_music.py (the bed) + make_thumbnail.py (upload thumbnails)
```

## Publishing

`youtube.md` and `thumbnail.jpg` only exist for questions actually uploaded
somewhere. Both are generated or written per question — the thumbnail comes from
a frame of that question's own video:

```sh
python faq/tools/make_thumbnail.py <slug>              # -> faq/videos/<slug>/thumbnail.jpg
python faq/tools/make_thumbnail.py <slug> --variant c  # alternate design
```

Two constraints worth knowing before designing one. A thumbnail is read at about
**168 px wide**, so the hero line is two short words and the screenshot is
cropped to a single element — a whole app screenshot scaled down is a grey
smear. And the thumbnail text must not repeat the title: the title asks, the
thumbnail answers.

## Why the video is a recording, not a mockup

`tutorial.webm` is captured by driving the actual running app with Playwright —
same UI, same backend, same rules. Nothing in it is drawn or re-created, so a
video cannot quietly disagree with the product. When the UI changes, you
re-record instead of redrawing, and if the flow no longer works the recorder
fails loudly rather than shipping a lie.

## Where the recorders live

In [`frontend/tests/faq/`](../frontend/tests/faq/), one `<question-slug>.spec.ts`
per question, sharing [`_recorder.ts`](../frontend/tests/faq/_recorder.ts).

They sit under `frontend/` only because that is where `node_modules` resolves
from — a spec in `faq/` could not `import { test } from "@playwright/test"`.
The artifact they produce is what lives here.

**They are not tests.** They assert nothing, they run for minutes, and
`playwright.faq.config.ts` keeps them out of `make test-browser` / `make test-all`.

## Re-recording

One command, from nothing running:

```sh
export JUSTMART_TEST_OWNER_PASSWORD=<config.yaml bootstrap.owner_password>
make faq-video                        # every question
make faq-video q=can-an-accepted      # just the ones matching a slug substring
```

It starts the Go backend and the Vite dev server itself, waits for both, records,
and tears them down (`webServer` in
[playwright.faq.config.ts](../frontend/playwright.faq.config.ts)). An already
running `make dev` is reused rather than fought over the port. It writes to the
dev DB; each recorder seeds its own scenario through the Connect API and cleans
up after itself. The recorder overwrites `faq/videos/<slug>/tutorial.webm` in
place.

**Recording never opens a Cloudflare tunnel**, even when `config.yaml` carries a
`cloudflare_tunnel_token` — the config passes
`JUSTMART_CLOUDFLARE_TUNNEL_TOKEN=off`. That sentinel exists for this: an empty
env value means "not set", so before it there was no way to switch a configured
tunnel off, and generating docs would have published the dev database on a
public hostname.

**ffmpeg is needed for the music**, not for the capture. Without it on PATH a
recording still succeeds and is simply written silent, with a warning — a
missing tool must not throw away a take that took minutes. See
[assets/music/README.md](assets/music/README.md) for the bed itself.

> If `config.yaml` sets `cloudflare_tunnel_token`, `make run` also opens a public
> tunnel to your dev machine for as long as it serves. Point `JUSTMART_CONFIG` at
> a copy without that key while recording.

## Adding a question

**Use the `/faq-video` skill** ([.claude/skills/faq-video/SKILL.md](../.claude/skills/faq-video/SKILL.md)) —
it drives the whole sequence and carries the traps that have already cost a
cycle each. The steps it follows, for when you are doing it by hand:

1. `faq/videos/<slug>/question.md` — the question and the written answer. The
   text has to stand on its own; the video is the demonstration, not the source
   of truth.
2. `frontend/tests/faq/<slug>.spec.ts` — copy an existing recorder. Seed the
   scenario over the Connect API (never on camera), then narrate with the
   `_recorder` helpers: `card` for a title, `say` for a caption, `spotlight` to
   ring what you are talking about, `click`/`type` to move the synthetic cursor.
   Save to `faq/videos/<slug>/tutorial.webm` — `save()` scores it for you.
3. Record it, watch it once end to end, and clean up the seeded rows.

**Show the boundary, not just the happy path.** Most of these questions are
really about when something is *not* allowed — that half is what stops someone
filing a bug when the button they saw in the video isn't there.

## Questions

| Question | Video |
|---|---|
| [Penerimaan restock yang sudah diterima, bisa dibatalkan?](videos/can-an-accepted-restock-be-cancelled/question.md) | [tutorial.webm](videos/can-an-accepted-restock-be-cancelled/tutorial.webm) |
| [Bagaimana kasir membuat transaksi (order)?](videos/how-does-a-cashier-create-an-order/question.md) | [tutorial.webm](videos/how-does-a-cashier-create-an-order/tutorial.webm) |
| [Bagaimana cara membuat restock (pesanan ke pemasok)?](videos/how-do-i-create-a-restock-order/question.md) | [tutorial.webm](videos/how-do-i-create-a-restock-order/tutorial.webm) |
| [Bagaimana cara memperbarui aplikasi Justmart?](videos/how-do-i-update-the-app/question.md) | [tutorial.webm](videos/how-do-i-update-the-app/tutorial.webm) |
| [Bagaimana memasukkan stok yang sudah ada di toko?](videos/how-do-i-enter-existing-stock/question.md) | [tutorial.webm](videos/how-do-i-enter-existing-stock/tutorial.webm) |
| [Bagaimana cara mengganti judul / nama toko yang tampil?](videos/how-do-i-change-the-shop-name/question.md) | [tutorial.webm](videos/how-do-i-change-the-shop-name/tutorial.webm) |
| [Bagaimana cara mencetak label barcode produk?](videos/how-do-i-print-a-product-barcode-label/question.md) | [tutorial.webm](videos/how-do-i-print-a-product-barcode-label/tutorial.webm) |
