# graphify — project notes

How this project uses the code knowledge graph in `graphify-out/`.
Product docs: <https://graphify.net>.

> **Why this file exists:** the `## graphify` block in [../CLAUDE.md](../CLAUDE.md) is
> **generated** — `graphify claude install` rewrites everything from that heading to the
> next `##` heading. Project-specific guidance must live here, or a reinstall silently
> overwrites it.

## What's wired up

| Piece | Where | Notes |
| --- | --- | --- |
| Graph output | `graphify-out/` (`graph.json`, `GRAPH_REPORT.md`, `GRAPH_TREE.html`) | **gitignored** — per-machine, rebuilt, never committed |
| Agent instructions | the generated `## graphify` block in `CLAUDE.md` | tells the agent to query the graph before grepping |
| Agent enforcement | `PreToolUse` hooks in `.claude/settings.json` | `graphify hook-guard search` (Bash) + `hook-guard read` (Read/Glob) |
| Auto-refresh | git `post-commit` / `post-checkout` hooks in `.git/hooks` | AST-only refresh per commit; local, `graphify hook uninstall` to remove |
| Rebuild commands | `make graph` / `make graph-full` / `make graph-label` | see below |

The hook commands in `.claude/settings.json` are written as an **absolute path** to the
graphify binary (`C:\Users\<you>\.local\bin\graphify.EXE`). That file is committed, so on a
second dev machine with a different user name the hooks silently fail — re-run
`graphify claude install` there to rewrite the path.

## Rebuilding the graph

| Command | What it does | Cost |
| --- | --- | --- |
| `make graph` | `graphify update .` — AST-only refresh | free, no LLM |
| `make graph-full` | `graphify extract . --backend claude-cli` — AST **+ semantic** | no API key; bills to the Claude Code plan |
| `make graph-label` | re-name communities + regenerate `GRAPH_REPORT.md` | one LLM pass, no re-extraction |
| `make graph-viz` | emit `graphify-out/GRAPH_TREE.html` (browsable code tree) | free, no LLM |

**No `graph.html` here.** graphify refuses its force-directed viz above 5,000 nodes and this
repo extracts ~11.9k, so `make graph-viz` (D3 collapsible tree) is the browsable view. Raise
`GRAPHIFY_VIZ_NODE_LIMIT` if you want the force-directed one anyway — expect it to be heavy.

Cheaper full pass: `make graph-full GRAPHIFY_MODEL=haiku` (`haiku` | `sonnet` | `opus`;
default `sonnet`).

### Why `--backend claude-cli`

It drives the locally-installed `claude` CLI and authenticates via the existing Claude Code
subscription — **no `ANTHROPIC_API_KEY`, no signup**. It is not listed in
`graphify extract --help`, but it is a fully supported backend.

## When `make graph` is NOT enough

AST-only extraction follows imports and calls. That leaves this stack's blind spots, which
only `make graph-full` fills:

- **The frontend↔backend boundary.** A ConnectRPC call is an HTTP request, not an import —
  and the two generated client/server halves (`frontend/src/gen/` vs `backend/gen/`) share a
  `.proto` origin but no import edge. So an AST-only graph **cannot** connect
  `queries/sales.ts` to `service/sale/complete_sale.go`. Never conclude "the frontend
  doesn't call X" from an AST-only graph.
- **`.proto` → generated code → handler** chains, for the same reason.
- **Docs.** `--code-only` skipped 27 doc files, so `CLAUDE.md`, `DEPLOYMENT.md`,
  `test-specification.md`, and `bussiness-planning.md` are absent until a full pass. Much of
  this project's *why* (hard rules, conventions, roadmap) lives only there.
- **goose migrations ↔ GORM models.** The `.sql` files and `internal/model/` structs are
  linked by table name only.

Rule of thumb: `make graph` after ordinary code edits; `make graph-full` when the graph must
answer *why* / *cross-stack* / *doc-aware* questions, or after changes that cross the RPC
boundary.

## Querying it

```sh
graphify query "how does FEFO consumption pick batches on CompleteSale"
graphify explain "resolveWarehouse"
graphify path "Pos.tsx" "complete_sale.go"
graphify affected "recomputeSaleTotals"   # reverse deps — what breaks if I change this
```

Prefer these over `grep` for orientation; grep once you know which lines to edit.

## Community labels

Community names are LLM-generated and cached in `graphify-out/.graphify_labels.json`.
Re-clustering shifts community IDs, so a stale cache can pin old names onto new clusters
(symptom: nonsense like a migration landing in a "POS Cart" community). Fix with a fresh
naming pass:

```sh
rm graphify-out/.graphify_labels.json
make graph-label
```

## Not wired up (options if you want them)

- **`graphify extract . --postgres <DSN>`** — maps live Postgres tables/views/functions + FK
  relationships into the graph. Genuinely useful here (the schema is the app's backbone), but
  it needs `make up` running and is **untested in this repo** — try it before trusting it.
- **`graphify watch .`** — rebuild on every file save instead of per commit.
- **`graphify global add`** — merge this graph with the sibling `getresolved` root graph for
  cross-repo queries. Note `apps/justmart` is its own git repo with its own `graphify-out/`,
  separate from the root repo's.
