import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { toast } from "./toaster";

// Proto int64 fields land in queryKeys as BigInt; JSON.stringify can't
// serialize them, which breaks TanStack Query's default hashing. Stringify
// BigInts explicitly here so every query key (analytics date ranges, IDs,
// etc.) hashes deterministically.
//
// Exported because Storybook's preview builds its OWN QueryClient (fresh per
// story) and has to install the same hash — without it, any page whose key
// carries an int64 throws "Do not know how to serialize a BigInt" on mount and
// the story renders nothing. That is a bench-only failure, invisible in the
// app, so the one implementation is shared rather than re-declared.
export function hashWithBigInt(queryKey: readonly unknown[]): string {
  return JSON.stringify(queryKey, (_key, value) =>
    typeof value === "bigint" ? value.toString() : value,
  );
}

// The auto-toast rule: an unhandled error surfaces as a toast unless the
// query/mutation opted out with `meta.silentError` (a form that renders the
// failure on its own field does).
//
// These are FACTORIES, and exported, because Storybook's preview builds its
// own QueryClient per story and has to install the same caches — a cache is
// stateful, so a shared instance would leak one story's in-flight errors into
// the next. Without them the bench silently swallowed every failure: a story
// staging a refused save showed no toast at all, which is the opposite of what
// the app does and exactly what such a story exists to show.
function toastUnlessSilent(err: unknown, meta: { silentError?: boolean } | undefined) {
  if (meta?.silentError !== true) toast.fromError(err);
}

export function newQueryCache(): QueryCache {
  return new QueryCache({ onError: (err, query) => toastUnlessSilent(err, query.meta) });
}

export function newMutationCache(): MutationCache {
  return new MutationCache({
    onError: (err, _vars, _ctx, mutation) => toastUnlessSilent(err, mutation.meta),
  });
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: 1,
      queryKeyHashFn: hashWithBigInt,
    },
    mutations: {
      retry: 0,
    },
  },
  queryCache: newQueryCache(),
  mutationCache: newMutationCache(),
});

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: { silentError?: boolean };
    mutationMeta: { silentError?: boolean };
  }
}
