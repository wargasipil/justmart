# FAQ — questions answered with a real screen recording

One folder per question, named after the question as a slug:

```
faq/videos/<question-slug>/
  question.md      the question + the written answer (Indonesian — shop staff read this)
  youtube.md       optional: title / description / tags, if the video is published
  tutorial.webm    GITIGNORED — the screen recording, from `make faq-video`
  thumbnail.jpg    GITIGNORED — 1280x720 upload thumbnail, from faq/tools/make_thumbnail.py

faq/assets/music/   the looping bed every video is scored with (generated — see its README)
faq/tools/          make_music.py (the bed) + make_thumbnail.py (upload thumbnails)
```

**The renders are not committed.** `tutorial.webm` and `thumbnail.jpg` are
gitignored: each is a multi-megabyte binary that changes wholesale on every
re-record, which is the worst possible shape for git history, and both are
reproducible from what *is* committed — the recorder spec in
[`frontend/tests/faq/`](../frontend/tests/faq/) and the `Variant` in
[`make_thumbnail.py`](tools/make_thumbnail.py). Regenerate either with:

```sh
make faq-video q=<slug>                        # -> tutorial.webm
python faq/tools/make_thumbnail.py <slug>      # -> thumbnail.jpg
```

So a fresh clone has the answers and the means to rebuild every video, but none
of the renders. The published copies live on YouTube; the local ones are build
output. The music bed is the one generated asset that stays tracked — it is
small, shared by every video, and changes only when someone edits the
generator.

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
`cloudflare_tunnel_token`. The backend is started against `.faq-config.yaml` — a
generated copy of your `config.yaml` with that one key blanked out — and the env
var is pinned empty so a token exported in your shell cannot leak in either.

This used to be a single `JUSTMART_CLOUDFLARE_TUNNEL_TOKEN=off`, which is why
that sentinel exists at all (an empty env value means "not set", so a token in
`config.yaml` could not otherwise be overridden). The stripped copy replaced it
for two reasons: the process cannot read a token it was never handed, which is
the stronger guarantee, and `off` puts the Settings ▸ Akses jarak jauh panel
into a state — "disabled by the environment" — that no shop owner will ever see,
which is unrecordable for the question about that very panel. The sentinel is
still supported and still the right tool outside recording.

**ffmpeg is needed for the music**, not for the capture. Without it on PATH a
recording still succeeds and is simply written silent, with a warning — a
missing tool must not throw away a take that took minutes. See
[assets/music/README.md](assets/music/README.md) for the bed itself.

> `make faq-video` handles this for you, but a plain `make run` does not: if
> `config.yaml` sets `cloudflare_tunnel_token` — or a token is saved in
> Settings ▸ Akses jarak jauh — it opens a public tunnel to your dev machine for
> as long as it serves.

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

| Question | Rekam ulang |
|---|---|
| [Penerimaan restock yang sudah diterima, bisa dibatalkan?](videos/can-an-accepted-restock-be-cancelled/question.md) | `make faq-video q=can-an-accepted-restock-be-cancelled` |
| [Bagaimana kasir membuat transaksi (order)?](videos/how-does-a-cashier-create-an-order/question.md) | `make faq-video q=how-does-a-cashier-create-an-order` |
| [Bagaimana cara membuat restock (pesanan ke pemasok)?](videos/how-do-i-create-a-restock-order/question.md) | `make faq-video q=how-do-i-create-a-restock-order` |
| [Bagaimana cara memperbarui aplikasi Justmart?](videos/how-do-i-update-the-app/question.md) | `make faq-video q=how-do-i-update-the-app` |
| [Bagaimana memasukkan stok yang sudah ada di toko?](videos/how-do-i-enter-existing-stock/question.md) | `make faq-video q=how-do-i-enter-existing-stock` |
| [Bagaimana cara mengganti judul / nama toko yang tampil?](videos/how-do-i-change-the-shop-name/question.md) | `make faq-video q=how-do-i-change-the-shop-name` |
| [Bagaimana cara mencetak label barcode produk?](videos/how-do-i-print-a-product-barcode-label/question.md) | `make faq-video q=how-do-i-print-a-product-barcode-label` |
| [Pesanan yang sudah selesai, bisa dibatalkan?](videos/can-an-order-be-cancelled/question.md) | `make faq-video q=can-an-order-be-cancelled` |
| [Bagaimana cara mengisi token tunnel Cloudflare?](videos/how-do-i-add-a-cloudflare-tunnel-token/question.md) | `make faq-video q=how-do-i-add-a-cloudflare-tunnel-token` |
