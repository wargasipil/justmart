---
name: refactor-audit
description: Assess frontend pages/components for excessive complexity, check them against the shared-component catalog in both directions (reinvents something that exists / should be promoted to shared), and split them into focused files following this repo's conventions. Audit-only by default — reports a ranked list with a proposed split before touching anything. Trigger with /refactor-audit, optionally scoped to a path (e.g. /refactor-audit routes/Pos.tsx).
---

# Refactor audit — frontend pages & components

Find files in `frontend/src/` that have grown too complex, and split them into focused
files **using the layout this repo already uses**, not a generic "extract component" pass.

## Two modes

**1. Sweep (default) — AUDIT ONLY.** Invoked bare (`/refactor-audit`), it scans the whole
frontend, scores, and reports. Never rewrite a 1000-line route because a sweep flagged it.
The report is the deliverable unless the user then says "refactor X" or "do it".

**2. Post-implementation gate — REFACTOR IN PLACE.** Invoked with explicit paths
(`/refactor-audit routes/inventory/ProductDetail.tsx`), typically as the last step of a
frontend task per the HARD RULE in CLAUDE.md. Here the audit-first default does **not**
apply: skip Phase 1, score the named files, **run Phase 2.5 on them**, and **if they're
flagged, do the split (and any reuse swap) in the same change**. Stopping to ask "want me
to split it?" is the failure mode this mode exists to prevent. Only stop and ask if the
split would be genuinely ambiguous (e.g. a piece is borderline "shared vs page-local") —
and then ask that specific question, not "shall I proceed?".

Either way Phases 2.5 and 4 are identical, and the change stays behaviour-preserving
apart from the single documented reuse-swap carve-out (Phase 4 rule 4).

---

## Phase 1 — Measure

```sh
cd frontend/src && find . -name "*.tsx" -not -path "./gen/*" -exec wc -l {} + | sort -rn | head -30
```

For each candidate above the line threshold, collect the structural signals:

```sh
# top-level components living in one file
grep -nE "^(export (default )?)?function [A-Z]|^const [A-Z][A-Za-z]* = \(" <file>
# state / effect / query surface
grep -cE "useState|useEffect|useMemo|useCallback" <file>
grep -nE "use[A-Za-z]*(Query|Mutation)\(" <file>
```

Do NOT read every large file end-to-end in the main context. For a repo-wide sweep,
delegate the per-file structural read to parallel `Explore` subagents (one per candidate,
or batched 3–4 per agent) and keep only their scored summaries.

---

## Phase 2 — Score

Line count alone is not complexity. A 400-line table page with one query is fine; a
250-line file holding a page + a form + a dialog is not. **Multiple co-located
responsibilities is the real signal.**

Flag a file when it hits **≥2 of these**:

| Signal | Threshold |
|---|---|
| Length | > 400 lines (> 250 for anything in `components/`) |
| Co-located components | ≥ 3 top-level `function Foo()` declarations in one file |
| Inline sub-component size | any nested/co-located component > 80 lines |
| Server state | ≥ 4 distinct `use*Query` / `use*Mutation` hooks |
| Local state | ≥ 8 `useState` calls, or ≥ 3 `useEffect` blocks |
| Mixed concerns | page shell **and** a form **and** a dialog/drawer in one file |
| Cross-cutting side effects | direct `localStorage` / `document` / `window` access woven into JSX |
| Deep nesting | JSX nested > 6 levels, or a ternary chain > 3 deep in the return |
| **Reinvented shared component** | any hit from the Phase 2.5 replace table — the file hand-rolls something that already exists |
| **Duplicated block** | a ≥ 30-line block that also appears (near-identically) in ≥ 2 other routes — a promotion candidate |

**Not a defect on its own** — do not flag for these alone:
- A long but flat table page (one query, one `<Pagination>`, one map over rows).
- A registry / locale / config file that is a long literal (`routes/dev/registry.ts`, `demos.tsx`).
- Generated code under `frontend/src/gen/` — **never touch it**.

Rank by *how tangled*, not by line count. A 300-line file mixing four concerns outranks
a 700-line flat list.

---

## Phase 2.5 — Reuse check (both directions)

Splitting a file is only half the job. Run this on **every** candidate, in both modes —
including a single named file in gate mode, where it is often the whole finding. It is
the one part of the audit that *deletes* code instead of moving it.

### (a) Does it reinvent something that already exists?

The catalog is `frontend/src/routes/dev/registry.ts` (the `/components` gallery) plus the
HARD RULEs in CLAUDE.md. Read the registry's `name` / `file` / `summary` fields — that is
the source of truth for "what already exists" — then grep the candidate for these
tells:

| Tell in the file | Replace with |
|---|---|
| `window.confirm` / `alert` / `prompt` (or bare) | `<ConfirmDialog>` · `toast` · a Chakra `Dialog` + `Field` |
| `NativeSelect`, or a raw `<select>` | `<SearchableSelect>` (dynamic, `loadOptions`) · `<EnumSelect>` (≤ 20 fixed) |
| `<Input type="number">` holding money | `<MoneyInput>`, or the `money` prop on `<FormField>` |
| `useState` + `<Text color="red.500">` validation | `<FormField>` + RHF + `zodResolver` |
| `NavLink` + `borderBottomWidth` tab strip | `<RouteTabs>` (routes) · `Tabs.Root` (state) |
| `ResponsiveContainer` / `<LineChart>` styled inline, any hex literal | `<ChartCard>` + `<TrendChart>` + `lib/chartTheme.ts` |
| A hand-rolled back link on a `:id` route | `<BackButton>` |
| A crumb trail rendered in the page | nothing — the TopBar `<Breadcrumbs>` owns it; add a `lib/breadcrumbs.ts` entry |
| A preset `<EnumSelect>` + two date pickers | `<DateRangeFilter>` + `lib/dateRange.ts` |
| `.slice()` over a full list to page it | `<Pagination>` + `usePageState` + a paginated `List*` RPC |
| `useAll*Query` used only to build a `nameById` map | the resolve hooks in `queries/refs.ts` |
| A raw `File` sent over the wire, or a hand-rolled canvas resize | `lib/imageRenditions.ts` |
| A create/edit form in a bespoke overlay | `<EntityDrawer>` (default) · `<EntityDialog>` (modal) |
| A hard-coded user-visible string, or a literal "Product"/"Produk" | `t("…")` · `$t(glossary.product)` |

A hit here is **in scope to apply**, unlike a bug (Phase 4 rule 4) — it is convention
compliance, and the replacement is the repo's documented behaviour. Two caveats: say so
explicitly in the report, since swapping a native `confirm()` for `<ConfirmDialog>` is a
visible change; and if the swap is large enough to be its own task, report it and stop
rather than burying it in a split diff.

### (b) Should a piece be promoted to shared?

Before proposing `components/<Name>.tsx` in Phase 4, prove it is actually shared:

```sh
cd frontend/src && grep -rn "<distinctive JSX or prop name>" routes/ components/ --include=*.tsx
```

- **≥ 2 other routes use a near-identical block** → promote. It ships with its
  `registry.ts` entry + `demos.tsx` demo in the same change (HARD RULE).
- **Only this route uses it** → it stays next to the route (Phase 4 rule 1). "It might be
  reused later" is not evidence.
- **It already exists under another name** → that is case (a), not a promotion.

In a repo-wide sweep, hand this grep to the same `Explore` subagent that reads the file,
so duplication is scored rather than eyeballed.

---

## Phase 3 — Report

Output a ranked table, then stop and ask which to take:

```
| Rank | File | Lines | Signals hit | Replace with existing | Proposed split |
|------|------|-------|-------------|----------------------|----------------|
| 1 | routes/Pos.tsx | 1709 | 7 co-located components, 6 dialogs, localStorage in JSX, 12 useState | — | → posDialogs.tsx (Customer/Rx/Receipt pickers), posCart.tsx, lib/posDraft.ts |
| 2 | routes/Foo.tsx | 380 | window.confirm ×2, raw number input for price | `<ConfirmDialog>`, `<MoneyInput>` | none — the reuse swap is the whole fix |
```

For each entry name the **destination files and what moves**, not just "split it up".
The "Replace with existing" column carries the Phase 2.5(a) hits; a row can be
replace-only, with no split at all — that is a *better* outcome than a split, since it
removes code. If a piece is genuinely reusable across pages, say so explicitly **with the
grep evidence from Phase 2.5(b)** — that changes the destination to `components/` and
pulls in the registry rule below.

---

## Phase 4 — Split playbook

Follow the destinations this codebase already established. Do not invent a new folder
vocabulary.

| What it is | Where it goes | Existing precedent |
|---|---|---|
| Create/edit form for an entity | `routes/<domain>/<entity>Drawers.tsx` | `routes/inventory/productDrawers.tsx` |
| Form body shared by a page + a drawer | `routes/<domain>/<Entity>FormFields.tsx` (controlled `value` / `onChange(patch)` + `formFrom*`/`formTo*` helpers) | `routes/prescriptions/PrescriptionFormFields.tsx` |
| One tab of a detail page | `routes/<domain>/<Entity><Tab>Tab.tsx` | `GrosirTab` inside `ProductDetail.tsx` (a current candidate) |
| A dialog only this page opens | `routes/<domain>/<page>Dialogs.tsx` | — |
| Pure derivation / formatting / storage keys | `lib/<topic>.ts` | `lib/pricing.ts`, `lib/printerTarget.ts`, `lib/dateRange.ts` |
| Data fetching | `queries/<domain>.ts` | every `queries/*.ts` |
| **Genuinely reusable across pages** (grep-proven) | `components/<Name>.tsx` **+ a `routes/dev/registry.ts` entry + a `demos.tsx` demo in the same change** | the ~40 files in `components/` |
| **Already exists in `components/`** | nowhere — delete it and import the existing one | Phase 2.5(a) |

Rules while moving code:

1. **Page-local JSX does NOT go in `components/`.** That folder is the shared style
   vocabulary surfaced by the `/components` gallery. If it is only used by one route,
   it belongs next to that route. Promotion needs the Phase 2.5(b) grep as evidence,
   not an intuition that it "feels reusable".
2. **A new file in `components/` ships with its `registry.ts` entry + a `demos.tsx`
   demo in the same change** (HARD RULE in CLAUDE.md). Demos are self-contained: local
   state only, no query hooks, no server calls.
3. **Never unmount an open `Dialog.Root`.** Splitting a dialog out must not introduce
   `{open && <Dialog…>}` or `if (!open) return null` — that leaves the Ark body lock
   in place and freezes the page. Keep it mounted, guard the *content* on the data.
   If the split moves a dialog near a `navigate()`, preserve the `releaseModalBodyLock`
   pattern from `routes/Pos.tsx`.
4. **Behaviour-preserving only.** A refactor pass does not fix bugs, rename props,
   change copy, or "improve" a query key. Note anything suspicious in the summary
   instead. If a genuine bug is found, report it — do not silently fix it in the same diff.
   **The one carve-out is a Phase 2.5(a) reuse swap**, which is convention compliance and
   may visibly change chrome (a native `confirm()` becomes a Chakra dialog). Call it out
   in the report; never let it ride along unmentioned.
5. **Preserve the conventions that are easy to lose in a move**: `t("…")` keys (no
   hard-coded user strings), `<FormField>` + RHF + Zod (no ad-hoc `useState` validation),
   `<MoneyInput>` / the `money` prop for money, `useServerFormErrors` wiring, role and
   `useBusinessMode()` gates, `<BackButton>` on detail pages.
6. **Extract state before JSX.** When a page has tangled state, prefer lifting a
   cohesive slice into a local `usePosCart()`-style hook in the same folder over
   threading 10 props into a new component.
7. **One file at a time.** Finish and verify a split before starting the next.

---

## Phase 5 — Verify

After each split:

```sh
cd frontend && npx tsc --noEmit
```

Then, if the touched surface is covered by a spec:

```sh
make test-browser   # needs `make run` + `make web` already up
```

If you start `make run` / `make web` yourself, **kill them before ending the turn**
(`netstat -ano | grep ":8080.*LISTENING"` → `taskkill //F //PID <pid>`) — that is a
HARD RULE in CLAUDE.md.

Report: files created, what moved where, line-count before/after, **any Phase 2.5(a)
reuse swap and what it visibly changed**, and confirmation that nothing else changed
behaviour. If a shared component was added, confirm both the `registry.ts` entry and the
`demos.tsx` demo landed; if one was *removed* in favour of an existing component, say
which existing one now covers it.

---

## Out of scope

- `frontend/src/gen/**` — generated, never edit.
- Backend Go services — they already follow one-file-per-RPC under
  `internal/service/<domain>/`. If a Go handler file looks overgrown, report it but
  refactor it under that existing convention, with its co-located `<rpc>_test.go`.
