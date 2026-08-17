import { GROUPS, type ComponentEntry } from "./registry";

// Bridge from the dev gallery's registry to Storybook's docs panel.
//
// The registry is already the curated source of truth for "what each shared
// component is, how you call it, and what bit us" — see registry.ts. Stories
// therefore READ that prose instead of restating it, so the two catalogs can
// never drift into disagreeing about the same component. A story still owns
// what the registry can't express: per-prop controls and the individual states
// worth pinning (loading, empty, disabled, error).

const BY_ID = new Map<string, ComponentEntry>(
  GROUPS.flatMap((g) => g.entries.map((e) => [e.id, e] as const)),
);

/** The registry entry for a gallery id (e.g. "money-input"). */
export function entryFor(id: string): ComponentEntry {
  const entry = BY_ID.get(id);
  if (!entry) {
    // Loud on purpose: a typo here would silently ship a story with no docs,
    // and a component REMOVED from the registry should break its story too.
    throw new Error(
      `storyDocs: no registry entry "${id}". Known ids: ${[...BY_ID.keys()].join(", ")}`,
    );
  }
  return entry;
}

/**
 * Storybook `parameters` carrying the registry's summary + usage + gotchas as
 * the component's docs description.
 *
 * ```ts
 * const meta = { component: MoneyInput, parameters: storyDocs("money-input") };
 * ```
 */
export function storyDocs(
  id: string,
  opts: {
    /**
     * Set for the handful of components that ARE a backend search
     * (SupplierSelect, BatchSelect, ProductPickerDialog). A fixture list would
     * demo the opposite of the thing, so their stories hit the dev server
     * through the app's own vite `/api` proxy and need `make run` up.
     */
    needsBackend?: boolean;
  } = {},
): { docs: { description: { component: string } } } {
  const entry = entryFor(id);
  const parts = [entry.summary, "", "```tsx", entry.usage, "```"];

  if (opts.needsBackend) {
    parts.unshift(
      "> **Needs a running backend** (`make run`). This component *is* a server search, so it is",
      "> wired to the real RPC through the app's `/api` proxy. It stays idle until you open it.",
      "",
    );
  }

  // The registry's hand-written prop docs, as a markdown table. Storybook's own
  // Controls table is inferred from the types and stays the authority on what
  // is settable; this one carries the INTENT ("empty = the caller's active
  // warehouse"), which no type can express — and it also fills the gap for
  // generic components (FormField) where react-docgen infers nothing useful.
  if (entry.props.length > 0) {
    parts.push(
      "",
      "| Prop | Type | Notes |",
      "| --- | --- | --- |",
      ...entry.props.map((p) => {
        const name = p.required ? `\`${p.name}\` *(required)*` : `\`${p.name}\``;
        return `| ${name} | \`${escapePipes(p.type)}\` | ${escapePipes(p.desc)} |`;
      }),
    );
  }

  if (entry.notes) parts.push("", `**Gotchas** — ${entry.notes}`);
  parts.push("", `Source: \`${entry.file}\` · gallery: \`/components\``);
  return { docs: { description: { component: parts.join("\n") } } };
}

// A literal `|` inside a cell would end it and shear the rest of the row into
// phantom columns — and several prop types are unions ('"xs" | "sm"').
function escapePipes(s: string): string {
  return s.replace(/\|/g, "\\|");
}
