import { useEffect, useRef } from "react";
import type { FieldValues, UseFormReturn } from "react-hook-form";

/**
 * Reset an RHF form on every closed → open transition of a drawer / dialog.
 *
 * Why this can't be solved inside <EntityDrawer>: the `useForm` call lives in
 * the component that RENDERS the drawer, not inside it, so it stays mounted
 * while the drawer is closed. Chakra's `lazyMount`/`unmountOnExit` therefore
 * doesn't help either — and RHF keeps values in a ref regardless of whether the
 * fields are mounted (`shouldUnregister` is false by default). Without an
 * explicit reset, cancelling a half-typed form leaves those values sitting in
 * the fields the next time it opens.
 *
 * It bites edit drawers too, not just create ones: RHF's `values` prop only
 * re-syncs when the object changes, so re-opening on the SAME record after
 * abandoning an edit is deep-equal, skips the sync, and shows the stale edits.
 *
 * `values` is read through a ref, so a fresh object literal on every render
 * (the normal call shape) doesn't re-trigger the reset — only the open edge
 * does. Pass the record's values for an edit form, the empty defaults for a
 * create form.
 */
export function useResetOnOpen<T extends FieldValues>(
  form: UseFormReturn<T>,
  open: boolean,
  values: T,
): void {
  const latest = useRef(values);
  latest.current = values;
  const wasOpen = useRef(false);

  useEffect(() => {
    if (open && !wasOpen.current) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      form.reset(latest.current as any);
    }
    wasOpen.current = open;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
}
