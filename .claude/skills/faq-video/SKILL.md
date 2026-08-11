---
name: faq-video
description: Turn a support question into a complete FAQ entry under faq/videos/ — a written Indonesian answer, a screen recording of the real app driven by Playwright, a thumbnail, and YouTube copy. Trigger with /faq-video followed by the question (e.g. /faq-video bisakah penerimaan restock dibatalkan?).
---

# FAQ video — question in, published entry out

Produces one folder under `faq/videos/<slug>/`:

| File | What it is |
|---|---|
| `question.md` | the written answer, Indonesian — the source of truth |
| `tutorial.webm` | a capture of the **real app** doing it, with captions and a music bed |
| `thumbnail.jpg` | 1280×720 upload thumbnail |
| `youtube.md` | title options, description, tags |

Read [faq/README.md](../../../faq/README.md) first — it holds the convention.
This file is the procedure and the accumulated gotchas.

**What is genuinely automatic:** recording (servers start and stop themselves),
scoring, thumbnail rendering, file placement. **What still needs you:** reading
the feature to learn its real rules, writing the narration, and choosing the
thumbnail crop. Do not promise the user a one-button pipeline — promise one
command per step and no rediscovery of the traps below.

---

## Phase 1 — Understand the feature (do not skip)

The answer must come from the code, not from the UI's happy path. For the
feature the question is about, find and read:

1. **The handler** — `backend/internal/service/<domain>/<rpc>.go`. Extract:
   - every **precondition** and the **stable error token** it returns;
   - what the operation does to *other* records (status recompute, denormalized
     fields rebuilt, documents kept vs deleted);
   - who is allowed to call it (proto `allowed_roles`).
2. **The frontend surface** — where the action lives, and whether the UI
   **precomputes** the blocked reason (many do, so the button is disabled with a
   reason instead of failing on click).
3. **The tokens' Indonesian text** — `frontend/src/locales/id.json` under
   `serverErrors.*`. The answer must use the **same wording the user will
   actually see**, not a paraphrase.
4. **Existing tests** — `<rpc>_test.go` and any `frontend/tests/e2e/*.spec.ts`
   for the feature. The e2e spec usually already contains a working **seed
   recipe** you can lift wholesale, which is most of Phase 3.

**Show the boundary, not just the happy path.** These questions are almost
always really about *when the thing is not allowed*. An entry that only shows
success teaches the wrong rule and generates the bug report you were trying to
prevent.

## Phase 2 — `question.md`

Indonesian, aimed at a shop owner, not a developer. Structure that works:

- H1 = the question as asked.
- **Bold one-line answer immediately underneath**, then the video link.
- "Ringkasnya" — what the feature *means*, and what it is **not** (the sibling
  feature it gets confused with).
- "Caranya" — numbered steps using the exact on-screen labels.
- "Setelah ..." — what changed, including the non-obvious side effects.
- "Kalau tombolnya tidak muncul" — a **table of every blocked reason**, taken
  from the tokens in Phase 1.
- Close with one sentence on *why* the rule is strict. Shop owners accept a
  restriction they understand.

## Phase 3 — The recorder

`frontend/tests/faq/<slug>.spec.ts`. Copy the reference recorder
([can-an-accepted-restock-be-cancelled.spec.ts](../../../frontend/tests/faq/can-an-accepted-restock-be-cancelled.spec.ts))
— it is the template, not just an example.

Shape:

```ts
const backstage = await openBackstage(browser);   // signed in, NOT recorded
const seeds = await seedScenario(backstage.page); // over the Connect API
const stage = await openStage(browser, backstage.storageState);
try {
  /* narrate */
  await stage.save(videoDest(testInfo, SLUG));    // scores + writes the .webm
} finally {
  await stage.context.close().catch(() => undefined);
  await cleanup(backstage.page, seeds);
  await backstage.context.close();
}
```

Rules that are load-bearing:

- **Seed off camera, always.** Use `rpc(backstage.page, "<pkg>.v1.Service/Method", {...})`.
  Never record setup, and never record a login — the stage starts from a storage
  state captured backstage, so no credentials are ever on screen.
- **Seed the blocked case too.** Usually a second identical record plus one
  cheap mutation (a sale, a transfer, a one-unit return) that trips the guard.
- **Give seeded rows realistic names.** "Paracetamol 500 mg", "PT Sumber Sehat
  Farma" — not `Test Med 1723`. Only the SKU/code needs a uniqueness marker.
- **Narrate with the `_recorder` helpers**: `card` (title/outro), `say`
  (caption), `spotlight` (ring what you are talking about), `click`/`type`
  (moves the synthetic cursor). Pace with `BEAT`, not raw numbers.
- **Match Indonesian labels.** The stage forces `justmart_lang=id`, so the UI is
  Indonesian — the e2e suite's English selectors ("Cancel receipt") will not
  match. Read `id.json` for the real strings.
- **Scope text lookups to `#root`.** The caption bar is real DOM outside the
  app, and `say()` renders its `<b>…</b>` emphasis literally — so narrating
  "…di **Catatan rilis**" makes a bare `getByText("Catatan rilis")` ambiguous
  with the very element you are about to spotlight. Take
  `const app = page.locator("#root")` and use it for every `getByText`.
- **Not every scenario can be seeded over the API.** When the thing on screen
  depends on how the SERVER was started (the Updates panel compares the build
  stamp against the live GitHub release), configure it in
  `playwright.faq.config.ts` instead — and open the recorder with a backstage
  check that throws when the state isn't there. `reuseExistingServer: true`
  means a stray `make dev` silently supplies the wrong server, and a recorder
  that narrates over the wrong state is exactly the lie the whole format exists
  to prevent.
- **`<ConfirmDialog>` is `role="alertdialog"`**, while `EntityDrawer`/
  `EntityDialog` are `role="dialog"` (see CLAUDE.md).
- **Clean up in `finally`**, and only void a record that is actually voidable —
  a 400 in teardown is noise.
- **A recorder that edits SHOP-WIDE settings must seed its own starting state
  and check the restore landed.** Row-level residue is harmless; a settings
  restore that silently fails becomes the *starting state of the next take* —
  the shop-name recorder opened its second run already renamed and narrated a
  change nobody could see. So force a known value backstage before rolling
  (`START_TITLE`), and in the finally block report the restore failure instead
  of `rpcQuiet`-ing it, then read the value back and warn if it did not stick.
- **Connect JSON returns enums by NAME, not number.** `GetSettings` answers
  `"businessType": "BUSSINESS_TYPE_PHARMACY_SHOP"`, so a `=== 1` check against a
  seeded response is silently always false. Requests accept either form.

Then record:

```sh
make faq-video q=<slug>
```

It starts the backend and Vite itself, waits for both, and tears them down after
— nothing to set up, nothing left running. Needs
`JUSTMART_TEST_OWNER_PASSWORD` matching `config.yaml` `bootstrap.owner_password`.

## Phase 4 — Watch the take (do not skip)

You cannot ship a recording you have not looked at. Cheapest useful check is a
contact sheet:

```sh
ffmpeg -i faq/videos/<slug>/tutorial.webm -vf "fps=1/8,scale=440:-1,tile=4x3" -frames:v 1 sheet.png
```

Read it and confirm: every caption matches what is on screen, no stale caption
lingers over a later step, no long stretch with no caption. If something looks
wrong, **zoom in before concluding** — sampling every 8 s makes a legitimately
slow step look like a stuck one. Verify at 1 fps over the suspect range before
changing anything.

## Phase 5 — Thumbnail

Add a `Variant` under **your slug's** key in
[faq/tools/make_thumbnail.py](../../../faq/tools/make_thumbnail.py)
(`VARIANTS[<slug>][<letter>]` — a frame time and a crop box only mean anything
against one video), giving frame time, crop box in source-frame coordinates,
copy, and an optional ring. Then:

```sh
python faq/tools/make_thumbnail.py <slug>
```

- **Two short words in the hero line.** Font size is derived from the panel
  width, so a long phrase silently shrinks itself into illegibility. "BATAL
  TERIMA" renders at ~104 px; "MASIH BISA DIBATALKAN" collapses to ~72 px.
- **Crop to one element** — a dialog, a row. A whole app screenshot scaled into
  a thumbnail is a grey smear.
- **Do not repeat the title.** The title asks, the thumbnail answers.
- **Check it at feed size** (~168 px wide) before accepting it. Downscale and
  look; several designs that read fine at full size do not survive it.

## Phase 6 — `youtube.md` and the index

- Title: problem-first, ≤ 60 chars, no brand name (the channel name is already
  under it), keywords front-loaded.
- Description: the answer in the **first two lines** (all that shows before
  "Selengkapnya"), then steps, side effects, the confused-sibling distinction,
  the blocked reasons. **These videos have no narration**, so YouTube has no
  transcript to index — the description is the only machine-readable text, and
  every term a user might type has to appear in it.
- **No chapters/timestamps.** Recording length varies run to run (68 s vs 87 s
  for identical content), so hardcoded times go stale on the next re-record.
- **No `<` or `>` anywhere in the title, description, or tags.** YouTube treats
  angle brackets as markup and strips or rejects the text around them — a menu
  path written "Pengaturan > Umum" can arrive as "Pengaturan" with the rest of
  the line gone, and the loss is silent (the upload succeeds). This bites here
  more than anywhere else because these entries are *menu-path instructions*, so
  the natural way to write every numbered step is exactly the banned character.
  Write menu paths plainly instead — "buka Pengaturan, Umum" (or `▸` if you want
  a separator) — and use words for comparisons ("maksimal 60 karakter", not
  "< 60"). The rule covers only what gets pasted into YouTube; `question.md` and
  the rest of the repo are unaffected.
- Add the question to the table at the bottom of `faq/README.md`.

---

## Traps already paid for

Every one of these cost a cycle. They are handled in `_recorder.ts` — do not
re-implement them, and do not undo them.

| Trap | Why it bites |
|---|---|
| UI records in English | Chromium's locale wins unless `justmart_lang=id` is set by an init script before the app boots. |
| TanStack devtools in frame | Dev-only chrome, bottom-right of every capture. Hidden by CSS in the studio chrome. |
| White flash at the start | Show the title `card()` **before** waiting for page data, so the app loads behind it. On a page whose first paint waits on a network round trip, `goto`'s own default wait is the flash — use `waitUntil: "commit"`, then `page.locator("#__faq_card").waitFor({ state: "attached" })` before the card. |
| Video lands in the test tree | `config.rootDir` is the common ancestor of the test dirs, not the config's folder. Use `videoDest(testInfo, slug)`. |
| Page freezes after a dialog | Ark's body lock is only released on a proper close transition — never unmount an open dialog. |
| Music pumping | Envelopes contained in one bar loop cleanly but drop to silence on every chord change. The pad chords overlap; the mix wraps circularly. |
| A public tunnel opens | `config.yaml` may carry `cloudflare_tunnel_token`. The faq config passes `JUSTMART_CLOUDFLARE_TUNNEL_TOKEN=off`, which is why that sentinel exists. |
| Dev-DB residue | A fully-received PO is not voidable, so the blocked-case seed survives teardown. Marker-unique, accumulates slowly, documented. |

## Definition of done

- [ ] `question.md` covers the happy path **and** every blocked reason.
- [ ] The recording shows both, and you have watched it.
- [ ] `thumbnail.jpg` is legible at 168 px.
- [ ] `youtube.md` written, free of `<` / `>` in the title, description and
      tags; `faq/README.md` table updated.
- [ ] No leftover dev servers (the config handles this — verify if you started
      any by hand).
- [ ] `npx tsc --noEmit` passes in `frontend/`.
