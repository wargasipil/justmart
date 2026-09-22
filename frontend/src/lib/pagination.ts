import { useEffect, useRef, useState } from "react";

// Default page size for every server-paginated list.
export const DEFAULT_PAGE_SIZE = 25;
export const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const;

// Limit used by page-level "preload the whole list" fetches — name-lookup
// display maps and the few remaining preload-mode selects. These are exempt
// from the server-side-search hard rule (which governs dynamic <SearchableSelect>
// option sources, not page-level name maps). Denormalizing names into the
// parent response is the eventual fix; until then this caps the preload.
export const ALL_LIMIT = 1000;

// usePageState centralizes list pagination: 0-based page + page size, and
// resets to page 0 whenever the caller's filter signature (resetKey) changes
// so a filtered result never lands the user on an out-of-range page.
export function usePageState(resetKey: string) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSizeRaw] = useState<number>(DEFAULT_PAGE_SIZE);
  const prev = useRef(resetKey);

  useEffect(() => {
    if (prev.current !== resetKey) {
      prev.current = resetKey;
      setPage(0);
    }
  }, [resetKey]);

  const setPageSize = (n: number) => {
    setPageSizeRaw(n);
    setPage(0);
  };

  return { page, setPage, pageSize, setPageSize };
}

// --- Load more (phones) -------------------------------------------------------
//
// Below `md`, <Pagination> renders "Load more" instead of Prev/Next: it keeps
// offset at 0 and GROWS pageSize by LOAD_MORE_STEP, so the rows already on
// screen stay and more are appended — every list hook works unchanged, since
// they all take page + pageSize. Growing pageSize changes the query key,
// though, so without keepPageData the list would blank behind a spinner on
// every tap.
export const LOAD_MORE_STEP = DEFAULT_PAGE_SIZE;

const PAGING_KEYS = new Set(["page", "pageSize", "limit", "offset"]);

// A list's identity minus its paging: numbers at the top level of the key
// (positional page/pageSize) and page/pageSize/limit/offset props are dropped;
// everything that selects WHICH rows (filters, ids, flags) stays.
function listScope(key: readonly unknown[]): string {
  const parts = key
    .filter((x) => typeof x !== "number")
    .map((x) =>
      x && typeof x === "object" && !Array.isArray(x)
        ? Object.fromEntries(
            Object.entries(x as Record<string, unknown>).filter(
              ([k]) => !PAGING_KEYS.has(k),
            ),
          )
        : x,
    );
  return JSON.stringify(parts, (_k, v) => (typeof v === "bigint" ? v.toString() : v));
}

/**
 * `placeholderData` for a paginated list query: while the next page (or a
 * bigger Load-more page) loads, keep showing the previous rows — but ONLY when
 * the key changed in paging alone. A new filter or a different parent id
 * still starts empty, so a list never shows another list's rows.
 *
 * ```ts
 * const queryKey = fooKeys.list({ query, page, pageSize });
 * useQuery({ queryKey, placeholderData: keepPageData(queryKey), queryFn })
 * ```
 */
export function keepPageData(queryKey: readonly unknown[]) {
  const scope = listScope(queryKey);
  return <T>(prev: T | undefined, prevQuery?: { queryKey: readonly unknown[] }) =>
    prevQuery && listScope(prevQuery.queryKey) === scope ? prev : undefined;
}
